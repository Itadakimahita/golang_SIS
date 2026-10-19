# Code Explanation

This file explains the code that used to be documented with inline comments. It goes
package by package, in the order data flows through the program:
`cmd/sis` → `service` → `analyzer` / `store` → `task`.

For the lifecycle diagram, run instructions and the spec-to-test table, see [README.md](README.md).

---

## 1. `internal/task/task.go` – the domain model

This package has no dependencies on the rest of the project. Everything else builds on it.

### `ErrInvalidTransition`
A sentinel error. Callers check it with `errors.Is(err, task.ErrInvalidTransition)`
instead of comparing error strings.

### `Status`
`type Status string` with four constants: `Pending`, `Processing`, `Completed`, `Failed`.
A string type prints readably in logs and will serialize naturally to JSON later, while
still being a distinct type, so a plain `"foo"` can't be passed where a `Status` is expected
without a conversion.

### `transitions` and `CanTransition`
```go
var transitions = map[Status][]Status{
    Pending:    {Processing},
    Processing: {Completed, Failed},
}
```
This is the whole state machine as data: for each status, the statuses it may move to.
`Completed` and `Failed` are not keys, so they have no way out (they are terminal).

`CanTransition(next)` is a **value receiver** method (`func (s Status) ...`). `Status` is
a small immutable value, so a copy is fine. It looks up the allowed list and uses
`slices.Contains`. Because a map returns the zero value (a nil slice) for a missing key,
a terminal status naturally gets "nothing allowed" without special-casing.

`IsTerminal()` is true when the allowed list is empty.

### `Result`
The output of processing, holding the parts of the URL plus derived facts:

| Field | Meaning |
|-------|---------|
| `Scheme`, `Host` | lower-cased |
| `Port` | the explicit port, or 80/443 if none was given |
| `DefaultPort` | whether `Port` is the default for the scheme |
| `IsIP` | host is an IP address |
| `Path`, `PathSegments` | escaped path, and its non-empty pieces |
| `Query` | parameter name → list of values (a key can repeat) |
| `Fragment` | the part after `#` |
| `Secure` | scheme is `https` |
| `HasCredentials` | `user:pass@` was present |
| `Warnings` | human-readable risk notes |

`Result.Clone()` is a **deep copy**. Go slices and maps are reference-like: copying a
struct copies the slice header or map pointer, not the data behind it. So `Clone` copies
`PathSegments`, `Warnings` and every slice inside `Query` explicitly. It has a value
receiver, so the `r` inside is already a shallow copy that it then fixes up and returns.

### `Task`
Fields: `ID`, `Input`, `Status`, `Result`, `Err`, `CreatedAt`, `UpdatedAt`.

- `Result` is a **pointer** (`*Result`) so that `nil` means "no result yet". A plain
  struct would always look like it had an empty result.
- `Err` is a string, not an `error`, because it records the reason for storage and
  display after the fact.

### `New(id, input, now)`
Returns a `*Task` in `Pending`. The time is passed in instead of calling `time.Now()`
here, which makes the function deterministic and testable. It does **not** validate:
validation belongs to the analyzer, so the domain type stays simple.

### `Clone()`
Copies the struct (`c := *t`) and then deep-copies `Result` if there is one. The store
uses it to prevent aliasing (see section 3).

### `transition`, `Start`, `Complete`, `Fail`
All use **pointer receivers** because they modify the task.

- `transition(next, now)` is the single choke point: it checks `CanTransition`; if not
  allowed, it returns an error wrapping `ErrInvalidTransition` (with `%w`) and changes
  **nothing**; otherwise it sets `Status` and `UpdatedAt`.
- `Start` → `Processing`.
- `Complete(res, now)` → `Completed`, and only after the transition succeeds does it
  attach a clone of the result. So a rejected `Complete` never leaves a stray result.
- `Fail(reason, now)` → `Failed`, and stores `reason.Error()`.

Since every status change goes through `transition`, no other code can put a task into
an illegal state by accident.

---

## 2. `internal/analyzer/analyzer.go` – validation and processing

### Errors
`ErrEmptyInput` and `ErrInvalidURL` are sentinel errors. Specific failures wrap
`ErrInvalidURL` with detail, e.g. `fmt.Errorf("%w: missing host", ErrInvalidURL)`, so the
message is informative while `errors.Is(err, ErrInvalidURL)` still works.

