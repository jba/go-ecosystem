package index

import (
	"net/http"
	"testing"

	"github.com/tmc/clones/httprr"
)

// replayClient returns an httprr-backed HTTP client that records or replays
// against testdata/*.httprr instead of hitting index.golang.org for real.
//
// Record new traces with:  go test ./index -httprecord=.
func replayClient(t *testing.T, file string) *http.Client {
	t.Helper()
	rr, err := httprr.Open(file, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rr.Close() })
	return rr.Client()
}

func TestRead(t *testing.T) {
	c := replayClient(t, "testdata/read.httprr")

	const limit = 10
	entries, err := read(t.Context(), c, "", limit)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("got no entries")
	}
	if len(entries) > limit {
		t.Errorf("got %d entries, want at most %d", len(entries), limit)
	}
	for _, e := range entries {
		if e.Path == "" || e.Version == "" || e.Timestamp == "" {
			t.Errorf("incomplete entry: %+v", e)
		}
	}
}

func TestEntries(t *testing.T) {
	c := replayClient(t, "testdata/entries.httprr")

	seen := map[Entry]bool{}
	// Page 100 at a time so the test exercises pagination across several
	// requests without recording a huge trace.
	const pageSize = 100
	const want = 250 // forces at least three pages
	n := 0
	for e, err := range entries(t.Context(), c, "", pageSize) {
		if err != nil {
			t.Fatal(err)
		}
		if seen[*e] {
			t.Errorf("duplicate entry yielded: %+v", e)
		}
		seen[*e] = true
		if n++; n >= want {
			break
		}
	}
	if n < want {
		t.Fatalf("got %d entries, want at least %d", n, want)
	}
}
