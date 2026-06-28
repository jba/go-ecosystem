package squeeze

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestBuildCode(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"mod/go.mod": "module m\n",
		"mod/a.go":   "package p\n\nfunc F() int { return 1 + 1 }\n",
		"mod/b.go":   "package p\n\nvar X = F()\n",
		"mod/README": "ignore me",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	code, err := buildCode(zr)
	if err != nil {
		t.Fatal(err)
	}
	if code == nil {
		t.Fatal("got nil code")
	}
}
