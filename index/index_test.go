package index

import (
	"context"
	"net/http"
	"testing"

	"github.com/tmc/clones/httprr"
)

// withReplayClient swaps the package's client for an httprr-backed one for the
// duration of the test, so index.Read/Entries record or replay against
// testdata/*.httprr instead of hitting index.golang.org for real.
//
// Record new traces with:  go test ./index -httprecord=.
func withReplayClient(t *testing.T, file string) {
	t.Helper()
	rr, err := httprr.Open(file, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rr.Close() })
	saved := client
	client = rr.Client()
	t.Cleanup(func() { client = saved })
}

func TestRead(t *testing.T) {
	withReplayClient(t, "testdata/read.httprr")

	const limit = 10
	entries, err := Read(context.Background(), "", limit)
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
	withReplayClient(t, "testdata/entries.httprr")

	seen := map[Entry]bool{}
	const want = 2500 // more than one index page (the index caps pages at 2000)
	n := 0
	for e, err := range Entries(context.Background(), "") {
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
