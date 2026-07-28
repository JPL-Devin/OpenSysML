package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func nameresCtx(t *testing.T, name, src string) (*Context, *ast.RootNamespace) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	return NewContext(name, idx, nil), root
}

func TestNameResolutionPassReportsUnresolved(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { alias A for P::Missing; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) == 0 {
		t.Fatalf("expected an unresolved diagnostic, got none")
	}
	d := got[0]
	if d.Source != "name-resolution" || d.Code != "unresolved" || d.Severity != SeverityError {
		t.Fatalf("got %+v, want source=name-resolution code=unresolved severity=error", d)
	}
}

func TestNameResolutionPassCleanWhenAllResolve(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { namespace N; alias A for P::N; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

func TestNameResolutionPassLevel(t *testing.T) {
	if (NameResolutionPass{}).Level() != LevelNameResolution {
		t.Fatalf("Level() = %v, want LevelNameResolution", (NameResolutionPass{}).Level())
	}
}
