package squeeze

import (
	"archive/zip"
	"bytes"
	"slices"

	"github.com/jba/huffman"
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

func TestSplitLossless(t *testing.T) {
	inputs := []string{
		"package p\n\nfunc F() int { return 1 + 1 }\n",
		"a := b << 2; c &^= d\n",
		"αβγ_x9 := \"héllo\" // comment\n\t→ \xff\xfe",
		"x123 y_4_z _ 007 ...rest\n",
		"",
	}
	for _, in := range inputs {
		s := &symbolizer{index: map[string]huffman.Symbol{}}
		syms := s.split([]byte(in))
		// Reconstruct by inverting the index.
		rev := map[huffman.Symbol]string{}
		for text, sym := range s.index {
			rev[sym] = text
		}
		var sb []byte
		for _, sym := range syms {
			sb = append(sb, rev[sym]...)
		}
		if !slices.Equal(sb, []byte(in)) {
			t.Errorf("split not lossless\n in: %q\nout: %q", in, sb)
		}
	}
}

func TestSplitTokens(t *testing.T) {
	s := &symbolizer{index: map[string]huffman.Symbol{}}
	s.split([]byte("x9 + 12 <<= y_3"))
	want := []string{"x9", " ", "+", " ", "1", "2", " ", "<<=", " ", "y_3"}
	for _, w := range want {
		if _, ok := s.index[w]; !ok {
			t.Errorf("expected token %q to be a symbol", w)
		}
	}
	// "12" must be two single-digit symbols, not one token.
	if _, ok := s.index["12"]; ok {
		t.Errorf("digits should be individual symbols; got combined %q", "12")
	}
}
