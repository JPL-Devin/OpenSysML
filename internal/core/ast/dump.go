package ast

import (
	"fmt"
	"strings"
)

// Dump renders a node tree as an indented S-expression for golden tests.
func Dump(n Node) string {
	var b strings.Builder
	dumpNode(&b, n, 0)
	return b.String()
}

func indent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

// qnString renders a QualifiedName as `A::B::C` (with `$::` prefix if global).
func qnString(qn *QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, len(qn.Parts))
	for i, p := range qn.Parts {
		parts[i] = p.Text
	}
	s := strings.Join(parts, "::")
	if qn.Global {
		return "$::" + s
	}
	return s
}

// dumpNode writes `(Type attrs children...)`. It writes the open line with
// header and any leaf attributes, then children each on their own line at
// depth+1, closing all open parens on the final line.
func dumpNode(b *strings.Builder, n Node, depth int) {
	indent(b, depth)
	switch v := n.(type) {
	case *LiteralInteger:
		fmt.Fprintf(b, `(LiteralInteger value=%q)`, v.Value)
	case *LiteralReal:
		fmt.Fprintf(b, `(LiteralReal value=%q)`, v.Value)
	case *LiteralString:
		fmt.Fprintf(b, `(LiteralString value=%q)`, v.Value)
	case *LiteralBool:
		fmt.Fprintf(b, `(LiteralBool value=%t)`, v.Value)
	case *LiteralInfinity:
		b.WriteString(`(LiteralInfinity)`)
	case *NullExpr:
		b.WriteString(`(NullExpr)`)
	case *FeatureReference:
		fmt.Fprintf(b, `(FeatureReference name=%q)`, qnString(v.Name))
	case *MetadataAccessExpr:
		fmt.Fprintf(b, `(MetadataAccessExpr ref=%q)`, qnString(v.Ref))
	case *OperatorExpr:
		fmt.Fprintf(b, `(OperatorExpr operator=%q`, v.Operator.String())
		writeChildren(b, depth, operandsWithTypeRef(v))
		return
	case *FeatureChainExpr:
		fmt.Fprintf(b, `(FeatureChainExpr member=%q`, qnString(v.Member))
		writeChildren(b, depth, []Node{v.Operand})
		return
	case *IndexExpr:
		b.WriteString(`(IndexExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Index})
		return
	case *CollectExpr:
		b.WriteString(`(CollectExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Body})
		return
	case *SelectExpr:
		b.WriteString(`(SelectExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Body})
		return
	case *InvocationExpr:
		fmt.Fprintf(b, `(InvocationExpr type=%q`, qnString(v.Type))
		writeChildren(b, depth, invocationChildren(v))
		return
	case *ConstructorExpr:
		fmt.Fprintf(b, `(ConstructorExpr type=%q`, qnString(v.Type))
		writeChildren(b, depth, v.Args)
		return
	case *SequenceExpr:
		b.WriteString(`(SequenceExpr`)
		writeChildren(b, depth, v.Elements)
		return
	case *BodyExpr:
		b.WriteString(`(BodyExpr`)
		var kids []Node
		if v.Result != nil {
			kids = append(kids, v.Result)
		}
		writeChildren(b, depth, kids)
		return
	case *ErrorNode:
		fmt.Fprintf(b, `(ErrorNode message=%q)`, v.Message)
	default:
		fmt.Fprintf(b, `(%T)`, n)
	}
}

// writeChildren appends children lines under a header that was written
// WITHOUT its closing paren; it closes with `)` after the last child (or
// immediately if there are none).
func writeChildren(b *strings.Builder, depth int, kids []Node) {
	if len(kids) == 0 {
		b.WriteString(")")
		return
	}
	for _, k := range kids {
		b.WriteString("\n")
		dumpNode(b, k, depth+1)
	}
	b.WriteString(")")
}

func operandsWithTypeRef(v *OperatorExpr) []Node {
	kids := append([]Node{}, v.Operands...)
	if v.TypeRef != nil {
		kids = append(kids, &FeatureReference{Name: v.TypeRef})
	}
	return kids
}

func invocationChildren(v *InvocationExpr) []Node {
	kids := []Node{}
	if v.Operand != nil {
		kids = append(kids, v.Operand)
	}
	kids = append(kids, v.Args...)
	return kids
}
