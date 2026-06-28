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
