package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// TestCommittedGrammarsAreCurrent is the drift gate: the grammars the extension
// ships must be what the current keyword list generates.
func TestCommittedGrammarsAreCurrent(t *testing.T) {
	for _, g := range Grammars() {
		want, err := Render(g)
		if err != nil {
			t.Fatalf("Render(%s) err = %v", g.File, err)
		}
		path := filepath.Join("..", "..", "syntaxes", g.File)
		got, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is stale; regenerate it with `make vscode-grammar`", path)
		}
	}
}

func TestEveryKeywordIsHighlighted(t *testing.T) {
	data, err := Render(Grammars()[0])
	if err != nil {
		t.Fatalf("Render err = %v", err)
	}
	var doc struct {
		Repository map[string]struct {
			Match string `json:"match"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var matchers []*regexp.Regexp
	for name, rule := range doc.Repository {
		if !strings.HasPrefix(name, "keywords-") && name != "constants" {
			continue
		}
		re, err := regexp.Compile(strings.ReplaceAll(rule.Match, `\b`, ``))
		if err != nil {
			t.Fatalf("rule %s has an invalid regex %q: %v", name, rule.Match, err)
		}
		matchers = append(matchers, re)
	}

	for _, kw := range lexer.Keywords() {
		matched := false
		for _, re := range matchers {
			if re.FindString(kw) == kw {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("keyword %q is not matched by any generated keyword rule", kw)
		}
	}
}

func TestScopesAreQualifiedPerLanguage(t *testing.T) {
	for _, g := range Grammars() {
		data, err := Render(g)
		if err != nil {
			t.Fatalf("Render(%s) err = %v", g.File, err)
		}
		if !strings.Contains(string(data), "keyword.declaration."+g.ScopeName) {
			t.Errorf("%s does not scope its keywords to %s", g.File, g.ScopeName)
		}
	}
}
