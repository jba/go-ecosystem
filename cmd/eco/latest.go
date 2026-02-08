package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"

	"github.com/jba/go-ecosystem/internal/errs"
	"github.com/jba/go-ecosystem/internal/httputil"
	"github.com/jba/go-ecosystem/proxy"
	"github.com/jba/go-ecosystem/versions"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// latestModuleVersion uses the proxy to get information about the latest
// version of modulePath. The cooked latest is computed by choosing the latest
// version after removing versions that are retracted in the go.mod file of the
// raw version.
//
// If a module has no tagged versions and hasn't been accessed at a
// pseudo-version in a while, then the proxy's list endpoint will serve nothing
// and its @latest endpoint will return a 404/410. (Example:
// cloud.google.com/go/compute/metadata, which has a
// v0.0.0-20181107005212-dafb9c8d8707 that @latest does not return.) That is not
// a failure, but a valid state in which there is no version information for a
// module, even though particular pseudo-versions of the module might exist. In
// this case, LatestModuleVersions returns ("", errs.NotFound).
func latestModuleVersion(ctx context.Context, modulePath string) (_ string, err error) {
	defer errs.Wrap(&err, "latestModuleVersion(%s)", modulePath)
	// Get the raw latest version.
	allVersions, err := proxy.List(ctx, modulePath)
	if err != nil {
		return "", err
	}
	latest, err := proxy.Latest(ctx, modulePath)
	if httputil.ErrorStatus(err) == http.StatusNotFound {
		// No information from the proxy, but not a showstopper either; we can
		// proceed with the result of the list endpoint.
	} else if err != nil {
		return "", err
	} else {
		allVersions = append(allVersions, latest)
	}
	if len(allVersions) == 0 {
		// No tagged versions, and nothing from @latest: no version information.
		return "", errNoVersions
	}

	seen := map[string]bool{}
	hasGoMod := func(version string) (bool, error) {
		if seen[version] {
			log.Printf("!!! %s saw version %s twice for mod endpoint", modulePath, version)
		}
		seen[version] = true
		goModBytes, err := proxy.Mod(ctx, modulePath, version)
		if err != nil {
			return false, err
		}
		// The proxy always returns a go.mod file.
		// But if it's just the module line, assume the module doesn't actually have one.
		// This can give the wrong answer for modules that have no required dependencies,
		// but it's much cheaper than downloading the zip.
		return bytes.IndexByte(goModBytes, '\n') != len(goModBytes)-1, nil
	}
	rawLatest, err := versions.Latest(allVersions, hasGoMod)
	if err != nil {
		return "", err
	}
	// Get the go.mod file at the raw latest version.
	modBytes, err := proxy.Mod(ctx, modulePath, rawLatest)
	if err != nil {
		return "", err
	}
	modFile, err := modfile.ParseLax(fmt.Sprintf("%s@%s/go.mod", modulePath, rawLatest), modBytes, nil)
	if err != nil {
		log.Printf("bad go.mod file: %v", err)
		return rawLatest, nil
		// return "", err
	}

	// Get the cooked latest version by disallowing retracted versions.
	unretractedVersions := slices.DeleteFunc(slices.Clone(allVersions),
		func(v string) bool { return isRetracted(modFile, v) })
	if len(allVersions) == len(unretractedVersions) {
		return rawLatest, nil
	}
	// This can return the empty string if all versions are retracted.
	return versions.Latest(unretractedVersions, hasGoMod)
}

var errNoVersions = errors.New("no versions from proxy")

// isRetracted reports whether the go.mod file retracts the version.
func isRetracted(mf *modfile.File, resolvedVersion string) bool {
	for _, r := range mf.Retract {
		if semver.Compare(resolvedVersion, r.Low) >= 0 && semver.Compare(resolvedVersion, r.High) <= 0 {
			return true
		}
	}
	return false
}
