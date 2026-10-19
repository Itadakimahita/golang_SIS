package task

import (
	"errors"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func TestStatusCanTransition(t *testing.T) {
	all := []Status{Pending, Processing, Completed, Failed}
	allowed := map[[2]Status]bool{
		{Pending, Processing}:   true,
		{Processing, Completed}: true,
		{Processing, Failed}:    true,
	}
	for _, from := range all {
		for _, to := range all {
			want := allowed[[2]Status{from, to}]
			if got := from.CanTransition(to); got != want {
				t.Errorf("%s -> %s: got %t, want %t", from, to, got, want)
			}
		}
	}
}

func TestLifecycleHappyPath(t *testing.T) {
	tk := New("t1", "https://example.com", now)
	if tk.Status != Pending || tk.Result != nil {
		t.Fatalf("new task = %+v, want pending with no result", tk)
	}
	if err := tk.Start(now); err != nil || tk.Status != Processing {
		t.Fatalf("Start: err=%v status=%s", err, tk.Status)
	}
	if err := tk.Complete(Result{Host: "example.com"}, now); err != nil || tk.Status != Completed {
		t.Fatalf("Complete: err=%v status=%s", err, tk.Status)
	}
	if tk.Result == nil || tk.Result.Host != "example.com" {
		t.Fatalf("result not attached: %+v", tk.Result)
	}
}

func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		act  func(*Task) error
		from func(*Task)
	}{
		{"complete while pending", func(tk *Task) error { return tk.Complete(Result{}, now) }, func(*Task) {}},
		{"fail while pending", func(tk *Task) error { return tk.Fail(errors.New("x"), now) }, func(*Task) {}},
		{"start twice", func(tk *Task) error { return tk.Start(now) }, func(tk *Task) { _ = tk.Start(now) }},
		{"start after completed", func(tk *Task) error { return tk.Start(now) }, func(tk *Task) {
			_ = tk.Start(now)
			_ = tk.Complete(Result{}, now)
		}},
		{"complete after failed", func(tk *Task) error { return tk.Complete(Result{}, now) }, func(tk *Task) {
			_ = tk.Start(now)
			_ = tk.Fail(errors.New("x"), now)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := New("t1", "u", now)
			tc.from(tk)
			before, hadResult := tk.Status, tk.Result != nil
			if err := tc.act(tk); !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("error = %v, want ErrInvalidTransition", err)
			}
			if tk.Status != before {
				t.Errorf("status changed from %s to %s on rejected transition", before, tk.Status)
			}
			if (tk.Result != nil) != hadResult {
				t.Errorf("result changed on rejected transition")
			}
		})
	}
}

func TestFailRecordsReason(t *testing.T) {
	tk := New("t1", "u", now)
	_ = tk.Start(now)
	if err := tk.Fail(errors.New("boom"), now); err != nil {
		t.Fatal(err)
	}
	if tk.Status != Failed || tk.Err != "boom" {
		t.Fatalf("got status=%s err=%q", tk.Status, tk.Err)
	}
}

func TestCloneIsDeep(t *testing.T) {
	tk := New("t1", "u", now)
	_ = tk.Start(now)
	_ = tk.Complete(Result{
		PathSegments: []string{"a"},
		Query:        map[string][]string{"k": {"v"}},
		Warnings:     []string{"w"},
	}, now)

	c := tk.Clone()
	c.Result.PathSegments[0] = "changed"
	c.Result.Query["k"][0] = "changed"
	c.Result.Warnings[0] = "changed"

	if tk.Result.PathSegments[0] != "a" || tk.Result.Query["k"][0] != "v" || tk.Result.Warnings[0] != "w" {
		t.Fatalf("mutating the clone changed the original: %+v", tk.Result)
	}
}
