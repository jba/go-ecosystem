package main

import (
	"archive/zip"
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveZip(t *testing.T) {
	ctx := context.Background()
	mpath := "rsc.io/ordered"
	version := "v1.1.1"
	destDir := t.TempDir()

	if err := saveZip(ctx, mpath, version, "", destDir); err != nil {
		t.Fatal(err)
	}

	zipPath, err := moduleFilePath(destDir, mpath, version)
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	var names []string
	for _, f := range zr.File {
		names = append(names, filepath.Base(f.Name))
	}
	slices.Sort(names)

	want := []string{"code.go", "code_test.go", "go.mod"}
	if !slices.Equal(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}
