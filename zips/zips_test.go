package zips

import "testing"

func TestIsSourceName(t *testing.T) {
	for _, test := range []struct {
		name string
		want bool
	}{
		{"go.mod", true},
		{"foo.go", true},
		{"foo/bar.go", true},
		{"foo/go.mod", true},
		{"_underscore/x.go", true}, // "_" dirs are accepted

		{"README.md", false},
		{"foo/bar.txt", false},
		{"foo/", false}, // directory entry
		{"vendor/x/y.go", false},
		{"foo/vendor/x/y.go", false},
		{"Godeps/x/y.go", false},
		{"foo/Godeps/x/y.go", false},
		{"testdata/x.go", false},
		{"foo/testdata/x.go", false},
		{".hidden/x.go", false},
		{"foo/.hidden/x.go", false},
	} {
		if got := IsSourceName(test.name); got != test.want {
			t.Errorf("IsSourceName(%q) = %t, want %t", test.name, got, test.want)
		}
	}
}
