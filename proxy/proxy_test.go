package proxy

import (
	"context"
	"fmt"
	"testing"
)

func TestList(t *testing.T) {
	got, err := List(context.Background(), "github.com/jba/cli")
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range got {
		fmt.Printf("%d: %q\n", i, g)
	}
}

func TestXXX(t *testing.T) {
	got, err := Latest(context.Background(), "github.com/michael-go/migrate")
	fmt.Println(got, err)
}
