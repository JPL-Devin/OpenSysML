package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contains reports whether candidates holds want.
func contains(candidates []string, want string) bool {
	for _, c := range candidates {
		if c == want {
			return true
		}
	}
	return false
}

// TestCompleteMetaCommands covers completing a command at the start of a line.
func TestCompleteMetaCommands(t *testing.T) {
	tests := []struct {
		line    string
		wants   []string
		rejects []string
	}{
		{line: "%", wants: []string{"%help", "%eval", "%search", "%builtins"}},
		{line: "%ev", wants: []string{"%eval"}, rejects: []string{"%help"}},
		{line: "%se", wants: []string{"%search"}, rejects: []string{"%eval"}},
		{line: "  %bui", wants: []string{"%builtins"}},
		{line: "%zzzz", rejects: []string{"%eval", "%help"}},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := NewSession().Complete(tt.line, len(tt.line))
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("completing %q: want %q in %v", tt.line, want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("completing %q: did not want %q in %v", tt.line, bad, got.Candidates)
				}
			}
			if want := strings.TrimLeft(tt.line, " \t"); got.Prefix != want {
				t.Errorf("completing %q: prefix = %q, want %q", tt.line, got.Prefix, want)
			}
		})
	}
}

// TestCompleteNames covers completing declared, builtin and library names.
func TestCompleteNames(t *testing.T) {
	s := NewSession()
	if res := s.Submit("part def Wheel { attribute diameter = 1.0; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
	}

	tests := []struct {
		name    string
		line    string
		wants   []string
		rejects []string
	}{
		{name: "declared name", line: "%eval Whe", wants: []string{"Wheel"}},
		{name: "declared member", line: "%eval diam", wants: []string{"diameter"}},
		{name: "builtin function", line: "%eval sqr", wants: []string{"sqrt"}},
		{name: "library namespace", line: "%eval ScalarVal", wants: []string{"ScalarValues"}},
		{
			name:    "one qualified segment at a time",
			line:    "%eval ScalarValues::",
			wants:   []string{"ScalarValues::Integer"},
			rejects: []string{"ScalarValues::Integer::"},
		},
		{
			// A completed namespace still answers with its members: offering
			// the typed word back inserts nothing.
			name:    "a whole namespace offers its members",
			line:    "%eval ScalarValues",
			wants:   []string{"ScalarValues::Integer"},
			rejects: []string{"ScalarValues", "ScalarValues::Integer::"},
		},
		{name: "inside an expression", line: "%eval 1 + Whe", wants: []string{"Wheel"}},
		{name: "no match", line: "%eval zzzznotaname", rejects: []string{"Wheel", "sqrt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Complete(tt.line, len(tt.line))
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("completing %q: want %q in %v", tt.line, want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("completing %q: did not want %q in %v", tt.line, bad, got.Candidates)
				}
			}
			for _, c := range got.Candidates {
				if !strings.HasPrefix(c, got.Prefix) {
					t.Errorf("candidate %q does not extend prefix %q", c, got.Prefix)
				}
			}
			if len(got.Candidates) > completionLimit {
				t.Errorf("got %d candidates, want at most %d", len(got.Candidates), completionLimit)
			}
		})
	}
}

// TestCompletePaths covers the file paths %load and %save complete to.
func TestCompletePaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"model.sysml", "other.sysml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("part def A;\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tests := []struct {
		name    string
		line    string
		wants   []string
		rejects []string
	}{
		{
			name:  "every entry of a directory",
			line:  "%load " + dir + "/",
			wants: []string{dir + "/model.sysml", dir + "/other.sysml", dir + "/nested/"},
		},
		{
			name:    "narrowed by what is typed",
			line:    "%load " + dir + "/mod",
			wants:   []string{dir + "/model.sysml"},
			rejects: []string{dir + "/other.sysml"},
		},
		{
			name:  "%save completes paths too",
			line:  "%save " + dir + "/oth",
			wants: []string{dir + "/other.sysml"},
		},
		{
			name:    "missing directory offers nothing",
			line:    "%load " + dir + "/nowhere/x",
			rejects: []string{dir + "/model.sysml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSession().Complete(tt.line, len(tt.line))
			for _, want := range tt.wants {
				if !contains(got.Candidates, want) {
					t.Errorf("completing %q: want %q in %v", tt.line, want, got.Candidates)
				}
			}
			for _, bad := range tt.rejects {
				if contains(got.Candidates, bad) {
					t.Errorf("completing %q: did not want %q in %v", tt.line, bad, got.Candidates)
				}
			}
		})
	}
}

// TestCompletePosition checks completion answers about the word at the cursor,
// not the end of the line.
func TestCompletePosition(t *testing.T) {
	s := NewSession()
	line := "%eval sqr + 1"
	got := s.Complete(line, len("%eval sqr"))
	if !contains(got.Candidates, "sqrt") {
		t.Errorf("want sqrt in %v", got.Candidates)
	}
	if got.Prefix != "sqr" {
		t.Errorf("prefix = %q, want %q", got.Prefix, "sqr")
	}
	// An out-of-range position is answered about the whole line.
	if out := s.Complete(line, len(line)+10); out.Prefix != "1" {
		t.Errorf("out-of-range position: prefix = %q, want %q", out.Prefix, "1")
	}
	if out := s.Complete(line, -1); out.Prefix != "1" {
		t.Errorf("negative position: prefix = %q, want %q", out.Prefix, "1")
	}
}
