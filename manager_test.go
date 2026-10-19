package main

import (
	"errors"
	"testing"
)

func newTestManager() *Manager {
	return NewManager(Calculator{})
}

func TestCreate(t *testing.T) {
	m := newTestManager()

	task, err := m.Create("2 + 3 * 4")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.ID == 0 {
		t.Error("task has no ID")
	}
	if task.Expression != "2 + 3 * 4" {
		t.Errorf("Expression = %q, want %q", task.Expression, "2 + 3 * 4")
	}
	if task.Status != Pending {
		t.Errorf("Status = %s, want %s", task.Status, Pending)
	}
	if task.Result != nil {
		t.Errorf("new task should have no result, got %+v", task.Result)
	}
}

func TestCreateInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr error
	}{
		{"empty", "", ErrEmptyExpression},
		{"spaces only", "   ", ErrEmptyExpression},
		{"dangling operator", "1 +", ErrInvalidExpression},
		{"unbalanced parenthesis", "(1 + 2", ErrInvalidExpression},
		{"unknown character", "2 $ 3", ErrInvalidExpression},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager()

			task, err := m.Create(tc.expr)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if task != nil {
				t.Errorf("got task %+v, want nil", task)
			}
			if n := len(m.List()); n != 0 {
				t.Errorf("invalid input was stored: %d tasks", n)
			}
		})
	}
}

func TestRejectedInputDoesNotUseUpAnID(t *testing.T) {
	m := newTestManager()
	m.Create("1 +")

	task, err := m.Create("1 + 1")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != 1 {
		t.Errorf("ID = %d, want 1", task.ID)
	}
}

func TestGet(t *testing.T) {
	m := newTestManager()
	created, _ := m.Create("7 * 6")

	got, err := m.Get(created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != created.ID || got.Expression != "7 * 6" {
		t.Errorf("got %+v, want task %d with expression %q", got, created.ID, "7 * 6")
	}
}

func TestGetMissing(t *testing.T) {
	m := newTestManager()

	task, err := m.Get(42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if task != nil {
		t.Errorf("got task %+v, want nil", task)
	}
}

func TestProcess(t *testing.T) {
	m := newTestManager()
	created, _ := m.Create("2 + 3 * (4 - 1)")

	if err := m.Process(created.ID); err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	task, _ := m.Get(created.ID)
	if task.Status != Completed {
		t.Errorf("Status = %s, want %s", task.Status, Completed)
	}
	if task.Result == nil {
		t.Fatal("completed task has no result")
	}
	if task.Result.Value != 11 {
		t.Errorf("Value = %v, want 11", task.Result.Value)
	}
	if task.Result.Postfix != "2 3 4 1 - * +" {
		t.Errorf("Postfix = %q, want %q", task.Result.Postfix, "2 3 4 1 - * +")
	}
	if task.Err != "" {
		t.Errorf("Err = %q, want empty", task.Err)
	}
}

func TestProcessMissing(t *testing.T) {
	m := newTestManager()

	if err := m.Process(7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestProcessFailure(t *testing.T) {
	m := newTestManager()
	created, err := m.Create("10 / (5 - 5)")
	if err != nil {
		t.Fatalf("a syntactically valid expression must be accepted: %v", err)
	}

	err = m.Process(created.ID)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}

	task, _ := m.Get(created.ID)
	if task.Status != Failed {
		t.Errorf("Status = %s, want %s", task.Status, Failed)
	}
	if task.Result != nil {
		t.Errorf("failed task should have no result, got %+v", task.Result)
	}
	if task.Err != ErrDivisionByZero.Error() {
		t.Errorf("Err = %q, want %q", task.Err, ErrDivisionByZero.Error())
	}
}

func TestProcessTwiceIsRejected(t *testing.T) {
	m := newTestManager()
	created, _ := m.Create("2 * 21")
	if err := m.Process(created.ID); err != nil {
		t.Fatal(err)
	}

	err := m.Process(created.ID)
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error = %v, want ErrInvalidState", err)
	}

	task, _ := m.Get(created.ID)
	if task.Status != Completed || task.Result == nil || task.Result.Value != 42 {
		t.Errorf("rejected operation changed the task: %+v", task)
	}
}

func TestFailedTaskCannotBeProcessedAgain(t *testing.T) {
	m := newTestManager()
	created, _ := m.Create("1 / 0")
	m.Process(created.ID)

	if err := m.Process(created.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error = %v, want ErrInvalidState", err)
	}
}

func TestInvalidStateTransitions(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		act    func(*Task) error
	}{
		{"finish a pending task", Pending, func(t *Task) error { return t.finish(Result{}) }},
		{"fail a pending task", Pending, func(t *Task) error { return t.fail(ErrNotFound) }},
		{"start a processing task", Processing, func(t *Task) error { return t.start() }},
		{"start a completed task", Completed, func(t *Task) error { return t.start() }},
		{"finish a completed task", Completed, func(t *Task) error { return t.finish(Result{}) }},
		{"start a failed task", Failed, func(t *Task) error { return t.start() }},
		{"finish a failed task", Failed, func(t *Task) error { return t.finish(Result{}) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{ID: 1, Status: tc.status}

			if err := tc.act(task); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("error = %v, want ErrInvalidState", err)
			}
			if task.Status != tc.status {
				t.Errorf("Status changed from %s to %s", tc.status, task.Status)
			}
			if task.Result != nil {
				t.Errorf("a result was attached: %+v", task.Result)
			}
		})
	}
}

func TestMultipleTasksAreIndependent(t *testing.T) {
	m := newTestManager()
	a, _ := m.Create("1 + 1")
	b, _ := m.Create("3 * 3")
	c, _ := m.Create("1 / 0")
	d, _ := m.Create("10 - 4")

	if a.ID == b.ID || b.ID == c.ID || c.ID == d.ID || a.ID == d.ID {
		t.Fatalf("IDs are not unique: %d %d %d %d", a.ID, b.ID, c.ID, d.ID)
	}

	if err := m.Process(b.ID); err != nil {
		t.Fatal(err)
	}
	for _, other := range []*Task{a, c, d} {
		if other.Status != Pending || other.Result != nil {
			t.Errorf("task %d was affected by processing task %d: %+v", other.ID, b.ID, other)
		}
	}

	m.Process(a.ID)
	m.Process(c.ID)
	m.Process(d.ID)

	if a.Result.Value != 2 || b.Result.Value != 9 || d.Result.Value != 6 {
		t.Errorf("results got mixed up: a=%v b=%v d=%v", a.Result.Value, b.Result.Value, d.Result.Value)
	}
	if c.Status != Failed || c.Result != nil {
		t.Errorf("task c should be failed with no result: %+v", c)
	}
	if a.Status != Completed || b.Status != Completed || d.Status != Completed {
		t.Errorf("unexpected statuses: a=%s b=%s d=%s", a.Status, b.Status, d.Status)
	}
}
