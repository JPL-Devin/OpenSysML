package ast

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Visibility mirrors SysML VisibilityKind.
type Visibility int

const (
	VisibilityDefault Visibility = iota // no explicit indicator
	VisibilityPublic
	VisibilityPrivate
	VisibilityProtected
)

// NameSegment is one identifier in a qualified name, with its source span.
// Sym is set by the resolver to the symbol this segment resolves to.
type NameSegment struct {
	Text string
	Span source.Span
	Sym  interface{} // *symbols.Symbol, set by resolver
}

// QualifiedName is an unresolved dotted/`::`-separated name reference.
// Global is true when the name began with `$::`.
type QualifiedName struct {
	NodeBase
	Global bool
	Parts  []NameSegment
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
// Succession stores optional 'then' edge to next member (namespace-level succession).
type Membership struct {
	NodeBase
	Visibility       Visibility
	Member           Node
	HasSuccession    bool // true if 'then' keyword follows this member
	SuccessionTarget string // short name of next member (resolved during semantic analysis)
	SuccessionGuard  Node // optional guard expression on succession edge
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
type Import struct {
	NodeBase
	Visibility  Visibility
	IsAll       bool
	Kind        ImportKind
	Imported    *QualifiedName
	IsRecursive bool // `::**`
	Body        []Node
	HasBody     bool
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
	Ident       Identification
	Range       *Multiplicity // optional - range bounds
	Members     []Node        // optional - body members (typically doc comments)
	HasBody     bool          // true if has {}, false if just ;
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
