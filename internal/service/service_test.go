package service

import (
	"errors"
	"slices"
	"testing"

	"sis/internal/analyzer"
	"sis/internal/store"
	"sis/internal/task"
)

func newService() *Service {
	return New(store.NewMemory(), analyzer.URLAnalyzer{})
}

func TestCreate(t *testing.T) {
	svc := newService()
	tk, err := svc.Create("https://example.com/a")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tk.ID == "" {
		t.Error("task has no ID")
	}
	if tk.Status != task.Pending {
		t.Errorf("status = %s, want pending", tk.Status)
	}
	if tk.Input != "https://example.com/a" {
		t.Errorf("input = %q", tk.Input)
	}
	if tk.Result != nil {
		t.Errorf("new task already has a result")
	}
}

func TestCreateInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"empty", "", analyzer.ErrEmptyInput},
		{"whitespace", "   ", analyzer.ErrEmptyInput},
		{"bad scheme", "gopher://example.com", analyzer.ErrInvalidURL},
		{"no host", "http://", analyzer.ErrInvalidURL},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService()
			tk, err := svc.Create(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tk != nil {
				t.Errorf("returned a task for invalid input: %+v", tk)
			}
			if n := len(svc.List()); n != 0 {
				t.Errorf("invalid input was stored: %d tasks", n)
			}
		})
	}
}

func TestInvalidInputDoesNotConsumeID(t *testing.T) {
	svc := newService()
	_, _ = svc.Create("")
	tk, err := svc.Create("https://example.com")
	if err != nil || tk.ID != "task-1" {
		t.Fatalf("got %v, %v; want task-1", tk, err)
	}
}

func TestGet(t *testing.T) {
	svc := newService()
	created, _ := svc.Create("https://example.com")

	got, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID || got.Input != created.Input {
		t.Errorf("got %+v, want %+v", got, created)
	}
}

func TestGetMissing(t *testing.T) {
	svc := newService()
	tk, err := svc.Get("task-42")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if tk != nil {
		t.Errorf("returned task for missing id: %+v", tk)
	}
}

func TestProcess(t *testing.T) {
	svc := newService()
	created, _ := svc.Create("http://user:pw@10.0.0.1:8080/a/b?token=x&q=1#top")

	done, err := svc.Process(created.ID)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if done.Status != task.Completed {
		t.Fatalf("status = %s, want completed", done.Status)
	}
	r := done.Result
	if r == nil {
		t.Fatal("completed task has no result")
	}
	if r.Scheme != "http" || r.Host != "10.0.0.1" || r.Port != 8080 || r.DefaultPort {
		t.Errorf("wrong components: %+v", r)
	}
	if !slices.Equal(r.PathSegments, []string{"a", "b"}) {
		t.Errorf("segments = %v", r.PathSegments)
	}
	if r.Fragment != "top" || !r.HasCredentials || !r.IsIP || r.Secure {
		t.Errorf("wrong flags: %+v", r)
	}
	if len(r.Warnings) != 5 {
		t.Errorf("warnings = %v, want 5", r.Warnings)
	}

	stored, _ := svc.Get(created.ID)
	if stored.Status != task.Completed || stored.Result == nil || stored.Result.Host != "10.0.0.1" {
		t.Errorf("stored task not updated: %+v", stored)
	}
	if !stored.UpdatedAt.After(stored.CreatedAt) && !stored.UpdatedAt.Equal(stored.CreatedAt) {
		t.Errorf("UpdatedAt %v before CreatedAt %v", stored.UpdatedAt, stored.CreatedAt)
	}
}

func TestProcessMissing(t *testing.T) {
	if _, err := newService().Process("task-9"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestProcessTwiceIsRejected(t *testing.T) {
	svc := newService()
	created, _ := svc.Create("https://example.com/x")
	first, err := svc.Process(created.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Process(created.ID)
	if !errors.Is(err, task.ErrInvalidTransition) {
		t.Fatalf("error = %v, want ErrInvalidTransition", err)
	}

	after, _ := svc.Get(created.ID)
	if after.Status != task.Completed || after.Result == nil || after.Result.Host != first.Result.Host {
		t.Errorf("rejected operation changed the task: %+v", after)
	}
}

func TestMultipleTasksAreIndependent(t *testing.T) {
	svc := newService()
	inputs := []string{
		"https://alpha.example.com/one",
		"http://beta.example.org:8080/two/three",
		"https://gamma.example.net/?k=v",
	}
	ids := make([]string, len(inputs))
	for i, in := range inputs {
		tk, err := svc.Create(in)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = tk.ID
	}
	if len(slices.Compact(slices.Sorted(slices.Values(ids)))) != len(ids) {
		t.Fatalf("IDs are not unique: %v", ids)
	}

	if _, err := svc.Process(ids[1]); err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		tk, _ := svc.Get(id)
		if tk.Input != inputs[i] {
			t.Errorf("task %s input = %q, want %q", id, tk.Input, inputs[i])
		}
		wantStatus := task.Pending
		if i == 1 {
			wantStatus = task.Completed
		}
		if tk.Status != wantStatus {
			t.Errorf("task %s status = %s, want %s", id, tk.Status, wantStatus)
		}
		if (tk.Result != nil) != (i == 1) {
			t.Errorf("task %s result presence wrong: %+v", id, tk.Result)
		}
	}

	for _, i := range []int{0, 2} {
		if _, err := svc.Process(ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	wantHosts := []string{"alpha.example.com", "beta.example.org", "gamma.example.net"}
	for i, id := range ids {
		tk, _ := svc.Get(id)
		if tk.Result.Host != wantHosts[i] {
			t.Errorf("task %s host = %q, want %q", id, tk.Result.Host, wantHosts[i])
		}
	}
}

type stubAnalyzer struct {
	analyzer.URLAnalyzer
	onAnalyze func() (task.Result, error)
}

func (s stubAnalyzer) Analyze(string) (task.Result, error) { return s.onAnalyze() }

func TestTaskIsProcessingWhileAnalyzing(t *testing.T) {
	var svc *Service
	var seen task.Status
	stub := stubAnalyzer{onAnalyze: func() (task.Result, error) {
		tk, _ := svc.Get("task-1")
		seen = tk.Status
		return task.Result{Host: "h"}, nil
	}}
	svc = New(store.NewMemory(), stub)
	_, _ = svc.Create("https://example.com")

	if _, err := svc.Process("task-1"); err != nil {
		t.Fatal(err)
	}
	if seen != task.Processing {
		t.Errorf("status during analysis = %s, want processing", seen)
	}
}

func TestAnalyzerFailureMarksTaskFailed(t *testing.T) {
	boom := errors.New("boom")
	svc := New(store.NewMemory(), stubAnalyzer{onAnalyze: func() (task.Result, error) {
		return task.Result{}, boom
	}})
	created, _ := svc.Create("https://example.com")

	failed, err := svc.Process(created.ID)
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want boom", err)
	}
	if failed.Status != task.Failed || failed.Err != "boom" || failed.Result != nil {
		t.Errorf("failed task = %+v", failed)
	}
	stored, _ := svc.Get(created.ID)
	if stored.Status != task.Failed {
		t.Errorf("stored status = %s, want failed", stored.Status)
	}
	if _, err := svc.Process(created.ID); !errors.Is(err, task.ErrInvalidTransition) {
		t.Errorf("reprocessing failed task: error = %v, want ErrInvalidTransition", err)
	}
}
