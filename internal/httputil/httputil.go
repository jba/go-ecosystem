// Package httputil provides HTTP utility functions.
package httputil

import (
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

// DoReadBody executes an HTTP request and returns the response body.
// It returns an HTTPError for non-2xx status codes.
func DoReadBody(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
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
