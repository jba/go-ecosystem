// Package jiter provides utilities for iterators.
package jiter

import "sync"

// ErrorState tracks error state for an iterator.
type ErrorState struct {
	mu   sync.Mutex
	err  error
	done bool
}

// Set records an error.
func (e *ErrorState) Set(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err == nil {
		e.err = err
	}
}

// Done marks the iterator as complete.
func (e *ErrorState) Done() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.done = true
}

// Func returns a function that returns the recorded error.
// It returns nil if the iterator completed without error.
func (e *ErrorState) Func() func() error {
	return func() error {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.err
	}
}
