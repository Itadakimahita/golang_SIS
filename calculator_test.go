package main

import (
	"errors"
	"testing"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		wantValue   float64
		wantPostfix string
	}{
		{"addition", "1 + 2", 3, "1 2 +"},
		{"multiplication before addition", "2 + 3 * 4", 14, "2 3 4 * +"},
		{"parentheses", "(2 + 3) * 4", 20, "2 3 + 4 *"},
		{"nested example", "2 + 3 * (4 - 1)", 11, "2 3 4 1 - * +"},
		{"decimal result", "10 / 4", 2.5, "10 4 /"},
		{"decimal input", "3.5 * 2", 7, "3.5 2 *"},
		{"subtraction is left associative", "10 - 4 - 3", 3, "10 4 - 3 -"},
		{"power is right associative", "2 ^ 3 ^ 2", 512, "2 3 2 ^ ^"},
		{"unary minus binds looser than power", "-2 ^ 2", -4, "2 2 ^ neg"},
		{"negative exponent", "2 ^ -1", 0.5, "2 1 neg ^"},
		{"negated group", "-(1 + 2)", -3, "1 2 + neg"},
		{"double negation", "--3", 3, "3 neg neg"},
		{"no spaces needed", "2*(3+4)", 14, "2 3 4 + *"},
		{"single number", "42", 42, "42"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Calculator{}.Evaluate(tc.expr)
			if err != nil {
				t.Fatalf("Evaluate(%q) returned error: %v", tc.expr, err)
			}
			if got.Value != tc.wantValue {
				t.Errorf("Value = %v, want %v", got.Value, tc.wantValue)
			}
			if got.Postfix != tc.wantPostfix {
				t.Errorf("Postfix = %q, want %q", got.Postfix, tc.wantPostfix)
			}
		})
	}
}

func TestCheckRejectsBadSyntax(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr error
	}{
		{"empty", "", ErrEmptyExpression},
		{"spaces only", "   ", ErrEmptyExpression},
		{"missing right operand", "1 +", ErrInvalidExpression},
		{"missing left operand", "* 2", ErrInvalidExpression},
		{"leading plus", "+ 1", ErrInvalidExpression},
		{"two numbers in a row", "2 3", ErrInvalidExpression},
		{"two operators in a row", "2 * * 3", ErrInvalidExpression},
		{"unclosed parenthesis", "(1 + 2", ErrInvalidExpression},
		{"extra closing parenthesis", "1 + 2)", ErrInvalidExpression},
		{"empty parentheses", "()", ErrInvalidExpression},
		{"number before parenthesis", "2 (3)", ErrInvalidExpression},
		{"unknown character", "2 & 3", ErrInvalidExpression},
		{"letters", "two + 3", ErrInvalidExpression},
		{"bad number", "1..2 + 1", ErrInvalidExpression},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := (Calculator{}).Check(tc.expr); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Check(%q) error = %v, want %v", tc.expr, err, tc.wantErr)
			}
		})
	}
}

func TestCheckAcceptsValidSyntaxEvenIfItCannotBeEvaluated(t *testing.T) {
	if err := (Calculator{}).Check("1 / 0"); err != nil {
		t.Fatalf("Check(1 / 0) returned error: %v", err)
	}
}

func TestEvaluateRuntimeErrors(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr error
	}{
		{"division by zero", "1 / 0", ErrDivisionByZero},
		{"division by zero from a sum", "5 / (2 - 2)", ErrDivisionByZero},
		{"zero to a negative power", "0 ^ -1", ErrNotFinite},
		{"square root of a negative", "(-8) ^ 0.5", ErrNotFinite},
		{"overflow", "10 ^ 400", ErrNotFinite},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (Calculator{}).Evaluate(tc.expr); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Evaluate(%q) error = %v, want %v", tc.expr, err, tc.wantErr)
			}
		})
	}
}
