package squeeze

import (
	"archive/zip"
	"go/scanner"
	"go/token"
	"io"
	"path"

	"github.com/jba/huffman"
)

// buildCode builds a Huffman code from all the .go files in zr. The code's
// symbols are the distinct Go tokens appearing in those files, and their
// frequencies are the number of times each token occurs.
func buildCode(zr *zip.Reader) (*huffman.Code, error) {
	// Assign each distinct token text a dense symbol index, and count how
	// often each occurs.
	syms := map[string]huffman.Symbol{}
	var freqs []int
	tally := func(s string) {
		sym, ok := syms[s]
		if !ok {
			sym = huffman.Symbol(len(freqs))
			syms[s] = sym
			freqs = append(freqs, 0)
		}
		freqs[sym]++
	}

	for _, f := range zr.File {
		if path.Ext(f.Name) != ".go" {
			continue
		}
		src, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		if err := tokenize(f.Name, src, tally); err != nil {
			return nil, err
		}
	}
	return huffman.NewCode(freqs)
}

// tokenize scans the Go source src, calling emit with the text of each token.
func tokenize(filename string, src []byte, emit func(string)) error {
	fset := token.NewFileSet()
	file := fset.AddFile(filename, fset.Base(), len(src))
	var s scanner.Scanner
	// Suppress error handling: we tokenize on a best-effort basis.
	s.Init(file, src, nil, scanner.ScanComments)
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return nil
		}
		// Literal-bearing tokens (identifiers, numbers, strings, comments)
		// carry their text in lit; operators and keywords use the token's
		// canonical string.
		text := lit
		if text == "" {
			text = tok.String()
		}
		emit(text)
	}
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
