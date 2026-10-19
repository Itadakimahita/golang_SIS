package main

import (
	"errors"
	"fmt"
)

type Status string

const (
	Pending    Status = "pending"
	Processing Status = "processing"
	Completed  Status = "completed"
	Failed     Status = "failed"
)

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidState = errors.New("invalid task state")
)

type Result struct {
	Value   float64
	Postfix string
}

type Task struct {
	ID         int
	Expression string
	Status     Status
	Result     *Result
	Err        string
}

func (t *Task) move(from, to Status) error {
	if t.Status != from {
		return fmt.Errorf("task %d is %s, cannot move to %s (needs %s): %w", t.ID, t.Status, to, from, ErrInvalidState)
	}
	t.Status = to
	return nil
}

func (t *Task) start() error {
	return t.move(Pending, Processing)
}

func (t *Task) finish(r Result) error {
	if err := t.move(Processing, Completed); err != nil {
		return err
	}
	t.Result = &r
	return nil
}

func (t *Task) fail(reason error) error {
	if err := t.move(Processing, Failed); err != nil {
		return err
	}
	t.Err = reason.Error()
	return nil
}
