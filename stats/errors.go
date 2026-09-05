package stats

import (
	"fmt"
	"strings"
)

// Problem is a single machine readable reason a payload was rejected.
type Problem struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationError reports a payload that does not match the registered schema.
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))

	for _, p := range e.Problems {
		parts = append(parts, fmt.Sprintf("%s: %s", p.Path, p.Message))
	}

	return "validation failed: " + strings.Join(parts, "; ")
}

// ConflictError reports a registration that contradicts what is already stored.
type ConflictError struct {
	Problems []Problem
}

func (e *ConflictError) Error() string {
	parts := make([]string, 0, len(e.Problems))

	for _, p := range e.Problems {
		parts = append(parts, fmt.Sprintf("%s: %s", p.Path, p.Message))
	}

	return "registration conflict: " + strings.Join(parts, "; ")
}

type problems []Problem

func (p *problems) add(code, path, format string, args ...any) {
	*p = append(*p, Problem{Code: code, Path: path, Message: fmt.Sprintf(format, args...)})
}
