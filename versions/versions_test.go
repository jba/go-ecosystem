// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package versions

import (
	"testing"
)

func TestLatestOf(t *testing.T) {
	for _, test := range []struct {
		name     string
		versions []string
		want     string
	}{
		{
			name:     "highest release",
			versions: []string{"v1.2.3", "v1.0.0", "v1.9.0-pre"},
			want:     "v1.2.3",
		},
		{
			name:     "highest pre-release if no release",
			versions: []string{"v1.2.3-alpha", "v1.0.0-beta", "v1.9.0-pre"},
			want:     "v1.9.0-pre",
		},
		{
			name:     "prefer pre-release to pseudo",
			versions: []string{"v1.0.0-20180713131340-b395d2d6f5ee", "v0.0.0-alpha"},
			want:     "v0.0.0-alpha",
		},

		{
			name:     "highest pseudo if no pre-release or release",
			versions: []string{"v0.0.0-20180713131340-b395d2d6f5ee", "v0.0.0-20190124233150-8f7fa2680c82"},
			want:     "v0.0.0-20190124233150-8f7fa2680c82",
		},
		{
			name:     "use incompatible",
			versions: []string{"v1.2.3", "v1.0.0", "v2.0.0+incompatible"},
			want:     "v2.0.0+incompatible",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := LatestOf(test.versions)
			if got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestLatestVersion(t *testing.T) {
	pseudo := "v0.0.0-20190124233150-8f7fa2680c82"
	for _, test := range []struct {
		name     string
		versions []string
		hasGoMod func(string) (bool, error)
		want     string
	}{
		{
			name:     "empty",
			versions: nil,
			want:     "",
		},
		{
			name:     "tagged release",
			versions: []string{pseudo, "v0.1.0", "v1.2.3-pre"},
			want:     "v0.1.0",
		},
		{
			name:     "tagged pre-release",
			versions: []string{pseudo, "v1.2.3-pre"},
			want:     "v1.2.3-pre",
		},
		{
			name:     "pseudo",
			versions: []string{pseudo},
			want:     pseudo,
		},
		{
			name:     "incompatible with go.mod",
			versions: []string{"v2.0.0+incompatible", "v1.2.3"},
			want:     "v1.2.3",
		},
		{
			name:     "incompatible without go.mod",
			versions: []string{"v2.0.0+incompatible", "v1.2.3"},
			hasGoMod: func(v string) (bool, error) { return false, nil },
			want:     "v2.0.0+incompatible",
		},
		{
			name: "incompatible without tagged go.mod",
			// Although the latest compatible version has a go.mod file,
			// it is not a tagged version.
			versions: []string{pseudo, "v2.0.0+incompatible"},
			want:     "v2.0.0+incompatible",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.hasGoMod == nil {
				test.hasGoMod = func(v string) (bool, error) { return true, nil }
			}
			got, err := Latest(test.versions, test.hasGoMod)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
