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
		if len(v.Params) > 0 {
			b.WriteString(` params=[`)
			for i, p := range v.Params {
				if i > 0 {
					b.WriteString(`,`)
				}
				b.WriteString(p.Name)
				if p.Type != nil {
					b.WriteString(`:`)
					b.WriteString(qnString(p.Type))
				}
			}
			b.WriteString(`]`)
		}
		var kids []Node
		if v.Result != nil {
			kids = append(kids, v.Result)
		}
		writeChildren(b, depth, kids)
		return
	case *ErrorNode:
		fmt.Fprintf(b, `(ErrorNode message=%q)`, v.Message)
	case *RootNamespace:
		b.WriteString(`(RootNamespace`)
		writeChildren(b, depth, v.Members)
		return
	case *Membership:
		fmt.Fprintf(b, `(Membership visibility=%q`, visibilityString(v.Visibility))
		writeChildren(b, depth, []Node{v.Member})
		return
	case *Package:
		fmt.Fprintf(b, `(Package name=%q library=%t standard=%t`, identName(v.Ident), v.IsLibrary, v.IsStandard)
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Members))
		return
	case *Namespace:
		fmt.Fprintf(b, `(Namespace name=%q`, identName(v.Ident))
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Members))
		return
	case *Import:
		fmt.Fprintf(b, `(Import visibility=%q all=%t kind=%s recursive=%t imported=%q`,
			visibilityString(v.Visibility), v.IsAll, importKindString(v.Kind), v.IsRecursive, qnString(v.Imported))
		writeChildren(b, depth, v.Body)
		return
	case *Alias:
		fmt.Fprintf(b, `(Alias visibility=%q name=%q for=%q`,
			visibilityString(v.Visibility), identName(v.Ident), qnString(v.For))
		writeChildren(b, depth, v.Body)
		return
	case *Dependency:
		fmt.Fprintf(b, `(Dependency clients=%q suppliers=%q`, qnList(v.Clients), qnList(v.Suppliers))
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Body))
		return
	case *Comment:
		fmt.Fprintf(b, `(Comment about=%q locale=%q)`, qnList(v.About), v.Locale)
	case *Documentation:
		fmt.Fprintf(b, `(Documentation locale=%q)`, v.Locale)
	case *TextualRepresentation:
		fmt.Fprintf(b, `(TextualRepresentation language=%q)`, v.Language)
	case *PrefixMetadata:
		fmt.Fprintf(b, `(PrefixMetadata type=%q)`, qnString(v.Type))
	case *FilterMember:
		b.WriteString(`(FilterMember`)
		writeChildren(b, depth, []Node{v.Condition})
		return
	case *Definition:
		fmt.Fprintf(b, `(Definition kind=%q abstract=%t variation=%t name=%q`,
			v.Kind.String(), v.IsAbstract, v.IsVariation, identName(v.Ident))
		writeChildren(b, depth, defusageChildren(v.Prefixes, v.Relationships, nil, nil, v.Members))
		return
	case *Usage:
		fmt.Fprintf(b, `(Usage kind=%q name=%q ref=%t direction=%q composite=%t derived=%t ordered=%t nonunique=%t`,
			v.Kind.String(), identName(v.Ident), v.IsReference, v.Direction.String(),
			v.IsComposite, v.IsDerived, v.IsOrdered, v.IsNonunique)
		if v.IsConjugated {
			b.WriteString(` conjugated=true`)
		}
		writeChildren(b, depth, usageChildren(v))
		return
	case *FlowEnds:
		fmt.Fprintf(b, `(FlowEnds from=%q to=%q payload=%q)`, qnString(v.From), qnString(v.To), qnString(v.Payload))
	case *ConnectorEnd:
		fmt.Fprintf(b, `(ConnectorEnd target=%q`, qnString(v.Target))
		var kids []Node
		if v.Multiplicity != nil {
			kids = append(kids, v.Multiplicity)
		}
		writeChildren(b, depth, kids)
		return
	case *Relationship:
		fmt.Fprintf(b, `(Relationship kind=%q target=%q)`, v.Kind.String(), qnString(v.Target))
	case *Multiplicity:
		fmt.Fprintf(b, `(Multiplicity range=%t`, v.IsRange)
		var kids []Node
		if v.Lower != nil {
			kids = append(kids, v.Lower)
		}
		if v.Upper != nil {
			kids = append(kids, v.Upper)
		}
		writeChildren(b, depth, kids)
		return
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

func visibilityString(v Visibility) string {
	switch v {
	case VisibilityPublic:
		return "public"
	case VisibilityPrivate:
		return "private"
	case VisibilityProtected:
		return "protected"
	default:
		return "default"
	}
}

func importKindString(k ImportKind) string {
	if k == ImportNamespace {
		return "namespace"
	}
	return "membership"
}

func identName(id Identification) string {
	if id.ShortName != "" && id.Name != "" {
		return "<" + id.ShortName + "> " + id.Name
	}
	if id.ShortName != "" {
		return "<" + id.ShortName + ">"
	}
	return id.Name
}

func qnList(qns []*QualifiedName) string {
	parts := make([]string, len(qns))
	for i, qn := range qns {
		parts[i] = qnString(qn)
	}
	return strings.Join(parts, ", ")
}

// prefixesAnd renders prefix-metadata nodes (as children) followed by members.
func prefixesAnd(prefixes []*PrefixMetadata, members []Node) []Node {
	kids := make([]Node, 0, len(prefixes)+len(members))
	for _, pm := range prefixes {
		kids = append(kids, pm)
	}
	kids = append(kids, members...)
	return kids
}

// defusageChildren flattens the ordered child set for a Definition/Usage
// dump: prefixes, relationships, optional multiplicity, optional value,
// then members. nil multiplicity/value are omitted.
func defusageChildren(prefixes []*PrefixMetadata, rels []*Relationship, mult *Multiplicity, value Node, members []Node) []Node {
	kids := make([]Node, 0, len(prefixes)+len(rels)+2+len(members))
	for _, pm := range prefixes {
		kids = append(kids, pm)
	}
	for _, r := range rels {
		kids = append(kids, r)
	}
	if mult != nil {
		kids = append(kids, mult)
	}
	if value != nil {
		kids = append(kids, value)
	}
	kids = append(kids, members...)
	return kids
}

// usageChildren is defusageChildren for a Usage, additionally emitting the
// optional ConnectorEnd and FlowEnds nodes (after value, before members).
func usageChildren(v *Usage) []Node {
	kids := make([]Node, 0)
	for _, pm := range v.Prefixes {
		kids = append(kids, pm)
	}
	for _, r := range v.Relationships {
		kids = append(kids, r)
	}
	if v.Multiplicity != nil {
		kids = append(kids, v.Multiplicity)
	}
	if v.Value != nil {
		kids = append(kids, v.Value)
	}
	for _, ce := range v.ConnectorEnds {
		kids = append(kids, ce)
	}
	if v.FlowEnds != nil {
		kids = append(kids, v.FlowEnds)
	}
	kids = append(kids, v.Members...)
	return kids
}
