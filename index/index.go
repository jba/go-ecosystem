// Package index supports queries on the Go module index (index.golang.org).
package index

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jba/go-ecosystem/internal/httputil"
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
	return read(ctx, http.DefaultClient, since, limit)
}

// read is the implementation of [Read], with an explicit HTTP client so tests
// can substitute a record/replay client.
func read(ctx context.Context, c *http.Client, since string, limit int) ([]*Entry, error) {
	u := "https://index.golang.org/index"
	params := url.Values{}
	if since != "" {
		params.Set("since", since)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	body, err := httputil.DoReadBody(c, req)
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
// If an error occurs, the iterator yields a nil [Entry] with a non-nil error and stops.
func Entries(ctx context.Context, since string) iter.Seq2[*Entry, error] {
	return entries(ctx, http.DefaultClient, since, 0)
}

// entries is the implementation of [Entries], with an explicit HTTP client and
// an additional limit parameter giving the number of entries to request per
// page. If limit is zero, the index's default page size is used. These exist
// so tests can use a record/replay client and a small page size.
func entries(ctx context.Context, c *http.Client, since string, limit int) iter.Seq2[*Entry, error] {
	return func(yield func(*Entry, error) bool) {
		prevs := map[Entry]bool{} // previously seen entries at since.
		for {
			entries, err := read(ctx, c, since, limit)
			if err != nil {
				yield(nil, err)
				return
			}
			n := 0
			for _, e := range entries {
				if prevs[*e] {
					continue
				}
				if !yield(e, nil) {
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
	}
}