### `Analyzer` interface
```go
type Analyzer interface {
    Validate(input string) error
    Analyze(input string) (task.Result, error)
}
```
This is the "meaningful interface" required by the spec. The service depends on this
interface, not on `URLAnalyzer`, so another domain or a test stub can be substituted.

`var _ Analyzer = URLAnalyzer{}` is a compile-time check that `URLAnalyzer` satisfies the
interface. If a method is renamed, the build fails here, not somewhere distant.

`URLAnalyzer` is an empty struct: it needs no state, and its methods use value receivers.

### `defaultPorts` and `sensitiveParams`
`defaultPorts` maps `http`→80 and `https`→443; its keys also define the **only accepted
schemes**. `sensitiveParams` lists query keys that should not appear in URLs.

### `parse(input)` – the shared validation
Used by both `Validate` and `Analyze`, so the two can never disagree about what is valid.
Steps, in order:

1. `strings.TrimSpace`; empty → `ErrEmptyInput`.
2. `url.Parse`; failure → `ErrInvalidURL` wrapping the parse error.
3. Lower-cased scheme must be a key of `defaultPorts` (so `ftp://…` and scheme-less
   `example.com/path` are rejected; the latter parses with an empty scheme).
4. `u.Hostname()` must be non-empty (rejects `https:///path`).
5. If a port is present it must be an integer in 1–65535. `url.Parse` only checks
   that the port is numeric, so the range check is done here.

Each rule is something that would break later processing or make the result meaningless.

### `Validate`
Calls `parse` and discards the parsed URL, returning only the error.

### `Analyze`
Calls `parse` again (so it is safe to call without `Validate`), then builds the `Result`:

- `Scheme`/`Host` lower-cased; `Port` starts at the scheme default, and if the URL has an
  explicit port that value replaces it. `DefaultPort` is then recomputed by comparing, so
  `http://example.com:80/` is correctly seen as the default port.
- `Path` uses `EscapedPath()` so encoded characters (e.g. `%20`) are kept as written.
- `Query` is `u.Query()`, which parses `a=1&a=2` into `{"a": ["1","2"]}`.
- `HasCredentials` is `u.User != nil`.
- `IsIP` uses `netip.ParseAddr(host)` and treats "no error" as "it is an IP".
- `PathSegments`: split the path on `/` and keep non-empty pieces, so `/docs/go/` gives
  `["docs", "go"]` and `/` gives none.
- Warnings are appended in a fixed order (http, credentials, port, IP, then sensitive
  parameters), which keeps results deterministic and tests simple.

---

## 3. `internal/store/store.go` – in-memory storage

### `Store` interface
`Add`, `Update`, `Get`, `List`. The service depends on this, so a database-backed store
could be dropped in later.

### `Memory`
```go
type Memory struct {
    tasks map[string]*task.Task
    order []string
}
```
- `tasks` gives O(1) lookup by ID.
- `order` records IDs as they were added, because **Go map iteration order is random**;
  `List` walks `order` to return tasks in creation order.

`var _ Store = (*Memory)(nil)` is the compile-time interface check again. The methods
have pointer receivers, so it is `*Memory` that implements `Store`.

### Copy-in, copy-out
Every method that stores or returns a task uses `Clone()`:

- `Add` and `Update` store a **clone** of what the caller passed.
- `Get` and `List` return **clones** of what is stored.

Without this, the map would hold the same pointer the caller holds, and changing the
caller's task (or a slice inside its result) would silently change stored data, bypassing
the store and mixing up tasks. With cloning, the only way to change stored data is
`Update`. `TestStoreReturnsCopies` and `TestCloneIsDeep` verify this.

### Errors
- `Add` with an existing ID → `ErrDuplicateID`, and the original is **not** overwritten.
- `Update` and `Get` with an unknown ID → `ErrNotFound`.

Errors are wrapped with context (`get "task-99": task not found`) using `%w`, so
`errors.Is(err, store.ErrNotFound)` still matches.

---

## 4. `internal/service/service.go` – the application logic

### `Service`
Holds a `Store`, an `Analyzer` (both interfaces), a `now` function, and `nextID`.

- `now` defaults to `time.Now` but is a field so it can be replaced.
- IDs are `task-1`, `task-2`, … built with `fmt.Sprintf` from `nextID`.

`New` wires the dependencies (this is plain constructor-based dependency injection).

