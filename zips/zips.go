// Package zips provides utilities for working with Go module zip files.
package zips

import (
	"archive/zip"
	"io"
	"path"
	"strings"
)

// CopyZipFile copies the contents of f into zw.
func CopyZipFile(zw *zip.Writer, f *zip.File) error {
	dst, err := zw.CreateHeader(&f.FileHeader)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() {
		return nil
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	return err
}

// IsSourceName reports whether name is a pathname that refers
// to a Go source file, or a go.mod file.
func IsSourceName(name string) bool {
	dir, file := path.Split(name)
	// TODO(jba): check if this is a valid import path?
	if IsIgnoredByGoTool(dir) || IsVendored(dir) || IsGodeps(dir) {
		return false
	}
	if file == "go.mod" {
		return true
	}
	if path.Ext(file) == ".go" {
		return true
	}
	return false
}

// IsIgnoredByGoTool reports whether the given import path corresponds
// to a directory that would be ignored by the go tool.
//
// The logic of the go tool for ignoring directories is documented at
// https://golang.org/cmd/go/#hdr-Package_lists_and_patterns:
//
//	Directory and file names that begin with "." or "_" are ignored
//	by the go tool, as are directories named "testdata".
//
// However, even though `go list` and other commands that take package
// wildcards will ignore these, they can still be imported and used in
// working Go programs. We continue to ignore the "." and "testdata"
// cases, but we've seen valid Go packages with "_", so we accept those.
//
// Copied from pkgsite/internal/fetch.
func IsIgnoredByGoTool(importPath string) bool {
	return pathHasElement(importPath, func(el string) bool {
		return strings.HasPrefix(el, ".") || el == "testdata"
	})
}

// pathHasElement reports whether pred returns true for any element of path.
func pathHasElement(path string, pred func(string) bool) bool {
	for _, el := range strings.Split(path, "/") {
		if pred(el) {
			return true
		}
	}
	return false
}

// IsVendored reports whether the given import path corresponds
// to a Go package that is inside a vendor directory.
//
// The logic for what is considered a vendor directory is documented at
// https://golang.org/cmd/go/#hdr-Vendor_Directories.
//
// Copied from pkgsite/internal/fetch.
func IsVendored(importPath string) bool {
	return strings.HasPrefix(importPath, "vendor/") ||
		strings.Contains(importPath, "/vendor/")
}

// IsGodeps reports whether the given import path corresponds to a Go package
// that is inside a Godeps directory.
func IsGodeps(importPath string) bool {
	return strings.HasPrefix(importPath, "Godeps/") ||
		strings.Contains(importPath, "/Godeps/")
}
