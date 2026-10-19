package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	ErrEmptyExpression   = errors.New("expression is empty")
	ErrInvalidExpression = errors.New("invalid expression")
	ErrDivisionByZero    = errors.New("division by zero")
	ErrNotFinite         = errors.New("result is not a finite number")
)

type Evaluator interface {
	Check(expr string) error
	Evaluate(expr string) (Result, error)
}

type Calculator struct{}

func (Calculator) Check(expr string) error {
	_, err := toPostfix(expr)
	return err
}

func (Calculator) Evaluate(expr string) (Result, error) {
	tokens, err := toPostfix(expr)
	if err != nil {
		return Result{}, err
	}
	value, err := evalPostfix(tokens)
	if err != nil {
		return Result{}, err
	}
	return Result{Value: value, Postfix: formatPostfix(tokens)}, nil
}

const negate = 'n'

type token struct {
	isNumber bool
	number   float64
	op       rune
}

func precedence(op rune) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	case negate:
		return 3
	case '^':
		return 4
	}
	return 0
}

func syntaxError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidExpression, fmt.Sprintf(format, args...))
}

func toPostfix(expr string) ([]token, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, ErrEmptyExpression
	}

	chars := []rune(expr)
	var out []token
	var stack []rune
	expectOperand := true

	pop := func() rune {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return top
	}

	for i := 0; i < len(chars); {
		c := chars[i]
		switch {
		case c == ' ' || c == '\t':
			i++

		case c >= '0' && c <= '9' || c == '.':
			if !expectOperand {
				return nil, syntaxError("missing operator before number at position %d", i+1)
			}
			j := i
			for j < len(chars) && (chars[j] >= '0' && chars[j] <= '9' || chars[j] == '.') {
				j++
			}
			n, err := strconv.ParseFloat(string(chars[i:j]), 64)
			if err != nil {
				return nil, syntaxError("bad number %q", string(chars[i:j]))
			}
			out = append(out, token{isNumber: true, number: n})
			expectOperand = false
			i = j

		case c == '(':
			if !expectOperand {
				return nil, syntaxError("missing operator before '(' at position %d", i+1)
			}
			stack = append(stack, c)
			i++

		case c == ')':
			if expectOperand {
				return nil, syntaxError("missing operand before ')' at position %d", i+1)
			}
			for len(stack) > 0 && stack[len(stack)-1] != '(' {
				out = append(out, token{op: pop()})
			}
			if len(stack) == 0 {
				return nil, syntaxError("unmatched ')' at position %d", i+1)
			}
			pop()
			i++

		case strings.ContainsRune("+-*/^", c):
			if expectOperand {
				if c != '-' {
					return nil, syntaxError("missing operand before %q at position %d", c, i+1)
				}
				stack = append(stack, negate)
				i++
				continue
			}
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				if top == '(' {
					break
				}
				if precedence(top) < precedence(c) || (precedence(top) == precedence(c) && c == '^') {
					break
				}
				out = append(out, token{op: pop()})
			}
			stack = append(stack, c)
			expectOperand = true
			i++

		default:
			return nil, syntaxError("unexpected character %q at position %d", c, i+1)
		}
	}

	if expectOperand {
		return nil, syntaxError("expression ends where a number was expected")
	}
	for len(stack) > 0 {
		top := pop()
		if top == '(' {
			return nil, syntaxError("unmatched '('")
		}
		out = append(out, token{op: top})
	}
	return out, nil
}

func evalPostfix(tokens []token) (float64, error) {
	var stack []float64

	for _, t := range tokens {
		if t.isNumber {
			stack = append(stack, t.number)
			continue
		}

		if t.op == negate {
			stack[len(stack)-1] = -stack[len(stack)-1]
			continue
		}

		a, b := stack[len(stack)-2], stack[len(stack)-1]
		stack = stack[:len(stack)-2]

		var r float64
		switch t.op {
		case '+':
			r = a + b
		case '-':
			r = a - b
		case '*':
			r = a * b
		case '/':
			if b == 0 {
				return 0, ErrDivisionByZero
			}
			r = a / b
		case '^':
			r = math.Pow(a, b)
		}
		if math.IsNaN(r) || math.IsInf(r, 0) {
			return 0, ErrNotFinite
		}
		stack = append(stack, r)
	}

	return stack[0], nil
}

func formatPostfix(tokens []token) string {
	parts := make([]string, len(tokens))
	for i, t := range tokens {
		switch {
		case t.isNumber:
			parts[i] = strconv.FormatFloat(t.number, 'g', -1, 64)
		case t.op == negate:
			parts[i] = "neg"
		default:
			parts[i] = string(t.op)
		}
	}
	return strings.Join(parts, " ")
}
