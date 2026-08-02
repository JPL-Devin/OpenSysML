package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func resolveDoc(t *testing.T, name, src string) *Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := New(idx)
	r.ResolveDocument(name, root)
	return r
}

func TestResolveDocumentReportsUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { alias A for P::Missing; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for P::Missing")
	}
}

func TestResolveDocumentCleanWhenAllResolve(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { namespace N; alias A for P::N; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveDocumentResolvesExpressionRefs(t *testing.T) {
	// FilterMember condition referencing an undefined name -> diagnostic.
	r := resolveDoc(t, "d.sysml",
		"package P { filter Undefined; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for expression ref Undefined")
	}
}

func TestResolve_FeatureChainExpr(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "simple chain - two levels",
			src: `
				package A {
					attribute x = 1;
				}
				package B {
					attribute ref : A;
					attribute test = ref.x;
				}
			`,
			wantErr: false,
		},
		{
			name: "nested chain - three levels",
			src: `
				package A {
					package Inner {
						attribute value = 42;
					}
				}
				package B {
					attribute ref : A;
					attribute test = ref.Inner.value;
				}
			`,
			wantErr: false,
		},
		{
			name: "unresolved first part",
			src: `
				package B {
					attribute test = missing.x;
				}
			`,
			wantErr: true,
		},
		{
			name: "unresolved second part",
			src: `
				package A {
					attribute x = 1;
				}
				package B {
					attribute ref : A;
					attribute test = ref.missing;
				}
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := resolveDoc(t, "test.sysml", tt.src)
			hasErr := len(r.Diagnostics) > 0
			if hasErr != tt.wantErr {
				t.Errorf("wantErr=%v, got diagnostics: %v", tt.wantErr, r.Diagnostics)
			}
		})
	}
}

func TestResolve_QualifiedNamePartsStoreSymbols(t *testing.T) {
	p := parser.New(source.New("test.sysml", []byte(`
		package A {
			package B {
				attribute x = 1;
			}
		}
		package C {
			attribute test : A::B;
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	r := New(idx)
	r.ResolveDocument("test.sysml", root)
	
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolve diagnostics: %v", r.Diagnostics)
	}
	
	// Find C::test usage and verify its typing relationship's QN parts have symbols
	cScope := idx.DocumentRoot("test.sysml")
	cSym, ok := cScope.LookupLocal("C")
	if !ok {
		t.Fatal("package C not found")
	}
	testSym, ok := cSym.Scope.LookupLocal("test")
	if !ok {
		t.Fatal("C::test not found")
	}
	
	usage := testSym.Decl.(*ast.Usage)
	if len(usage.Relationships) == 0 {
		t.Fatal("test has no relationships")
	}
	
	rel := usage.Relationships[0]
	if rel.Kind != ast.RelTyping {
		t.Fatalf("expected typing relationship, got %v", rel.Kind)
	}
	
	var qn *ast.QualifiedName
	switch target := rel.Target.(type) {
	case *ast.FeatureReference:
		qn = target.Name
	case *ast.QualifiedName:
		qn = target
	default:
		t.Fatalf("unexpected target type: %T", rel.Target)
	}
	
	if len(qn.Parts) != 2 {
		t.Fatalf("expected 2 parts in A::B, got %d", len(qn.Parts))
	}
	
	// Verify each part has resolved symbol
	if qn.Parts[0].Sym == nil {
		t.Error("Part 0 (A) symbol not set")
	}
	if qn.Parts[1].Sym == nil {
		t.Error("Part 1 (B) symbol not set")
	}
	
	// Verify symbols are correct
	aSym := qn.Parts[0].Sym.(*symbols.Symbol)
	if aSym.Name != "A" {
		t.Errorf("Part 0 symbol name = %q, want A", aSym.Name)
	}
	
	bSym := qn.Parts[1].Sym.(*symbols.Symbol)
	if bSym.Name != "B" {
		t.Errorf("Part 1 symbol name = %q, want B", bSym.Name)
	}
}
