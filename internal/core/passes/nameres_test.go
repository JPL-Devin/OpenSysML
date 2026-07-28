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

// parseDoc parses src into a RootNamespace, failing on any parse diagnostic.
func parseDoc(t *testing.T, name, src string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics in %s: %+v", name, p.Diagnostics)
	}
	return root
}

func TestNameResolutionPassReportsAmbiguous(t *testing.T) {
	// Two documents each declare a top-level package P, so a qualified
	// reference whose first segment is P has two global candidates and the
	// resolver reports an ambiguous reference.
	rootA := parseDoc(t, "a.sysml", "package P { namespace X; }")
	rootB := parseDoc(t, "b.sysml", "package P { namespace Y; }")
	rootC := parseDoc(t, "c.sysml", "package Q { alias A for P::X; }")

	idx := symbols.NewIndex()
	idx.AddDocument("a.sysml", rootA)
	idx.AddDocument("b.sysml", rootB)
	idx.AddDocument("c.sysml", rootC)

	ctx := NewContext("c.sysml", idx, nil)
	got := NameResolutionPass{}.Run(ctx, "c.sysml", rootC)
	if len(got) == 0 {
		t.Fatalf("expected an ambiguous diagnostic, got none")
	}
	d := got[0]
	if d.Source != "name-resolution" || d.Code != "ambiguous" || d.Severity != SeverityError {
		t.Fatalf("got %+v, want source=name-resolution code=ambiguous severity=error", d)
	}
}
