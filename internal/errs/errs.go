// Package errs provides error handling utilities.
package errs

import "fmt"

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
