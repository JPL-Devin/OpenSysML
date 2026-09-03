package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// `$::pick(v)` names the root: every top-level declaration of the name, in the
// calling document and the others, is a candidate, not only the first indexed.
func TestInvocationCandidatesAtTheGlobalRoot(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "calc def pick { in x : ScalarValues::Integer; } package App { attribute v = 1; }",
		"b.sysml": "calc def pick { in x : ScalarValues::String; }",
		"c.sysml": "package Other { calc def pick { in x : ScalarValues::Boolean; } }",
	})
	r := New(idx)
	app := scopeOf(t, idx.DocumentRoot("a.sysml"), "App")

	cands := r.InvocationCandidates(app, qn(true, "pick"))
	if len(cands) != 2 {
		t.Fatalf("$::pick has %d candidates, want the two root-level calcs: %v", len(cands), fqnsOf(idx, cands))
	}
	for _, c := range cands {
		if c.Kind != symbols.SymbolCalcDef || idx.GetFQN(c) != "pick" {
			t.Errorf("candidate %s (%v) is not a root-level pick", idx.GetFQN(c), c.Kind)
		}
	}
	if sym, ok := r.ResolveQualified(app, qn(true, "pick")); !ok || sym != cands[0] {
		t.Errorf("the name alone resolves to %v, want the first candidate", sym)
	}
	if first := r.InvocationCandidates(idx.DocumentRoot("b.sysml"), qn(true, "pick")); len(first) != 2 {
		t.Errorf("from b.sysml $::pick has %d candidates, want 2", len(first))
	}
}

func fqnsOf(idx *symbols.Index, syms []*symbols.Symbol) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = idx.GetFQN(s)
	}
	return out
}
