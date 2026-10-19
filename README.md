# SIS – Expression Calculator

A small Go program that calculates math expressions. You create a task with an
expression like `2 + 3 * (4 - 1)`, process it, and get the answer, plus the expression
rewritten in postfix form (how the computer actually evaluates it). Everything is stored
in memory.

## Example

```
Input:    2 + 3 * (4 - 1)
Result:   11
Postfix:  2 3 4 1 - * +
```

What it understands:

- numbers, including decimals: `3.5`
- `+`  `-`  `*`  `/`  `^` (power)
- parentheses: `(2 + 3) * 4`
- negative numbers: `-5`, `-(1 + 2)`, `2 ^ -1`
- normal math rules: `*` and `/` before `+` and `-`, left to right for `10 - 4 - 3`,
  and `2 ^ 3 ^ 2` is `2 ^ 9` = 512 (power goes right to left)

## Task lifecycle

```
Pending ──► Processing ──► Completed
                │
                └───────► Failed
```

| State        | Meaning                                  | Next state              |
|--------------|------------------------------------------|-------------------------|
| `pending`    | created, not calculated yet              | `processing`            |
| `processing` | being calculated right now               | `completed` or `failed` |
| `completed`  | done, the result is attached             | none                    |
| `failed`     | the calculation hit a math error         | none                    |

Any other change is an error. For example, processing a completed or failed task again
returns `ErrInvalidState` and leaves the task as it was.

## Two kinds of errors

1. **Bad syntax** is rejected when you create the task, so it is never stored.
   Examples: empty text, `1 +`, `(1 + 2`, `2 3`, `2 & 3`.
2. **Math errors** can only be found while calculating, so they happen during processing
   and the task becomes `failed` (with the reason saved in `Err`).
   Examples: `1 / 0`, `0 ^ -1`, `(-8) ^ 0.5`, `10 ^ 400`.

That is why `1 / 0` can be created, but processing it fails.

## Run

Needs Go 1.27.

```bash
go run .                         # demo with some built-in expressions
go run . "2 * (3 + 4)" "1 / 0"   # or give your own expressions
go test ./...
go vet ./...
gofmt -l .
```

## Files

| File                 | What is in it |
|----------------------|---------------|
| `task.go`            | `Task`, `Result`, `Status`, and the state changes (`start`, `finish`, `fail`) |
| `calculator.go`      | the `Evaluator` interface and `Calculator`: parsing and calculating |
| `manager.go`         | `Manager`: create, get, list and process tasks, stored in a map |
| `main.go`            | the demo |
| `calculator_test.go` | tests for parsing and calculating |
| `manager_test.go`    | tests for the task lifecycle |

## How it works

1. **Parsing.** The text is read left to right and converted to postfix notation with the
   shunting-yard algorithm. This also finds all the syntax errors.
2. **Calculating.** The postfix list is evaluated with a stack: a number is pushed, an
   operator takes the top numbers off the stack and pushes the answer.

## Design

- `Manager` depends on the `Evaluator` interface, so the calculator can be replaced.
- Tasks live in a `map[int]*Task`. IDs are 1, 2, 3, ... and a rejected expression does not
  use up an ID.
- The state only changes through `start()`, `finish()` and `fail()`, and each one checks
  the current state first.
- Errors are returned (never `panic`) and wrapped with `%w`, so they can be checked with
  `errors.Is`.

## Tests

| Spec requirement             | Test |
|------------------------------|------|
| Creating a task              | `TestCreate` |
| Invalid input                | `TestCreateInvalidInput`, `TestCheckRejectsBadSyntax` (table-driven) |
| Getting a task               | `TestGet` |
| Task that does not exist     | `TestGetMissing`, `TestProcessMissing` |
| Processing and result        | `TestProcess`, `TestEvaluate` (table-driven) |
| State after processing       | `TestProcess`, `TestProcessFailure` |
| Invalid operation            | `TestProcessTwiceIsRejected`, `TestInvalidStateTransitions` (table-driven) |
| Several independent tasks    | `TestMultipleTasksAreIndependent` |

More detail about the code is in [CODE_EXPLANATION.md](CODE_EXPLANATION.md).
