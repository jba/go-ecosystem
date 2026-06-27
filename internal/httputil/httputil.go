// Package httputil provides HTTP utility functions.
package httputil

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// HTTPError represents an HTTP error response.
type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, http.StatusText(e.Status))
}

// -1 if not an HTTPError
func ErrorStatus(err error) int {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status
	}
	return -1
}

// DoReadBody executes an HTTP request using c and returns the response body.
// If c is nil, http.DefaultClient is used.
// It returns an HTTPError for non-2xx status codes.
func DoReadBody(c *http.Client, req *http.Request) ([]byte, error) {
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.StatusCode}
	}
	return body, nil
}
