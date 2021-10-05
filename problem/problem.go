package problem

import "fmt"

// List contains a list of problems.
// Typically functions bubble up an observed error.
// A List can be used to note the error and continue
// function execution thus turning "error" into the "problem".
type List struct {
	errors []error
}

// Adds a problem to the list using fmt.Errorf
func (p *List) Add(format string, args ...interface{}) *List {
	p.errors = append(p.errors, fmt.Errorf(format, args...))
	return p
}

// Returns all added problems
func (p *List) Errors() []error {
	return p.errors
}
