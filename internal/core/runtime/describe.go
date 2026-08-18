package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// notOfKind reports that sym declares something other than the kind asked of
// it: a usage error naming what it is, never a Go type name.
func notOfKind(sentinel error, sym *symbols.Symbol, kind string) error {
	return fmt.Errorf("%w: %s is %s, not a %s definition or usage",
		sentinel, sym.Name, describeDecl(sym.Decl), kind)
}

// describeOperand names an operand's type for an operator diagnostic, with its
// article, so a message reads "an Integer and a string".
func describeOperand(val Value) string {
	switch val.Kind {
	case ValConst:
		return describeValue(val)
	case ValString:
		return "a string"
	case ValNull:
		return "null"
	case ValInstance:
		return "an instance"
	case ValSequence:
		return "a sequence"
	case ValSet:
		return "a set"
	case ValExpr:
		return "an expression"
	case ValQuantity:
		return "a quantity"
	case ValVariant:
		return "a variant"
	case ValEnumLiteral:
		return "the enumeration literal " + val.LiteralText()
	}
	return "a value"
}

// describeDecl names a declaration the way the notation does, e.g. "a part def"
// or "a calc usage", so a diagnostic about a symbol of the wrong kind never
// prints a Go type name.
func describeDecl(decl ast.Node) string {
	switch d := decl.(type) {
	case *ast.Definition:
		return "a " + d.Kind.String() + " def"
	case *ast.Usage:
		return "a " + d.Kind.String() + " usage"
	case nil:
		return "nothing"
	}
	return "a declaration"
}
