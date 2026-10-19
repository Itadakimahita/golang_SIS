package store

import (
	"errors"
	"fmt"

	"sis/internal/task"
)

var ErrNotFound = errors.New("task not found")

var ErrDuplicateID = errors.New("task id already exists")

type Store interface {
	Add(t *task.Task) error
	Update(t *task.Task) error
	Get(id string) (*task.Task, error)
	List() []*task.Task
}

type Memory struct {
	tasks map[string]*task.Task
	order []string
}

var _ Store = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{tasks: make(map[string]*task.Task)}
}

func (m *Memory) Add(t *task.Task) error {
	if _, ok := m.tasks[t.ID]; ok {
		return fmt.Errorf("add %s: %w", t.ID, ErrDuplicateID)
	}
	m.tasks[t.ID] = t.Clone()
	m.order = append(m.order, t.ID)
	return nil
}

func (m *Memory) Update(t *task.Task) error {
	if _, ok := m.tasks[t.ID]; !ok {
		return fmt.Errorf("update %s: %w", t.ID, ErrNotFound)
	}
	m.tasks[t.ID] = t.Clone()
	return nil
}

func (m *Memory) Get(id string) (*task.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("get %q: %w", id, ErrNotFound)
	}
	return t.Clone(), nil
}

func (m *Memory) List() []*task.Task {
	out := make([]*task.Task, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.tasks[id].Clone())
	}
	return out
}
