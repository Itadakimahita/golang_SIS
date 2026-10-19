package main

import (
	"fmt"
	"os"
)

func main() {
	expressions := os.Args[1:]
	if len(expressions) == 0 {
		expressions = []string{
			"2 + 3 * (4 - 1)",
			"2 ^ 3 ^ 2",
			"-(1.5 + 2.5) * 3",
			"10 / (5 - 5)",
			"2 +",
			"7 & 3",
		}
	}

	m := NewManager(Calculator{})

	for _, expr := range expressions {
		if _, err := m.Create(expr); err != nil {
			fmt.Printf("rejected %-18q %v\n", expr, err)
		}
	}

	fmt.Println()
	for _, t := range m.List() {
		err := m.Process(t.ID)
		switch t.Status {
		case Completed:
			fmt.Printf("task %d  %-18s = %-8g postfix: %s\n", t.ID, t.Expression, t.Result.Value, t.Result.Postfix)
		case Failed:
			fmt.Printf("task %d  %-18s failed: %v\n", t.ID, t.Expression, err)
		}
	}

	fmt.Println()
	if len(m.List()) > 0 {
		if err := m.Process(1); err != nil {
			fmt.Println("process task 1 again:", err)
		}
	}
	if _, err := m.Get(99); err != nil {
		fmt.Println("get task 99:", err)
	}
}
