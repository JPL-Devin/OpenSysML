package ast

import "github.com/Open-MBEE/OpenSysML/internal/core/source"

// Visibility mirrors SysML VisibilityKind.
type Visibility int

const (
	VisibilityDefault Visibility = iota // no explicit indicator
	VisibilityPublic
	VisibilityPrivate
	VisibilityProtected
)

// NameSegment is one identifier in a qualified name, with its source span.
// It carries no semantic information: the symbol a segment resolves to lives in
// the resolver's side table, reachable through Resolver.PartSymbol.
type NameSegment struct {
	Text string
	Span source.Span
	// Chained records that the segment was written after '.' rather than after
	// '::': a feature chain step (`S2.S3`) rather than a namespace member.
	Chained bool
}

// QualifiedName is an unresolved dotted/`::`-separated name reference.
// Global is true when the name began with `$::`.
type QualifiedName struct {
	NodeBase
	Global bool
	Parts  []NameSegment
}

// AsQualifiedName unwraps the two forms a name reference parses to: a bare
// QualifiedName, or one wrapped in a FeatureReference. It returns nil for
// anything else.
func AsQualifiedName(node Node) *QualifiedName {
	switch n := node.(type) {
	case *QualifiedName:
		return n
	case *FeatureReference:
		return n.Name
	}
	return nil
}

// SimpleName returns a reference's last segment — the name the resolver and the
// runtime match on — or "" when node names nothing.
func SimpleName(node Node) string {
	qname := AsQualifiedName(node)
	if qname == nil || len(qname.Parts) == 0 {
		return ""
	}
	return qname.Parts[len(qname.Parts)-1].Text
}

// TargetName returns the last segment of a relationship target — a qualified
// name, a feature reference, or a feature chain (`providePower.generateTorque`)
// — together with its span, or "" when node names nothing.
func TargetName(node Node) (string, source.Span) {
	if chain, ok := node.(*FeatureChainExpr); ok {
		if chain.Member == nil {
			return "", source.Span{}
		}
		node = chain.Member
	}
	qname := AsQualifiedName(node)
	if qname == nil || len(qname.Parts) == 0 {
		return "", source.Span{}
	}
	last := qname.Parts[len(qname.Parts)-1]
	return last.Text, last.Span
}

// NamingFeature returns the relationship that names a usage lacking a declared
// name (KerML 7.3.4.5): its reference subsetting, else its lone redefinition.
// A usage that declares a name, or redefines more than one feature, has none.
// A declared short name is no name here: KerML derives effectiveName from
// declaredName alone.
func NamingFeature(u *Usage) *Relationship {
	if u == nil || u.Ident.Name != "" {
		return nil
	}
	var redefinitions []*Relationship
	for _, rel := range u.Relationships {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case RelReferences:
			// A binding's reference subsetting is the end it binds, not a name
			// it answers to: `bind a.b.c = d` declares no member `c`.
			if u.Kind == UsageBinding {
				continue
			}
			if name, _ := TargetName(rel.Target); name != "" {
				return rel
			}
		case RelRedefines:
			redefinitions = append(redefinitions, rel)
		}
	}
	if len(redefinitions) == 1 {
		return redefinitions[0]
	}
	return nil
}

// EffectiveName returns the name a usage answers to: its declared name, else
// the name its naming feature supplies.
func EffectiveName(u *Usage) (string, source.Span) {
	if u == nil {
		return "", source.Span{}
	}
	if u.Ident.Name != "" {
		return u.Ident.Name, u.Ident.NameSpan
	}
	if rel := NamingFeature(u); rel != nil {
		return TargetName(rel.Target)
	}
	return "", source.Span{}
}

// Identification captures `<shortName> name` or `name` on a declaration.
type Identification struct {
	ShortName     string
	ShortNameSpan source.Span
	Name          string
	NameSpan      source.Span
}

