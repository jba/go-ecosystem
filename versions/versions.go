// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copied from pkgsite's internal/version package at
// commit 035bfc02f3faa0221e0edf90b0a21d3619c95fdd.

// Package versions provides support for Go version strings.
package versions

import (
	"log"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// Later reports whether v1 is later than v2, using semver but preferring
// release versions to pre-release versions, and both to pseudo-versions.
func Later(v1, v2 string) bool {
	rel1 := semver.Prerelease(v1) == ""
	rel2 := semver.Prerelease(v2) == ""
	if rel1 && rel2 {
		return semver.Compare(v1, v2) > 0
	}
	if rel1 != rel2 {
		return rel1
	}
	// Both are pre-release.
	pseudo1 := module.IsPseudoVersion(v1)
	pseudo2 := module.IsPseudoVersion(v2)
	if pseudo1 == pseudo2 {
		return semver.Compare(v1, v2) > 0
	}
	return !pseudo1
}

// LatestOf returns the latest version of a module from a list of versions, using
// the go command's definition of latest: semver is observed, except that
// release versions are preferred to prerelease, and both are preferred to pseudo-versions.
// If versions is empty, the empty string is returned.
func LatestOf(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if Later(v, latest) {
			latest = v
		}
	}
	return latest
}

// Latest finds the latest version of a module using the same algorithm as the
// Go command. It prefers tagged release versions to tagged pre-release
// versions, and both of those to pseudo-versions. If versions is empty, Latest
// returns the empty string.
//
// hasGoMod should report whether the version it is given has a go.mod file.
// Latest returns the latest incompatible version only if the latest compatible
// version does not have a go.mod file.
//
// The meaning of latest is defined at
// https://golang.org/ref/mod#version-queries. That definition does not deal
// with retractions, or with a subtlety involving incompatible versions. The
// actual definition is embodied in the go command's queryMatcher.filterVersions
// method. This function is a re-implementation and specialization of that
// method at Go version 1.16
// (https://go.googlesource.com/go/+/refs/tags/go1.16/src/cmd/go/internal/modload/query.go#441).
func Latest(versions []string, hasGoMod func(v string) (bool, error)) (v string, err error) {
	latest := LatestOf(versions)
	if latest == "" {
		return "", nil
	}
	// If the latest is a compatible version, use it.
	if !IsIncompatible(latest) {
		return latest, nil
	}
	// The latest version is incompatible. If there is a go.mod file at the
	// latest compatible tagged version, assume the module author has adopted
	// proper versioning, and use that latest compatible version. Otherwise, use
	// this incompatible version.
	compats := slices.DeleteFunc(slices.Clone(versions),
		func(v string) bool { return IsIncompatible(v) || module.IsPseudoVersion(v) })
	latestCompat := LatestOf(compats)
	if latestCompat == "" {
		// No compatible versions; use the latest (incompatible) version.
		log.Printf("using latest incompatible version")
		return latest, nil
	}
	latestCompatHasGoMod, err := hasGoMod(latestCompat)
	if err != nil {
		return "", err
	}
	if latestCompatHasGoMod {
		return latestCompat, nil
	}
	return latest, nil
}

// IsIncompatible reports whether a valid version v is an incompatible version.
func IsIncompatible(v string) bool {
	return strings.HasSuffix(v, "+incompatible")
}
