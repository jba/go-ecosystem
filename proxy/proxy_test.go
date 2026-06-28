package proxy

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/tmc/clones/httprr"
)

// testModule is a module with a stable version history used by the tests below.
const testModule = "github.com/jba/bitset"

// replayClient returns an httprr-backed HTTP client that records or replays
// against testdata/*.httprr instead of hitting proxy.golang.org for real.
//
// Record new traces with:  go test ./proxy -httprecord=.
func replayClient(t *testing.T, file string) *http.Client {
	t.Helper()
	rr, err := httprr.Open(file, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rr.Close() })
	return rr.Client()
}

func TestList(t *testing.T) {
	c := replayClient(t, "testdata/list.httprr")

	got, err := list(t.Context(), c, testModule)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("got no versions")
	}
	if !slices.Contains(got, "v0.2.0") {
		t.Errorf("got %v, want it to contain v0.2.0", got)
	}
}

func TestInfo(t *testing.T) {
	c := replayClient(t, "testdata/info.httprr")

	const version = "v0.2.0"
	got, err := info(t.Context(), c, testModule, version)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != version {
		t.Errorf("got version %q, want %q", got.Version, version)
	}
	if got.Time == "" {
		t.Error("got empty Time")
	}
}

func TestLatest(t *testing.T) {
	c := replayClient(t, "testdata/latest.httprr")

	got, err := latest(t.Context(), c, testModule)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("got empty latest version")
	}
}

func TestMod(t *testing.T) {
	c := replayClient(t, "testdata/mod.httprr")

	got, err := mod(t.Context(), c, testModule, "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("got empty go.mod")
	}
	want := "module " + testModule
	if first, _, _ := strings.Cut(string(got), "\n"); first != want {
		t.Errorf("go.mod first line = %q, want %q", first, want)
	}
}