// Membership wraps a namespace member with a visibility prefix. Member is
// the owned element (a Package/Namespace/Dependency/Comment/... or ErrorNode).
// A `then` prefixing a member is not recorded here: it sequences the members
// either side of it rather than describing one of them, so the parser desugars
// it to a SuccessionEdge of its own (see internal/core/parser/succession.go).
type Membership struct {
	NodeBase
	Visibility Visibility
	Member     Node
}

// RootNamespace is the top of every parsed file: a flat list of members.
type RootNamespace struct {
	NodeBase
	Members []Node // *Membership | *Import | *Alias | *ErrorNode
}

// PrefixMetadata records a `# QualifiedName` metadata annotation reference.
type PrefixMetadata struct {
	NodeBase
	Type *QualifiedName
	Body []Node // optional body with property initializers: @Meta{prop = value;}
}

// Namespace is `namespace <id> { ... }`.
type Namespace struct {
	NodeBase
	Prefixes []*PrefixMetadata
	Ident    Identification
	Members  []Node
	HasBody  bool // false when body was `;`
}

// Package is `package <id> { ... }`. Library/Standard flags cover
// `library package` and `standard library package`.
type Package struct {
	NodeBase
	Prefixes   []*PrefixMetadata
	Ident      Identification
	IsLibrary  bool
	IsStandard bool
	Members    []Node
	HasBody    bool
}

// ImportKind distinguishes membership vs namespace imports.
type ImportKind int

const (
	ImportMembership ImportKind = iota // import A::B ;
	ImportNamespace                    // import A::B::* ;
)

// Import is `[visibility] import [all] QualifiedName[::*][::**] ;|{}`.
//
// An `expose` declaration in a view body is also an Import: SysML v2 8.3.26.2
// makes Expose a specialization of Import (MembershipExpose specializes
// MembershipImport, NamespaceExpose specializes NamespaceImport), so it is
// represented by this node with IsExpose set.
type Import struct {
	NodeBase
	Visibility  Visibility
	IsAll       bool
	Kind        ImportKind
	Imported    *QualifiedName
	IsRecursive bool // `::**`
	FilterExpr  Node // Optional filter expression [<expr>]
	Body        []Node
	HasBody     bool
	// IsExpose marks an `expose` declaration. Per SysML v2 8.3.26.2 an Expose
	// always imports all elements regardless of visibility (isImportAll = true)
	// and always has protected visibility, so IsAll and Visibility are fixed
	// accordingly by the parser.
	IsExpose bool
}

// Alias is `alias <shortName> name for QualifiedName ;|{}`.
type Alias struct {
	NodeBase
	Visibility Visibility
	Ident      Identification
	For        *QualifiedName
	Body       []Node
	HasBody    bool
}

// MultiplicityDecl is `multiplicity <id> [range] ;|{}`.
// Declares a named multiplicity range like exactlyOne [1..1].
type MultiplicityDecl struct {
	NodeBase
	Ident   Identification
	Range   *Multiplicity // optional - range bounds
	Members []Node        // optional - body members (typically doc comments)
	HasBody bool          // true if has {}, false if just ;
}

// Dependency is `dependency [<id> from] clients to suppliers ;|{}`.
type Dependency struct {
	NodeBase
	Prefixes  []*PrefixMetadata
	Ident     Identification
	Clients   []*QualifiedName
	Suppliers []*QualifiedName
	Body      []Node
	HasBody   bool
}

// Comment is `[comment <id> [about refs]] [locale s] /* ... */`.
type Comment struct {
	NodeBase
	Ident    Identification
	About    []*QualifiedName
	Locale   string
	BodySpan source.Span // the REGULAR_COMMENT token span
}

// Documentation is `doc <id> [locale s] /* ... */`.
type Documentation struct {
	NodeBase
	Ident    Identification
	Locale   string
	BodySpan source.Span
}

// TextualRepresentation is `[rep <id>] language s /* ... */`.
type TextualRepresentation struct {
	NodeBase
	Ident    Identification
	Language string
	BodySpan source.Span
}

// FilterMember is an `filter <expression> ;` element filter.
type FilterMember struct {
	NodeBase
	Condition Node
}
