// Package squeeze trims Go module zip files down to their source files.
package squeeze

import (
	"archive/zip"

	"github.com/jba/go-ecosystem/zips"
)

// Zip copies into zw only the Go source files from zr, and the go.mod file.
func Zip(zw *zip.Writer, zr *zip.Reader) error {
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
