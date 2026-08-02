package resolve

import (
	"testing"

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
