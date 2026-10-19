package service

import (
	"fmt"
	"time"

	"sis/internal/analyzer"
	"sis/internal/store"
	"sis/internal/task"
)

type Service struct {
	store    store.Store
	analyzer analyzer.Analyzer
	now      func() time.Time
	nextID   int
}

func New(s store.Store, a analyzer.Analyzer) *Service {
	return &Service{store: s, analyzer: a, now: time.Now, nextID: 1}
}

func (s *Service) Create(input string) (*task.Task, error) {
	if err := s.analyzer.Validate(input); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	t := task.New(fmt.Sprintf("task-%d", s.nextID), input, s.now())
	if err := s.store.Add(t); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	s.nextID++
	return t.Clone(), nil
}

func (s *Service) Get(id string) (*task.Task, error) {
	return s.store.Get(id)
}

func (s *Service) List() []*task.Task {
	return s.store.List()
}

func (s *Service) Process(id string) (*task.Task, error) {
	t, err := s.store.Get(id)
	if err != nil {
		return nil, fmt.Errorf("process task: %w", err)
	}
	if err := t.Start(s.now()); err != nil {
		return nil, fmt.Errorf("process task: %w", err)
	}
	if err := s.store.Update(t); err != nil {
		return nil, fmt.Errorf("process task: %w", err)
	}

	res, aerr := s.analyzer.Analyze(t.Input)
	if aerr != nil {
		if err := t.Fail(aerr, s.now()); err != nil {
			return nil, fmt.Errorf("process task: %w", err)
		}
	} else if err := t.Complete(res, s.now()); err != nil {
		return nil, fmt.Errorf("process task: %w", err)
	}
	if err := s.store.Update(t); err != nil {
		return nil, fmt.Errorf("process task: %w", err)
	}
	if aerr != nil {
		return t.Clone(), fmt.Errorf("process task %s: %w", id, aerr)
	}
	return t.Clone(), nil
}
