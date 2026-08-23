package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

type stubPass struct {
	level PassLevel
	diags []Diagnostic
}

func (s stubPass) Level() PassLevel { return s.level }

func (s stubPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	return s.diags
}

func TestRegistryRunsPassesInLevelOrder(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []Diagnostic{{Severity: SeverityWarning, Source: "b"}}})
	reg.Register(stubPass{level: LevelSyntax, diags: []Diagnostic{{Severity: SeverityWarning, Source: "a"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Source != "a" || got[1].Source != "b" {
		t.Fatalf("passes ran out of level order: %q then %q", got[0].Source, got[1].Source)
	}
}

func TestRegistrySkipsHigherLevelAfterError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelSyntax, diags: []Diagnostic{{Severity: SeverityError, Source: "syntax"}}})
	reg.Register(stubPass{level: LevelType, diags: []Diagnostic{{Source: "type"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics, want 1 (type pass must be skipped)", len(got))
	}
	if got[0].Source != "syntax" {
		t.Fatalf("got source %q, want syntax", got[0].Source)
	}
}

type selfGatedStub struct{ stubPass }

func (selfGatedStub) SelfGated() {}

func TestRegistryRunsSelfGatedPassAfterError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelSyntax, diags: []Diagnostic{{Severity: SeverityError, Source: "syntax"}}})
	reg.Register(stubPass{level: LevelType, diags: []Diagnostic{{Source: "type"}}})
	reg.Register(selfGatedStub{stubPass{level: LevelType, diags: []Diagnostic{{Source: "self-gated"}}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 || got[1].Source != "self-gated" {
		t.Fatalf("want syntax plus self-gated only, got %v", got)
	}
}

func TestRegistrySameLevelNeverSkips(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []Diagnostic{{Severity: SeverityError, Source: "a"}}})
	reg.Register(stubPass{level: LevelNameResolution, diags: []Diagnostic{{Source: "b"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2 (same-level passes never skip)", len(got))
	}
}
