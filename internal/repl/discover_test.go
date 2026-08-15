package repl

import (
	"strings"
	"testing"
)

// TestSearchCommand covers what %search answers over the bundled library and
// the session's own declarations.
func TestSearchCommand(t *testing.T) {
	tests := []struct {
		name     string
		declare  string
		line     string
		wants    []string
		rejects  []string
		maxLines int
	}{
		{
			name:  "library declaration with its kind",
			line:  "%search Integer",
			wants: []string{"ScalarValues::Integer  attributeDef"},
		},
		{
			name: "re-export aliases are not listed as declarations",
			line: "%search Integer",
			rejects: []string{
				"\nSysML::Integer",
				"\nKerML::Integer",
			},
		},
		{
			name:  "case insensitive",
			line:  "%search scalarvalues",
			wants: []string{"ScalarValues"},
		},
		{
			name:    "session declarations are searched",
			declare: "part def Wheel { attribute diameter = 1.0; }",
			line:    "%search Wheel",
			wants:   []string{"Wheel  partDef"},
		},
		{
			name:  "no match",
			line:  "%search zzzznotasymbol",
			wants: []string{`no symbol matches "zzzznotasymbol"`},
		},
		{
			name:     "listing is bounded",
			line:     "%search a",
			wants:    []string{"more; narrow the search"},
			maxLines: searchLimit + 1,
		},
		{
			name:  "usage without an argument",
			line:  "%search",
			wants: []string{"usage: %search <substring>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			if tt.declare != "" {
				if res := s.Submit(tt.declare); len(res.Diagnostics) > 0 {
					t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
				}
			}
			out, _, err := s.RunMeta(tt.line)
			if err != nil {
				t.Fatalf("%s: %v", tt.line, err)
			}
			got := strings.Join(out, "\n")
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
			if tt.maxLines > 0 && len(out) > tt.maxLines {
				t.Errorf("got %d lines, want at most %d", len(out), tt.maxLines)
			}
		})
	}
}

// TestBuiltinsCommand checks %builtins reports the functions the runtime
// implements, in the form each is called in.
func TestBuiltinsCommand(t *testing.T) {
	out := run(t, NewSession(), "%builtins")
	wants(t, out,
		"sqrt(x)  RealFunctions::sqrt",
		"abs(x)  ",
		"max(x, y)  ",
		"floor(x)  ",
		"x->isEmpty()  SequenceFunctions::isEmpty",
		"x->sum()  NumericalFunctions::sum",
	)
}

// TestQualifiedNameSuggestion covers the hint an unresolved bare library name
// gets: the qualified name the symbol index already knows it under.
func TestQualifiedNameSuggestion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wants   []string
		rejects []string
	}{
		{
			name:  "bare library name",
			input: "part def A { attribute x : Integer = 1; }",
			wants: []string{"unresolved reference: Integer — did you mean ScalarValues::Integer?"},
		},
		{
			name:    "qualified name gets no unqualified hint",
			input:   "part def B { attribute x : NoSuchLib::Integer = 1; }",
			rejects: []string{"did you mean"},
		},
		{
			name:    "unknown name gets no hint",
			input:   "part def C { attribute x : Zzzznotatype = 1; }",
			rejects: []string{"did you mean"},
		},
		{
			name:    "resolvable name has no diagnostic",
			input:   "part def D { attribute x : ScalarValues::Integer = 1; }",
			rejects: []string{"unresolved reference"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			res := s.Submit(tt.input)
			var msgs []string
			for _, d := range res.Diagnostics {
				msgs = append(msgs, d.Message)
			}
			got := strings.Join(msgs, "\n")
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
		})
	}
}

// TestUnknownCommandSuggestion covers the edit-distance hint a misspelled meta
// command gets, and the fallback when nothing is close.
func TestUnknownCommandSuggestion(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{cmd: "%evl", want: `unknown command "%evl" — did you mean %eval?`},
		{cmd: "%hepl", want: `unknown command "%hepl" — did you mean %help?`},
		{cmd: "%isntances", want: `unknown command "%isntances" — did you mean %instances?`},
		{cmd: "%zzzzqqqq", want: `unknown command "%zzzzqqqq" (try %help)`},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := unknownCommandLine(tt.cmd); got != tt.want {
				t.Errorf("unknownCommandLine(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

// TestSymbolSuggestion covers the hints a name no declaration answers to gets.
func TestSymbolSuggestion(t *testing.T) {
	tests := []struct {
		name    string
		declare string
		line    string
		wants   []string
	}{
		{
			name:    "misspelled declaration",
			declare: "part def Wheel { attribute diameter = 1.0; }",
			line:    "%instantiate Whel",
			wants:   []string{"unresolved reference: Whel", "did you mean Wheel?"},
		},
		{
			name:  "bare library name is offered qualified",
			line:  "%instantiate Integer",
			wants: []string{"unresolved reference: Integer", "did you mean ScalarValues::Integer?"},
		},
		{
			name:    "nothing close",
			declare: "part def Wheel { attribute diameter = 1.0; }",
			line:    "%instantiate Zzzzqqqqwwww",
			wants:   []string{"unresolved reference: Zzzzqqqqwwww"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			if tt.declare != "" {
				if res := s.Submit(tt.declare); len(res.Diagnostics) > 0 {
					t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
				}
			}
			wants(t, run(t, s, tt.line), tt.wants...)
		})
	}
}

// TestLibraryReachableFromEmptySession checks library symbols answer browsing,
// lookup and instantiation before the session declares anything.
func TestLibraryReachableFromEmptySession(t *testing.T) {
	tests := []struct {
		line    string
		wants   []string
		rejects []string
	}{
		{line: "%search mass", wants: []string{"ISQBase::mass"}},
		{line: "%eval ISQ::mass", rejects: []string{"no declarations loaded"}},
		{line: "%instantiate ISQ::mass", wants: []string{"✓ Created instance of ISQBase::mass"}},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := run(t, NewSession(), tt.line)
			wants(t, got, tt.wants...)
			rejects(t, got, tt.rejects...)
		})
	}
}

// TestAnnotateBuildsIndexOnlyForDiagnostics checks a submission that resolves
// cleanly does not index the library just to look for names to suggest.
func TestAnnotateBuildsIndexOnlyForDiagnostics(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool // an index was built
	}{
		{name: "clean submission", input: "part def A;", want: false},
		{name: "unresolved reference", input: "part def B { attribute x : Integer = 1; }", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			s.annotateDiagnostics(s.Submit(tt.input).Diagnostics)
			if got := s.idx != nil; got != tt.want {
				t.Fatalf("index built = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			if got := editDistance(tt.a, tt.b); got != tt.want {
				t.Errorf("editDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
