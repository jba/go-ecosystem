// Package errs provides error handling utilities.
package errs

import (
	"errors"
	"fmt"
	"io"
)

// Wrap wraps an error with additional context.
// It is designed to be used with defer:
//
//	func foo() (err error) {
//	    defer errs.Wrap(&err, "foo failed")
//	    ...
//	}
func Wrap(errp *error, format string, args ...any) {
	if *errp != nil {
		*errp = fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), *errp)
	}
}

// Cleanup invokes f.
// It sets *errp to the join of *errp and the error returned from f.
// If *errp is nil, it is set to f's return value.
func Cleanup(errp *error, f func() error) {
	*errp = errors.Join(*errp, f())
}

// A Writer is an io.Writer that remembers errors.
// When a call to Write returns an error, no subsequent Writes
// are performed, and [Writer.Err] will return the error.
type Writer struct {
	w   io.Writer
	err error
}

func NewWriter(w io.Writer) *Writer {
	if ew, ok := w.(*Writer); ok {
		return ew
	}
	return &Writer{w: w}
}

func (w *Writer) Write(buf []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.w.Write(buf)
	if err != nil {
		w.err = err
	}
	return n, err
}

func (w *Writer) Err() error {
	return w.err
}
