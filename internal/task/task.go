package task

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

var ErrInvalidTransition = errors.New("invalid state transition")

type Status string

const (
	Pending    Status = "pending"
	Processing Status = "processing"
	Completed  Status = "completed"
	Failed     Status = "failed"
)

var transitions = map[Status][]Status{
	Pending:    {Processing},
	Processing: {Completed, Failed},
}

func (s Status) CanTransition(next Status) bool {
	return slices.Contains(transitions[s], next)
}

func (s Status) IsTerminal() bool {
	return len(transitions[s]) == 0
}

type Result struct {
	Scheme         string
	Host           string
	Port           int
	DefaultPort    bool
	IsIP           bool
	Path           string
	PathSegments   []string
	Query          map[string][]string
	Fragment       string
	Secure         bool
	HasCredentials bool
	Warnings       []string
}

func (r Result) Clone() Result {
	r.PathSegments = slices.Clone(r.PathSegments)
	r.Warnings = slices.Clone(r.Warnings)
	if r.Query != nil {
		q := make(map[string][]string, len(r.Query))
		for k, v := range r.Query {
			q[k] = slices.Clone(v)
		}
		r.Query = q
	}
	return r
}

type Task struct {
	ID        string
	Input     string
	Status    Status
	Result    *Result
	Err       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(id, input string, now time.Time) *Task {
	return &Task{
		ID:        id,
		Input:     input,
		Status:    Pending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (t *Task) Clone() *Task {
	c := *t
	if t.Result != nil {
		r := t.Result.Clone()
		c.Result = &r
	}
	return &c
}

func (t *Task) transition(next Status, now time.Time) error {
	if !t.Status.CanTransition(next) {
		return fmt.Errorf("task %s: %s -> %s: %w", t.ID, t.Status, next, ErrInvalidTransition)
	}
	t.Status = next
	t.UpdatedAt = now
	return nil
}

func (t *Task) Start(now time.Time) error {
	return t.transition(Processing, now)
}

func (t *Task) Complete(res Result, now time.Time) error {
	if err := t.transition(Completed, now); err != nil {
		return err
	}
	r := res.Clone()
	t.Result = &r
	return nil
}

func (t *Task) Fail(reason error, now time.Time) error {
	if err := t.transition(Failed, now); err != nil {
		return err
	}
	t.Err = reason.Error()
	return nil
}
