# SIS – URL Analyzer

My Checkpoint 1 project for the Go course. It's a small program that takes a URL,
checks that it makes sense, breaks it into parts, and tells you if anything looks risky.
Everything lives in memory, and nothing runs concurrently yet.

## What's a "task" here?

One task = one URL to analyze. You give the app a URL, it stores it as a task, and later
you tell it to process that task. Processing fills in the result.

For example, `http://admin:pw@192.168.0.10:8080/login?token=abc` gives:

```
scheme=http  host=192.168.0.10  port=8080  path=[login]  query={token: [abc]}
warning: uses unencrypted http
warning: contains credentials in the URL
warning: non-default port 8080
warning: host is a raw IP address
warning: sensitive query parameter "token"
```

I picked this domain because it's easy to test (the same input always gives the same
output) and it uses Go's standard library only (`net/url`, `net/netip`).

### What counts as invalid input

- empty or whitespace-only text
- something that doesn't parse as a URL
- a scheme other than `http` / `https` (so `ftp://...` and plain `example.com` are out)
- no host (`https:///path`)
- a port outside 1–65535

Invalid URLs are rejected when you create the task, so they never get stored.

## Task lifecycle

```
Pending ──► Processing ──► Completed
                 │
                 └──────► Failed
```

| State        | Meaning                                   | Can go to             |
|--------------|-------------------------------------------|-----------------------|
| `pending`    | created, not processed yet                | `processing`          |
| `processing` | the analyzer is running                   | `completed`, `failed` |
| `completed`  | done, the result is attached              | nothing (final)       |
| `failed`     | the analyzer returned an error            | nothing (final)       |

Anything else is rejected with an `ErrInvalidTransition` error and the task stays as it
was. For example, you can't process the same task twice.

`failed` isn't strictly needed right now, because the URL is validated before the task
exists. I kept it because the analyzer is an interface and some other implementation
could still fail while processing.

## How to run it

You need Go 1.27.

```bash
go run ./cmd/sis      # small demo: creates, processes and prints a few tasks
go test ./...         # run the tests
go vet ./...
gofmt -l .            # prints nothing if everything is formatted
```

## How the code is organized

```
cmd/sis/            the demo program
internal/task/      Task, Status, Result and the lifecycle rules
internal/analyzer/  the Analyzer interface and the URL analyzer
internal/store/     the Store interface and an in-memory version
internal/service/   Create, Get, List and Process – ties everything together
```

The service talks to the analyzer and the store only through interfaces. That's mostly so
I can swap in a fake analyzer in tests, and so a real database or a different kind of task
could be plugged in later without touching the lifecycle code.

A few decisions worth knowing about:

- **Status changes only happen through methods on the task** (`Start`, `Complete`,
  `Fail`). They check a small transition table, so nothing can put a task into a state it
  shouldn't be in.
- **The store gives out copies.** If you get a task and change it, the stored one doesn't
  change unless you save it back. That's what keeps tasks from affecting each other.
- **`Process` saves the `processing` state before it runs the analyzer**, so the state
  reflects what's actually happening and not just a local variable.
- **IDs** are `task-1`, `task-2`, ... A rejected URL doesn't use up a number.
- **Errors are returned, never panicked.** They're wrapped with `%w`, so you can check them
  with `errors.Is`.
- **No locks.** The store isn't safe for concurrent use. Concurrency isn't part of this
  checkpoint, so I left it out on purpose.

There's a longer walkthrough of the code in [CODE_EXPLANATION.md](CODE_EXPLANATION.md).

## Tests

Only the standard `testing` package. Table-driven tests are used for URL validation,
analysis results, and invalid state transitions.

| What the spec asks for            | Where it's tested |
|-----------------------------------|-------------------|
| Creating a task                   | `service.TestCreate` |
| Invalid input                     | `service.TestCreateInvalidInput`, `analyzer.TestValidate` |
| Getting a task                    | `service.TestGet`, `store.TestAddAndGet` |
| Getting a task that doesn't exist | `service.TestGetMissing`, `store.TestGetMissing` |
| Processing and the result         | `service.TestProcess`, `analyzer.TestAnalyze` |
| State after processing            | `service.TestProcess`, `service.TestTaskIsProcessingWhileAnalyzing` |
| Invalid operation                 | `service.TestProcessTwiceIsRejected`, `task.TestInvalidTransitions` |
| Several independent tasks         | `service.TestMultipleTasksAreIndependent`, `store.TestStoreReturnsCopies` |

There's also a test for the `failed` path (`service.TestAnalyzerFailureMarksTaskFailed`),
which uses a fake analyzer that always returns an error.
