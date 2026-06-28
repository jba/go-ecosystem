package squeeze

import (
	"archive/zip"
	"io"
	"path"
	"unicode"
	"unicode/utf8"

	"github.com/jba/go-ecosystem/zips"
	"github.com/jba/huffman"
)

// buildCode builds a Huffman code from all the .go files in zr. The code's
// symbols are produced by tokenizing the source (see [symbolizer.split]), and
// their frequencies are the number of times each token occurs.
func buildCode(zr *zip.Reader) (*huffman.Code, error) {
	// A symbolizer assigns each distinct token a dense symbol index. The same
	// instance is shared across all files so symbols are consistent.
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
		if _, err := cb.Write(src); err != nil {
			return nil, err
		}
	}
	return cb.Code()
}

// A symbolizer maps token texts to dense [huffman.Symbol] indices.
type symbolizer struct {
	index map[string]huffman.Symbol
}

// split splits Go source into symbols. It satisfies [huffman.SplitFunc].
//
// The split is lossless and never fails: concatenating the texts of the
// returned symbols always reproduces src exactly, for any input bytes.
//
// The tokens are, in priority order:
//   - a valid Go identifier (a Unicode letter or '_' followed by letters,
//     digits, or '_'), even if it includes digits;
//   - a single decimal digit;
//   - a Go operator or punctuation token (none of which contain letters or
//     digits), matched greedily so longer tokens win (e.g. "<<=" over "<<");
//   - a single whitespace character;
//   - any other single byte.
func (s *symbolizer) split(src []byte) []huffman.Symbol {
	var syms []huffman.Symbol
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		var tok []byte
		switch {
		case isIdentStart(r):
			j := i + size
			for j < len(src) {
				r2, s2 := utf8.DecodeRune(src[j:])
				if !isIdentPart(r2) {
					break
				}
				j += s2
			}
			tok = src[i:j]
		case unicode.IsDigit(r):
			tok = src[i : i+size] // a single digit rune
		default:
			if op := matchOperator(src[i:]); op != "" {
				tok = src[i : i+len(op)]
			} else if unicode.IsSpace(r) {
				tok = src[i : i+size]
			} else {
				tok = src[i : i+1] // any other single byte
			}
		}
		syms = append(syms, s.symbol(string(tok)))
		i += len(tok)
	}
	return syms
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

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// goOperators are the multi-character Go operator and punctuation tokens,
// ordered longest first so that matchOperator finds the greedy match.
// Single-character operators are omitted: they are covered by the default
// single-byte case, which produces the same token text.
var goOperators = []string{
	"<<=", ">>=", "&^=", "...",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"<<", ">>", "&^", "&&", "||", "<-", "++", "--",
	"==", "!=", "<=", ">=", ":=",
}

// matchOperator returns the longest Go operator token that is a prefix of src,
// or "" if none is.
func matchOperator(src []byte) string {
	for _, op := range goOperators {
		if len(src) >= len(op) && string(src[:len(op)]) == op {
			return op
		}
	}
	return ""
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
