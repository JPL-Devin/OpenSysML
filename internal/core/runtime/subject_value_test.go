package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// A declared subject's value part binds the subject for the check, whether it is
// written `= expr` (a feature value binding) or `default expr` (a fallback);
// either way an externally supplied subject wins.
func TestRequirementSubjectDeclarationValue(t *testing.T) {
	src := `package test {
		requirement Bound {
			subject speed : Real = 50;
			require speed < 100;
		}
		requirement Defaulted {
			subject speed : Real default 150;
			require speed < 100;
		}
	}`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)
	testPkg := idx.DocumentRoot("test.sysml").Children()[0]

	for _, tt := range []struct {
		name    string
		wantErr bool
	}{{"Bound", false}, {"Defaulted", true}} {
		sym, ok := testPkg.LookupLocal(tt.name)
		if !ok {
			t.Fatalf("%s not found", tt.name)
		}
		satisfied, err := ctx.EvaluateRequirement(sym, testPkg)
		switch {
		case tt.wantErr && err == nil:
			t.Errorf("%s: satisfied = %t, want the declared value to violate the condition", tt.name, satisfied)
		case !tt.wantErr && err != nil:
			t.Errorf("%s evaluation failed: %v", tt.name, err)
		case !tt.wantErr && !satisfied:
			t.Errorf("%s should be satisfied", tt.name)
		}
	}
}

// nestedSubjectSrc declares conditions at each level of a two-level nesting and
// an object redefining the leaf, so the declaration and the object answer
// differently — in both directions.
const nestedSubjectSrc = `package test {
	part def Leaf {
		attribute value : Real = 1.0;
		attribute other : Real = 2.0;
		constraint small { value < 10.0 }
		constraint overSibling { value > other }
	}
	part def Mid {
		part leaf : Leaf;
		constraint crossesBoundary { leaf.value < 10.0 }
	}
	part def Top {
		part mid : Mid;
	}
	part top : Top {
		part :>> mid {
			part :>> leaf {
				attribute :>> value = 99.0;
			}
		}
	}
	part quick : Top {
		part :>> mid {
			part :>> leaf {
				attribute :>> value = 3.0;
			}
		}
	}
}`

// nestedSubjectFixture indexes src and returns the runtime with the package its
// declarations live in.
func nestedSubjectFixture(t *testing.T, src string) (*Context, *symbols.Scope) {
	t.Helper()
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	root := idx.DocumentRoot("test.sysml")
	if len(root.Children()) == 0 {
		t.Fatal("no package indexed")
	}
	return NewContext(semantics.NewModel(resolver), resolver, 100000), root.Children()[0]
}

// memberPath looks a member up along a path of names, as `Leaf::small` is.
func memberPath(t *testing.T, scope *symbols.Scope, names ...string) *symbols.Symbol {
	t.Helper()
	var sym *symbols.Symbol
	for _, name := range names {
		if scope == nil {
			t.Fatalf("%s has no members", sym.Name)
		}
		found, ok := scope.LookupLocal(name)
		if !ok {
			t.Fatalf("%s not found", name)
		}
		sym, scope = found, found.Scope
	}
	return sym
}

// A condition naming a feature a nested object redefines is about that object,
// at whichever level the condition is declared: the redefinition on the object
// answers, not the default the declaration carries. Nothing materialized leaves
// the check about the declaration.
func TestNestedRedefinitionIsTheConditionSubject(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, nestedSubjectSrc)
	cases := []struct {
		path            []string
		declared, about bool // the verdict on the declaration, and on the object
	}{
		{[]string{"Leaf", "small"}, true, false},
		{[]string{"Leaf", "overSibling"}, false, true},
		{[]string{"Mid", "crossesBoundary"}, true, false},
	}
	for _, tt := range cases {
		sym := memberPath(t, pkg, tt.path...)
		satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("%s with no object: %v", sym.Name, err)
		}
		if satisfied != tt.declared {
			t.Errorf("%s with no object: satisfied = %t, want the declared value's verdict %t", sym.Name, satisfied, tt.declared)
		}
	}

	top := memberPath(t, pkg, "top")
	if _, err := ctx.Instantiate(top); err != nil {
		t.Fatalf("instantiate top: %v", err)
	}
	for _, tt := range cases {
		sym := memberPath(t, pkg, tt.path...)
		satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("%s on the object: %v", sym.Name, err)
		}
		if satisfied != tt.about {
			t.Errorf("%s on the object: satisfied = %t, want the object's verdict %t", sym.Name, satisfied, tt.about)
		}
	}
}

// A check handed an object that does not carry the condition itself descends
// into it: the condition of a nested definition is about the nested object.
func TestNestedSubjectUnderSuppliedObject(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, nestedSubjectSrc)
	top := memberPath(t, pkg, "top")
	inst, err := ctx.Instantiate(top)
	if err != nil {
		t.Fatalf("instantiate top: %v", err)
	}
	small := memberPath(t, pkg, "Leaf", "small")
	satisfied, err := ctx.EvaluateConstraintOn(small, small.OwnerScope, inst)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("small on top: %v", err)
	}
	if satisfied {
		t.Error("small on top: satisfied = true, want the nested object's redefined value to violate it")
	}
}

// Two objects redefining the same nested feature differently make the subject a
// question, which is reported rather than answered from either of them.
func TestNestedSubjectAmbiguous(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, nestedSubjectSrc)
	for _, name := range []string{"top", "quick"} {
		if _, err := ctx.Instantiate(memberPath(t, pkg, name)); err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
	}
	small := memberPath(t, pkg, "Leaf", "small")
	if _, err := ctx.EvaluateConstraint(small, small.OwnerScope); !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("small with two carriers: err = %v, want ErrAmbiguousSubject", err)
	}
}
