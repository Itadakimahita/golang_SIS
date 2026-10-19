package main

import (
	"errors"
	"fmt"
	"os"

	"sis/internal/analyzer"
	"sis/internal/service"
	"sis/internal/store"
	"sis/internal/task"
)

func main() {
	svc := service.New(store.NewMemory(), analyzer.URLAnalyzer{})

	inputs := []string{
		"https://example.com/docs/go/tour?lang=en&lang=ru#intro",
		"http://admin:hunter2@192.168.0.10:8080/login?token=abc",
		"   ",
		"ftp://example.com/file.txt",
	}

	for _, in := range inputs {
		t, err := svc.Create(in)
		if err != nil {
			fmt.Printf("rejected %q: %v\n", in, err)
			continue
		}
		fmt.Printf("created %s (%s)\n", t.ID, t.Status)
	}

	for _, t := range svc.List() {
		done, err := svc.Process(t.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		printTask(done)
	}

	if _, err := svc.Process("task-1"); errors.Is(err, task.ErrInvalidTransition) {
		fmt.Printf("\nre-processing task-1 rejected: %v\n", err)
	}
	if _, err := svc.Get("task-99"); errors.Is(err, store.ErrNotFound) {
		fmt.Printf("lookup of task-99 failed as expected: %v\n", err)
	}
}

func printTask(t *task.Task) {
	r := t.Result
	fmt.Printf("\n%s [%s] %s\n", t.ID, t.Status, t.Input)
	fmt.Printf("  scheme=%s host=%s port=%d path=%v\n", r.Scheme, r.Host, r.Port, r.PathSegments)
	fmt.Printf("  query=%v fragment=%q secure=%t\n", r.Query, r.Fragment, r.Secure)
	for _, w := range r.Warnings {
		fmt.Printf("  warning: %s\n", w)
	}
}
