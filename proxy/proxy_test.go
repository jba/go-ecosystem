package proxy

import (
	"context"
	"flag"
	"fmt"
	"testing"
)

var live = flag.Bool("live", false, "run tests that make live network requests")

func TestList(t *testing.T) {
	if !*live {
		t.Skip("skipping live test; use -live to run")
	}
	got, err := List(context.Background(), "github.com/jba/cli")
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range got {
		fmt.Printf("%d: %q\n", i, g)
	}
}
