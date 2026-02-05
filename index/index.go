// Package index supports queries on the Go module index (index.golang.org).
package index

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"

	"jba/work/lib/httputil"
	"jba/work/lib/jiter"
)

type Entry struct {
	Path      string
	Version   string
	Timestamp string
}

// Read reads entries from index.golang.org.
//
// since should either be the empty string or a value returned in the
// Timestamp field of a previously read Entry.
//
// The limit is passed on to the index unless it is zero.
func Read(ctx context.Context, since string, limit int) ([]*Entry, error) {
	url := "https://index.golang.org/index"
	var params []string
	if since != "" {
		params = append(params, "since="+since)
	}
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	body, err := httputil.DoReadBody(req)
	if err != nil {
		return nil, err
	}
	var entries []*Entry
	dec := json.NewDecoder(bytes.NewReader(body))
	// The module index returns a stream of JSON objects formatted with newline
	// as the delimiter.
	for dec.More() {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("decoding JSON: %v", err)
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

// Entries returns an iterator over index entries since the given time, which should be the
// empty string or a value from an [Entry].
// It never returns the same entry twice, even if they have the same timestamp.
func Entries(ctx context.Context, since string) (iter.Seq[*Entry], func() error) {
	var es jiter.ErrorState
	return func(yield func(*Entry) bool) {
		defer es.Done()
		prevs := map[Entry]bool{} // previously seen entries at since.
		for {
			entries, err := Read(ctx, since, 0)
			if err != nil {
				es.Set(err)
				return
			}
			n := 0
			for _, e := range entries {
				if prevs[*e] {
					continue
				}
				if !yield(e) {
					return
				}
				n++
			}
			if n == 0 {
				return
			}
			since = entries[len(entries)-1].Timestamp
			// Remember entries we've returned at this timestamp so we don't repeat them.
			clear(prevs)
			for i := len(entries) - 1; i >= 0; i-- {
				if entries[i].Timestamp != since {
					break
				}
				prevs[*entries[i]] = true
			}
		}
	}, es.Func()
}
