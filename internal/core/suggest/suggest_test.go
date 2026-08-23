package suggest_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestEditDistance locks the distances the suggestions rank on, including the
// transposition that a swapped pair of letters is one edit away.
func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "eval", b: "eval", want: 0},
		{a: "evl", b: "eval", want: 1},
		{a: "eavl", b: "eval", want: 1},
		{a: "isntances", b: "instances", want: 1},
		{a: "", b: "eval", want: 4},
		{a: "abcd", b: "", want: 4},
		{a: "wheel", b: "whel", want: 1},
		{a: "cat", b: "dog", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			if got := suggest.EditDistance(tt.a, tt.b); got != tt.want {
				t.Errorf("suggest.EditDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestNearest covers the tolerance a typo of each length gets, and the order
// the candidates come back in.
func TestNearest(t *testing.T) {
	names := []string{"Integer", "Real", "String", "instances", "eval"}
	tests := []struct {
		word string
		want []string
	}{
		{word: "Intger", want: []string{"Integer"}},
		{word: "isntances", want: []string{"instances"}},
		{word: "Rael", want: []string{"Real"}},
		{word: "eval", want: nil}, // a name that is itself is not a suggestion
		{word: "Zzzzqqqqwwww", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			got := suggest.Nearest(tt.word, names)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("suggest.Nearest(%q) = %v, want %v", tt.word, got, tt.want)
			}
		})
	}
}

// TestQualifiedOverLibrary covers the qualified spellings the bundled library
// offers for a bare name, and that a re-export is not offered as a declaration.
func TestQualifiedOverLibrary(t *testing.T) {
	idx := libraryIndex(t)
	tests := []struct {
		name    string
		want    string
		rejects []string
	}{
		{name: "Integer", want: "ScalarValues::Integer", rejects: []string{"SysML::Integer", "KerML::Integer"}},
		{name: "String", want: "ScalarValues::String"},
		{name: "Zzzznotatype", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest.Qualified(idx, tt.name)
			if tt.want == "" {
				if len(got) > 0 {
					t.Fatalf("suggest.Qualified(%q) = %v, want none", tt.name, got)
				}
				return
			}
			if len(got) == 0 || got[0] != tt.want {
				t.Fatalf("suggest.Qualified(%q) = %v, want %q first", tt.name, got, tt.want)
			}
			for _, bad := range tt.rejects {
				for _, c := range got {
					if c == bad {
						t.Errorf("suggest.Qualified(%q) offered the re-export %q", tt.name, bad)
					}
				}
			}
		})
	}
}

// TestWithAndOrList covers how a suggestion reads, and that a candidate equal
// to the word it explains is not offered back.
func TestWithAndOrList(t *testing.T) {
	tests := []struct {
		name       string
		word       string
		candidates []string
		want       string
	}{
		{name: "none", word: "x", want: "unresolved reference: x"},
		{name: "one", word: "x", candidates: []string{"A::x"}, want: "unresolved reference: x — did you mean A::x?"},
		{
			name:       "several",
			word:       "x",
			candidates: []string{"A::x", "B::x", "C::x"},
			want:       "unresolved reference: x — did you mean A::x, B::x or C::x?",
		},
		{name: "itself", word: "x", candidates: []string{"x"}, want: "unresolved reference: x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest.With("unresolved reference: "+tt.word, tt.word, tt.candidates)
			if got != tt.want {
				t.Errorf("suggest.With(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

// libraryIndex indexes the bundled standard library.
func libraryIndex(t *testing.T) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	libs.LoadInto(idx)
	return idx
}
