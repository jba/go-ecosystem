package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: splitzips <zipfile>...")
	}
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(zipFiles []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	outputDir := filepath.Join(home, "splitzips")

	for _, zipPath := range zipFiles {
		if err := processZip(zipPath, outputDir); err != nil {
			return fmt.Errorf("processing %s: %w", zipPath, err)
		}
	}
	return nil
}

func processZip(zipPath, outputDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Group files by path prefix.
	groups := make(map[string][]*zip.File)
	for _, f := range r.File {
		prefix := pathPrefix(f.Name)
		if prefix == "" {
			return fmt.Errorf("%s has no path prefix", f.Name)
		}
		groups[prefix] = append(groups[prefix], f)
	}

	// Write each group to its own zip file.
	for prefix, files := range groups {
		if err := writeZip(outputDir, prefix, files); err != nil {
			return fmt.Errorf("writing %s: %w", prefix, err)
		}
	}
	return nil
}

// pathPrefix returns the path prefix for a file path.
// The prefix is from the beginning up to and including the component containing "@".
// Returns empty string if no "@" component is found.
func pathPrefix(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.Contains(part, "@") {
			return strings.Join(parts[:i+1], "/")
		}
	}
	return ""
}

func writeZip(outputDir, prefix string, files []*zip.File) error {
	outPath := filepath.Join(outputDir, prefix+".zip")

	// Create parent directories.
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}

	w := zip.NewWriter(f)

	var writeErr error
	for _, file := range files {
		if err := copyToZip(w, file); err != nil {
			writeErr = err
			break
		}
	}

	closeErr := w.Close()
	fileErr := f.Close()

	return errors.Join(writeErr, closeErr, fileErr)
}

func copyToZip(w *zip.Writer, file *zip.File) error {
	// Create the file in the output zip with the same header.
	header := file.FileHeader
	dst, err := w.CreateHeader(&header)
	if err != nil {
		return err
	}

	// If it's a directory, nothing more to do.
	if file.FileInfo().IsDir() {
		return nil
	}

	// Copy the file contents.
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)
	return err
}
