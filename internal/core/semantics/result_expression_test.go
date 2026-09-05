package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func ownerNames(m *Model, sym *symbols.Symbol) []string {
	var names []string
	for _, owner := range m.ResultExpressionOwners(sym) {
		names = append(names, owner.Name)
	}
	return names
}

func TestResultExpressionOwnersInheritedOnly(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { x > 0 } constraint def Sub :> Base { }")
	if got := ownerNames(m, sym(t, root, "Sub")); strings.Join(got, ",") != "Base" {
		t.Fatalf("an empty specialization inherits Base's result, got %v", got)
	}
}

func TestResultExpressionOwnersOwnedFirst(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { x > 0 } constraint def Sub :> Base { x > 1 }")
	if got := ownerNames(m, sym(t, root, "Sub")); strings.Join(got, ",") != "Sub,Base" {
		t.Fatalf("the owned result comes first, then the inherited one, got %v", got)
	}
}

func TestResultExpressionOwnersDiamondCountsOnce(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { x > 0 } constraint def L :> Base; constraint def R :> Base;"+
			" constraint def D :> L, R;")
	if got := ownerNames(m, sym(t, root, "D")); strings.Join(got, ",") != "Base" {
		t.Fatalf("Base reached along two paths contributes once, got %v", got)
	}
}

func TestResultExpressionOwnersTwoGenerals(t *testing.T) {
	m, root := buildModel(t,
		"constraint def A { x > 0 } constraint def B { x > 1 } constraint def C :> A, B;")
	if got := ownerNames(m, sym(t, root, "C")); strings.Join(got, ",") != "A,B" {
		t.Fatalf("each general owning a result is an owner, got %v", got)
	}
}

func TestResultExpressionManyConditionsOneOwner(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { in x; x > 0 x < 10 }")
	base := sym(t, root, "Base")
	if got := len(OwnedResultExpressions(base)); got != 2 {
		t.Fatalf("two conditions are two result expressions, got %d", got)
	}
	if got := ownerNames(m, base); strings.Join(got, ",") != "Base" {
		t.Fatalf("both belong to one owner, got %v", got)
	}
}

func TestResultExpressionNestedAssertionIsNotAResult(t *testing.T) {
	m, root := buildModel(t,
		"requirement def Margin { attribute x; require constraint enough { x > 0 } }"+
			" requirement def Tight :> Margin { require constraint :>> enough { assert constraint { x > 1 } } }")
	redefining := sym(t, sym(t, root, "Tight").Scope, "enough")
	if got := ownerNames(m, redefining); len(got) != 1 || got[0] != "enough" {
		t.Fatalf("a nested assertion adds no result; the redefined one is inherited, got %v", got)
	}
}
