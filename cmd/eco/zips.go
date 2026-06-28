package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/jba/go-ecosystem/internal/errs"
	"github.com/jba/go-ecosystem/proxy"
	"github.com/jba/go-ecosystem/zips"
	"golang.org/x/mod/module"
)

func saveZip(ctx context.Context, mpath, version, cacheDir, destDir string) (err error) {
	defer errs.Wrap(&err, "saveZip(%s, %s)", mpath, version)

	zipFilePath, err := moduleFilePath(destDir, mpath, version)
	if err != nil {
		return err
	}

	// If output file already exists, do nothing.
	if _, err := os.Stat(zipFilePath); err == nil {
		log.Printf("%s@%s: already exists at %s", mpath, version, zipFilePath)
		return nil
	}

	// Remove any other files in the output directory (other versions).
	outDir := filepath.Dir(zipFilePath)
	if entries, err := os.ReadDir(outDir); err == nil {
		for _, e := range entries {
			if err := os.Remove(filepath.Join(outDir, e.Name())); err != nil {
				return err
			}
		}
	}

	zr, prov, err := getZip(ctx, mpath, version, cacheDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(zipFilePath)
	if err != nil {
		return err
	}
	defer errs.Cleanup(&err, f.Close)
	zw := zip.NewWriter(f)
	if err := trimZip(zw, zr); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	log.Printf("%s@%s: from %s to %s", mpath, version, prov, zipFilePath)
	return nil
}

// getZip obtains the zip file for the given module.
// It will check the local module cache first.
// If it doesn't find it there, it will check cacheDir if it is not empty.
// Lastly, it will download it from the proxy, and write it to a non-empty cacheDir.
func getZip(ctx context.Context, mpath, version string, cacheDir string) (_ *zip.Reader, provenance string, err error) {
	modCache, err := GoEnv("GOMODCACHE")
	if err != nil {
		return nil, "", err
	}
	modCacheZipDir := filepath.Join(modCache, "cache", "download")
	if zr, err := openModuleZip(modCacheZipDir, mpath, version); err == nil {
		return zr, modCacheZipDir, nil
	}
	if cacheDir != "" {
		if info, err := os.Stat(cacheDir); err != nil || !info.IsDir() {
			return nil, "", fmt.Errorf("%s does not exist or is not a directory", cacheDir)
		}
		if zr, err := openModuleZip(cacheDir, mpath, version); err == nil {
			return zr, cacheDir, nil
		}
	}
	data, err := proxy.ZipData(ctx, mpath, version)
	if err != nil {
		return nil, "", err
	}
	if cacheDir != "" {
		mpath, err := moduleFilePath(cacheDir, mpath, version)
		if err != nil {
			return nil, "", err
		}
		if err := os.MkdirAll(filepath.Dir(mpath), 0o755); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(mpath, data, 0o644); err != nil {
			return nil, "", err
		}
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", err
	}
	return zr, "proxy", nil
}

func openModuleZip(dir string, mpath, version string) (*zip.Reader, error) {
	mpath, err := moduleFilePath(dir, mpath, version)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(mpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

func moduleFilePath(dir string, mpath, version string) (string, error) {
	epath, err := module.EscapePath(mpath)
	if err != nil {
		return "", err
	}
	eversion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, epath, "@v", eversion+".zip"), nil
}

// trimZip copies into zw only the Go source files
// from zr, and the go.mod file.
func trimZip(zw *zip.Writer, zr *zip.Reader) error {
	for _, f := range zr.File {
		if !zips.IsSourceName(f.Name) {
			continue
		}
		if err := zips.CopyZipFile(zw, f); err != nil {
			return err
		}
	}
	return nil
}
