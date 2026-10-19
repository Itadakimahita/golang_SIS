package store

import (
	"errors"
	"testing"
	"time"

	"sis/internal/task"
)

var now = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func TestAddAndGet(t *testing.T) {
	m := NewMemory()
	if err := m.Add(task.New("a", "u1", now)); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get("a")
	if err != nil || got.Input != "u1" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestGetMissing(t *testing.T) {
	if _, err := NewMemory().Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestAddDuplicate(t *testing.T) {
	m := NewMemory()
	_ = m.Add(task.New("a", "u1", now))
	if err := m.Add(task.New("a", "u2", now)); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("error = %v, want ErrDuplicateID", err)
	}
	if got, _ := m.Get("a"); got.Input != "u1" {
		t.Fatalf("duplicate add overwrote the original: %+v", got)
	}
}

func TestUpdateMissing(t *testing.T) {
	if err := NewMemory().Update(task.New("a", "u", now)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestStoreReturnsCopies(t *testing.T) {
	m := NewMemory()
	_ = m.Add(task.New("a", "u", now))

	got, _ := m.Get("a")
	got.Status = task.Completed

	again, _ := m.Get("a")
	if again.Status != task.Pending {
		t.Fatalf("stored task changed through a returned copy: %s", again.Status)
	}
}

func TestListKeepsCreationOrder(t *testing.T) {
	m := NewMemory()
	for _, id := range []string{"c", "a", "b"} {
		_ = m.Add(task.New(id, "u", now))
	}
	var ids []string
	for _, tk := range m.List() {
		ids = append(ids, tk.ID)
	}
	if want := []string{"c", "a", "b"}; len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("order = %v, want %v", ids, want)
	}
}
