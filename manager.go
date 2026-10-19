package main

import "fmt"

type Manager struct {
	evaluator Evaluator
	tasks     map[int]*Task
	nextID    int
}

func NewManager(e Evaluator) *Manager {
	return &Manager{
		evaluator: e,
		tasks:     make(map[int]*Task),
		nextID:    1,
	}
}

func (m *Manager) Create(expr string) (*Task, error) {
	if err := m.evaluator.Check(expr); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	t := &Task{ID: m.nextID, Expression: expr, Status: Pending}
	m.tasks[t.ID] = t
	m.nextID++
	return t, nil
}

func (m *Manager) Get(id int) (*Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %d: %w", id, ErrNotFound)
	}
	return t, nil
}

func (m *Manager) List() []*Task {
	out := make([]*Task, 0, len(m.tasks))
	for id := 1; id < m.nextID; id++ {
		out = append(out, m.tasks[id])
	}
	return out
}

func (m *Manager) Process(id int) error {
	t, err := m.Get(id)
	if err != nil {
		return err
	}

	if err := t.start(); err != nil {
		return err
	}

	result, err := m.evaluator.Evaluate(t.Expression)
	if err != nil {
		if ferr := t.fail(err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("task %d: %w", id, err)
	}
	return t.finish(result)
}
