# Code Explanation

The whole program is one package (`main`) in a few small files.

## task.go

- `Status` is a string type with four values: `Pending`, `Processing`, `Completed`, `Failed`.
- `ErrNotFound` and `ErrInvalidState` are defined here. Other code checks them with
  `errors.Is`.
- `Result` holds the answer (`Value`) and the postfix form (`Postfix`).
- `Task` holds `ID`, `Expression`, `Status`, `Result` and `Err`. `Result` is a pointer so
  `nil` means "no result yet". `Err` is the failure reason as text.
- `move(from, to)` is the one place that changes the status. It checks the task is in the
  `from` state; if not, it returns `ErrInvalidState` and changes nothing.
- `start()` = `Pending → Processing`.
  `finish(result)` = `Processing → Completed` and saves the result.
  `fail(reason)` = `Processing → Failed` and saves the reason.
- They use a **pointer receiver** (`*Task`) because they change the task.

## calculator.go

### Interface

`Evaluator` has two methods:

- `Check(expr)` – is the syntax valid? (used when creating a task)
- `Evaluate(expr)` – calculate it and return a `Result` (used when processing)

`Calculator` implements it. It has no fields, so it uses value receivers.

### Errors

- `ErrEmptyExpression`, `ErrInvalidExpression` – syntax problems, found by `Check`.
- `ErrDivisionByZero`, `ErrNotFinite` – math problems, found only by `Evaluate`.
  `ErrNotFinite` covers results like infinity or "not a number" (`0 ^ -1`,
  `(-8) ^ 0.5`, `10 ^ 400`).

### Step 1: `toPostfix` (parsing)

Converts normal ("infix") notation to postfix with the **shunting-yard algorithm**.

- Numbers go straight to the output list.
- Operators wait on a stack. When a new operator arrives, operators on the stack that bind
  tighter (or equally tight, for left-to-right operators) are moved to the output first.
  That is what makes `2 + 3 * 4` come out as `2 3 4 * +`.
- `precedence` ranks the operators: `+ -` (1), `* /` (2), negation (3), `^` (4).
- `^` is the exception: with equal precedence it is **not** popped, so `2 ^ 3 ^ 2` groups
  as `2 ^ (3 ^ 2)`.
- Parentheses are put on the stack and popped when `)` arrives. If one is missing, that is
  a syntax error.
- **Negative numbers.** The flag `expectOperand` says whether the next thing must be a
  number. A `-` seen while `expectOperand` is true is a negation, not a subtraction. It is
  stored as the internal operator `negate`. Because it ranks below `^`, `-2 ^ 2` is `-4`.
- The same flag finds most syntax errors: two numbers in a row (`2 3`), two operators in a
  row, an operator at the end, `()`, and a number right before `(`.
- Unknown characters and bad numbers like `1..2` are errors too. Error messages include
  the position.

If this function succeeds, the postfix list is guaranteed to be well formed, which is why
the next step does not need to check the stack.

### Step 2: `evalPostfix` (calculating)

Goes through the postfix list with a stack of numbers:

- number → push it
- negation → flip the sign of the top number
- other operator → take the top two numbers, apply it, push the answer

Division by zero returns `ErrDivisionByZero`. After each operation, an infinite or NaN
result returns `ErrNotFinite`.

### `formatPostfix`

Turns the token list into text like `2 3 4 1 - * +` (negation is written as `neg`).

## manager.go

- `Manager` has an `Evaluator`, a map of tasks (`map[int]*Task`) and `nextID`.
- `Create(expr)` calls `Check`. If the syntax is bad it returns the error (wrapped) and
  creates nothing, and no ID is used. Otherwise it stores a `Pending` task.
- `Get(id)` looks the task up; a missing one gives `ErrNotFound`.
- `List()` returns all tasks in creation order (a map has no order, so it counts IDs from
  1 up to `nextID`).
- `Process(id)`:
  1. get the task (missing → `ErrNotFound`)
  2. `start()` it (not pending → `ErrInvalidState`, task unchanged)
  3. call `Evaluate`
  4. on error → `fail()` the task and return the error, on success → `finish()` with the result

Each task is its own struct in the map, so processing one never touches another.

## main.go

A demo. It creates tasks from the command-line arguments (or a built-in list), prints the
rejected ones, processes the rest, and prints each result or failure. At the end it shows
the two error cases: processing a completed task and getting a task that does not exist.

## Tests

- `calculator_test.go`
  - `TestEvaluate` (table): precedence, parentheses, decimals, left/right associativity,
    negation, and the exact postfix output
  - `TestCheckRejectsBadSyntax` (table): 14 kinds of bad input
  - `TestCheckAcceptsValidSyntaxEvenIfItCannotBeEvaluated`: `1 / 0` passes `Check`
  - `TestEvaluateRuntimeErrors` (table): division by zero, infinity, NaN
- `manager_test.go`
  - creating, invalid input (table), rejected input not using an ID, getting, missing task
  - `TestProcess`: status `completed`, correct value and postfix
  - `TestProcessFailure`: `1 / 0` ends as `failed` with no result and the reason saved
  - `TestProcessTwiceIsRejected` and `TestFailedTaskCannotBeProcessedAgain`
  - `TestInvalidStateTransitions` (table): every wrong `start` / `finish` / `fail`
  - `TestMultipleTasksAreIndependent`: four tasks (one failing), processing one leaves the
    others untouched, and every result belongs to its own expression
