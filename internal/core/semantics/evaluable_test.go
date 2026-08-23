package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evaluableIn parses expr as an expression written at the root of src and
// reports whether the model can evaluate it.
func evaluableIn(t *testing.T, src, expr string) bool {
	t.Helper()
	m, root := buildModel(t, src)
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("failed to parse %q: %v", expr, p.Diagnostics)
	}
	return m.ModelLevelEvaluable(root, node)
}

const evaluableModel = `part def A { attribute y; }
enum def E { enum e; }
attribute k = 2;
attribute unset;
calc def f { in x; return : ScalarValues::Integer = 1; }
`

// A metadata feature value and a filter condition must be decidable from the
// model alone (KerML §7.4.9, Expression::isModelLevelEvaluable).
func TestModelLevelEvaluable(t *testing.T) {
	for expr, want := range map[string]bool{
		"1":                       true,
		"1.5":                     true,
		`"e"`:                     true,
		"true":                    true,
		"null":                    true,
		"*":                       true,
		"1 + 2":                   true,
		"true and (1 < 2)":        true,
		"not (2 == 2)":            true,
		"(1, 2, 3)":               true,
		"E::e":                    true,
		"k":                       true,
		"k + 1":                   true,
		"new A(null, 1, \"\")":    true,
		"~3":                      false,
		"unset":                   false,
		"(as A).y":                false,
		"f((as A).y)":             false,
		"f(1)":                    false,
		"(as A).y->Base::size()":  false,
		"new A(null, (as A).y)":   false,
		"nowhere":                 false,
		"(1, (as A).y)":           false,
		"true and ((as A).y > 1)": false,
	} {
		if got := evaluableIn(t, evaluableModel, expr); got != want {
			t.Errorf("ModelLevelEvaluable(%q) = %v, want %v", expr, got, want)
		}
	}
}

// A library function is one a model-level evaluation may call; a function the
// model under validation declares is not.
func TestModelLevelEvaluableCallsOnlyLibraryFunctions(t *testing.T) {
	const lib = `standard library package Kit { calc def twice { in x; return : ScalarValues::Integer = 2; } }`
	p := parser.New(source.New("kit.sysml", []byte(lib)))
	libRoot := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("library parse diagnostics: %v", p.Diagnostics)
	}
	q := parser.New(source.New("t.sysml", []byte("calc def once { in x; return : ScalarValues::Integer = 1; }")))
	root := q.ParseFile()
	if len(q.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", q.Diagnostics)
	}

	idx := symbols.NewIndex()
	idx.AddDocument("kit.sysml", libRoot)
	idx.AddDocument("t.sysml", root)
	idx.MarkLibrary("kit.sysml")
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument("kit.sysml", libRoot)
	r.ResolveDocument("t.sysml", root)
	scope := idx.DocumentRoot("t.sysml")

	for expr, want := range map[string]bool{
		"Kit::twice(2)": true,
		"once(2)":       false,
	} {
		e := parser.New(source.New("<expr>", []byte(expr))).ParseExpression()
		if e == nil {
			t.Fatalf("failed to parse %q", expr)
		}
		if got := m.ModelLevelEvaluable(scope, e); got != want {
			t.Errorf("ModelLevelEvaluable(%q) = %v, want %v", expr, got, want)
		}
	}
}