### `Create(input)`
1. `analyzer.Validate(input)`; on error, return it wrapped. **No task is created and no ID
   is consumed.**
2. Build the task with `task.New`.
3. `store.Add`.
4. Only then increment `nextID`.
5. Return a clone, so the caller's copy is independent of the stored one.

### `Get(id)` and `List()`
Thin pass-throughs to the store. A missing ID surfaces as `store.ErrNotFound`.

### `Process(id)` – the core flow
1. **Load** the task from the store (a clone). Unknown ID → wrapped `ErrNotFound`.
2. **`Start`**. If the task is not `Pending`, this returns `ErrInvalidTransition`
   and we return immediately, so the stored task is untouched. This is what rejects
   processing a completed or failed task a second time.
3. **Persist `Processing`** with `Update` *before* analyzing. The state is therefore real,
   not just a local variable: anything that reads the store while the analyzer runs sees
   `processing` (`TestTaskIsProcessingWhileAnalyzing` checks exactly this).
4. **Analyze** the input.
5. On analyzer error → `Fail` the task; otherwise → `Complete` it with the result.
6. **Persist** the final state with `Update`.
7. Return a clone. If the analyzer failed, the failed task is returned **together with** the
   error (wrapping the analyzer's error), so the caller can both inspect the task and
   check `errors.Is`.

Because the work is synchronous, steps 3–6 happen in sequence within one call. In a later
checkpoint the same state machine can be driven from a worker goroutine.

---

## 5. `cmd/sis/main.go` – the demo

Builds the service with `store.NewMemory()` and `analyzer.URLAnalyzer{}`, then:

1. Tries four inputs: two valid, one whitespace-only, one with an `ftp` scheme. Rejected
   ones are printed with their error and skipped.
2. Processes every stored task and prints the result via `printTask`.
3. Re-processes `task-1` and looks up `task-99`, using `errors.Is` to show that the
   "invalid transition" and "not found" errors are detected correctly.

`printTask` prints the main fields and each warning.

---

## 6. Tests

All tests use only the standard `testing` package.

### Patterns used
- **Table-driven tests**: a slice of `{name, input, expected}` structs looped over with
  `t.Run(tc.name, …)`, giving one subtest per case. Used in `TestValidate`,
  `TestAnalyze`, `TestInvalidTransitions`, `TestCreateInvalidInput`, and
  `TestStatusCanTransition` (which checks every from/to combination against the table).
- **`errors.Is`** to assert which error occurred, not just that one did.
- **Stub through embedding**: in `service_test.go`, `stubAnalyzer` embeds
  `analyzer.URLAnalyzer` (so `Validate` is inherited) and overrides only `Analyze` with a
  function the test supplies. This is how the tests:
  - read the store *during* analysis to prove the `processing` state is persisted;
  - force an analyzer failure to exercise the `Failed` path, which the real analyzer can't
    produce after validation.

### What each file covers
| File | Focus |
|------|-------|
| `task_test.go` | the transition table, happy path, every illegal transition leaves the task unchanged, failure reason, deep copy |
| `analyzer_test.go` | valid/invalid URLs; every `Result` field and warning on four representative URLs |
| `store_test.go` | add/get, missing ID, duplicate ID, update of missing ID, copy isolation, list order |
| `service_test.go` | creation, invalid input (and no ID consumed), get, missing, processing, double-processing rejected, independent tasks, `processing` visible mid-run, failure path |

### Independence of tasks
`TestMultipleTasksAreIndependent` creates three tasks, processes only the middle one, and
checks the other two are still `pending` with no result. It then processes the rest and
checks each result's host matches its **own** input.

---

## 7. Go concepts used (spec checklist)

| Concept | Where |
|---------|-------|
| Structs and methods | `Task`, `Result`, `Memory`, `Service`, `URLAnalyzer` |
| Value receivers | `Status.CanTransition`, `Result.Clone`, `URLAnalyzer` methods |
| Pointer receivers | `Task.Start/Complete/Fail/Clone`, `Memory` methods, `Service` methods |
| Interfaces | `Analyzer`, `Store` (+ compile-time checks) |
| Maps | `transitions`, `defaultPorts`, `Memory.tasks`, `Result.Query` |
| Slices | `PathSegments`, `Warnings`, `Memory.order`, transition lists |
| Explicit errors | sentinel errors, `%w` wrapping, `errors.Is`; no `panic` |
| Package organization | `internal/` packages with one-directional dependencies |
