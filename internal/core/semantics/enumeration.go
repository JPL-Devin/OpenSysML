package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// EnumerationOwning returns the enumeration definition sym is a literal of, or
// nil when sym is no enumeration literal: a literal is an enumeration usage
// declared in an enumeration definition's body (SysML v2 §7.6.4).
func EnumerationOwning(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageEnumeration {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || owner.Kind != symbols.SymbolEnumerationDef {
		return nil
	}
	return owner
}

// LiteralValue returns the value expression a literal declares, or nil when it
// declares none and is identified by itself: a literal of an enumeration
// specializing a scalar type — `enum def GradePoints :> Real { A = 4.0; }` — is
// a value of that type.
func LiteralValue(sym *symbols.Symbol) ast.Node {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	return usage.Value
}

// LiteralsOf returns the literals an enumeration definition declares, including
// the ones it inherits from an enumeration it specializes.
func (m *Model) LiteralsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Kind != symbols.SymbolEnumerationDef {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range m.MembersOf(sym) {
		if EnumerationOwning(member) != nil {
			out = append(out, member)
		}
	}
	return out
}
