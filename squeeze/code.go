package squeeze

import (
	"archive/zip"
	"go/scanner"
	"go/token"
	"io"
	"path"

	"github.com/jba/go-ecosystem/zips"
	"github.com/jba/huffman"
)

// buildCode builds a Huffman code from all the .go files in zr. The code's
// symbols are the distinct Go tokens appearing in those files, and their
// frequencies are the number of times each token occurs.
func buildCode(zr *zip.Reader) (*huffman.Code, error) {
	// A symbolizer assigns each distinct token text a dense symbol index. The
	// same instance is shared across all files so symbols are consistent.
	sym := &symbolizer{index: map[string]huffman.Symbol{}}
	cb := huffman.NewCodeBuilder(sym.split)

	for _, f := range zr.File {
		// Only Go source files; go.mod is a source file per zips.IsSourceName
		// but is not Go code, so exclude it.
		if !zips.IsSourceName(f.Name) || path.Base(f.Name) == "go.mod" {
			continue
		}
		src, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		sym.filename = f.Name
		if _, err := cb.Write(src); err != nil {
			return nil, err
		}
	}
	return cb.Code()
}

// A symbolizer maps Go token texts to dense [huffman.Symbol] indices.
type symbolizer struct {
	index    map[string]huffman.Symbol
	filename string // name of the file currently being split, for the scanner
}

// split tokenizes Go source and returns the symbol for each token. It satisfies
// [huffman.SplitFunc].
func (s *symbolizer) split(src []byte) []huffman.Symbol {
	fset := token.NewFileSet()
	file := fset.AddFile(s.filename, fset.Base(), len(src))
	var sc scanner.Scanner
	// Suppress error handling: we tokenize on a best-effort basis.
	sc.Init(file, src, nil, scanner.ScanComments)

	var syms []huffman.Symbol
	for {
		_, tok, lit := sc.Scan()
		if tok == token.EOF {
			return syms
		}
		// Literal-bearing tokens (identifiers, numbers, strings, comments)
		// carry their text in lit; operators and keywords use the token's
		// canonical string.
		text := lit
		if text == "" {
			text = tok.String()
		}
		syms = append(syms, s.symbol(text))
	}
}

// symbol returns the symbol for the given token text, assigning a new one if
// necessary.
func (s *symbolizer) symbol(text string) huffman.Symbol {
	sym, ok := s.index[text]
	if !ok {
		sym = huffman.Symbol(len(s.index))
		s.index[text] = sym
	}
	return sym
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
