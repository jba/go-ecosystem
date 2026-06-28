package squeeze

import (
	"bytes"
	"testing"
)

// goOperatorBytes is the byte-slice form of goOperators, in the same order.
var goOperatorBytes = func() [][]byte {
	bs := make([][]byte, len(goOperators))
	for i, op := range goOperators {
		bs[i] = []byte(op)
	}
	return bs
}()

// matchOperatorBytes is a variant of matchOperator that compares against
// precomputed byte slices with bytes.HasPrefix, avoiding the []byte->string
// conversion in the hot loop.
func matchOperatorBytes(src []byte) string {
	for i, op := range goOperatorBytes {
		if bytes.HasPrefix(src, op) {
			return goOperators[i]
		}
	}
	return ""
}

// matchOperatorSwitch dispatches on the first byte, then checks only the
// operators that can begin with it. Operators within a case are ordered
// longest-first to preserve greedy matching.
func matchOperatorSwitch(src []byte) string {
	if len(src) == 0 {
		return ""
	}
	has := func(s string) bool { return len(src) >= len(s) && string(src[:len(s)]) == s }
	switch src[0] {
	case '<':
		if has("<<=") {
			return "<<="
		}
		if has("<<") {
			return "<<"
		}
		if has("<-") {
			return "<-"
		}
		if has("<=") {
			return "<="
		}
	case '>':
		if has(">>=") {
			return ">>="
		}
		if has(">>") {
			return ">>"
		}
		if has(">=") {
			return ">="
		}
	case '&':
		if has("&^=") {
			return "&^="
		}
		if has("&^") {
			return "&^"
		}
		if has("&&") {
			return "&&"
		}
		if has("&=") {
			return "&="
		}
	case '.':
		if has("...") {
			return "..."
		}
	case '+':
		if has("+=") {
			return "+="
		}
		if has("++") {
			return "++"
		}
	case '-':
		if has("-=") {
			return "-="
		}
		if has("--") {
			return "--"
		}
	case '*':
		if has("*=") {
			return "*="
		}
	case '/':
		if has("/=") {
			return "/="
		}
	case '%':
		if has("%=") {
			return "%="
		}
	case '|':
		if has("|=") {
			return "|="
		}
		if has("||") {
			return "||"
		}
	case '^':
		if has("^=") {
			return "^="
		}
	case '=':
		if has("==") {
			return "=="
		}
	case '!':
		if has("!=") {
			return "!="
		}
	case ':':
		if has(":=") {
			return ":="
		}
	}
	return ""
}

// benchInputs exercises the matchers: hits at various positions in the operator
// list (best/worst case), and a miss (the common case for arbitrary source).
var benchInputs = [][]byte{
	[]byte("<<= rest"), // first entry: best case
	[]byte(":= x"),     // last entry: worst case among matches
	[]byte("== y"),
	[]byte("identifier"), // miss
	[]byte(" "),          // miss (whitespace)
}

func BenchmarkMatchOperatorString(b *testing.B) {
	var sink string
	for b.Loop() {
		for _, in := range benchInputs {
			sink = matchOperator(in)
		}
	}
	_ = sink
}

func BenchmarkMatchOperatorBytes(b *testing.B) {
	var sink string
	for b.Loop() {
		for _, in := range benchInputs {
			sink = matchOperatorBytes(in)
		}
	}
	_ = sink
}

func BenchmarkMatchOperatorSwitch(b *testing.B) {
	var sink string
	for b.Loop() {
		for _, in := range benchInputs {
			sink = matchOperatorSwitch(in)
		}
	}
	_ = sink
}

// TestMatchOperatorVariantsAgree checks that the alternative implementations
// produce the same result as matchOperator for a range of inputs.
func TestMatchOperatorVariantsAgree(t *testing.T) {
	var inputs [][]byte
	for _, op := range goOperators {
		inputs = append(inputs, []byte(op), []byte(op+"x"))
	}
	inputs = append(inputs,
		[]byte(""), []byte("x"), []byte(" "), []byte("<"), []byte("&"),
		[]byte("<<"), []byte("<-"), []byte("..."), []byte(".."), []byte("&^="),
	)
	for _, in := range inputs {
		want := matchOperator(in)
		if got := matchOperatorBytes(in); got != want {
			t.Errorf("matchOperatorBytes(%q) = %q, want %q", in, got, want)
		}
		if got := matchOperatorSwitch(in); got != want {
			t.Errorf("matchOperatorSwitch(%q) = %q, want %q", in, got, want)
		}
	}
}
