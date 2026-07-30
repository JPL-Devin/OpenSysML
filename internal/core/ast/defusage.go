package ast

// DefinitionKind discriminates the concrete definition taxonomy element.
type DefinitionKind int

const (
	DefPart DefinitionKind = iota
	DefAttribute
	// extensible: DefItem, DefPort, DefAction, ...
)

func (k DefinitionKind) String() string {
	switch k {
	case DefPart:
		return "part"
	case DefAttribute:
		return "attribute"
	default:
		return "unknown"
	}
}

// UsageKind discriminates the concrete usage taxonomy element.
type UsageKind int

const (
	UsagePart UsageKind = iota
	UsageAttribute
)

func (k UsageKind) String() string {
	switch k {
	case UsagePart:
		return "part"
	case UsageAttribute:
		return "attribute"
	default:
		return "unknown"
	}
}

// RelationshipKind discriminates a specialization/typing edge at a
// definition or usage declaration head.
type RelationshipKind int

const (
	RelTyping      RelationshipKind = iota // ':' / 'defined by'
	RelSpecializes                         // 'specializes' / ':>'
	RelSubsets                             // 'subsets' / ':>'
	RelRedefines                           // 'redefines' / ':>>'
	RelReferences                          // 'references' / '::>'
	RelCrosses                             // 'crosses' / '=>'
)

func (k RelationshipKind) String() string {
	switch k {
	case RelTyping:
		return "typing"
	case RelSpecializes:
		return "specializes"
	case RelSubsets:
		return "subsets"
	case RelRedefines:
		return "redefines"
	case RelReferences:
		return "references"
	case RelCrosses:
		return "crosses"
	default:
		return "unknown"
	}
}

// FeatureDirection is the in/out/inout direction modifier on a usage.
type FeatureDirection int

const (
	DirNone FeatureDirection = iota
	DirIn
	DirOut
	DirInOut
)

func (d FeatureDirection) String() string {
	switch d {
	case DirIn:
		return "in"
	case DirOut:
		return "out"
	case DirInOut:
		return "inout"
	default:
		return "none"
	}
}

// Relationship is one specialization/typing edge: exactly one Target. A
// clause with multiple targets (`specializes A, B`) produces multiple
// Relationship entries sharing the same Kind.
type Relationship struct {
	NodeBase
	Kind   RelationshipKind
	Target *QualifiedName
}

// Multiplicity is a `[n]` / `[lo..hi]` / `[*]` bound on a usage. Bounds are
// expression Nodes (reusing the expression AST); `*` becomes LiteralInfinity.
// For the single-bound form `[n]`, Upper holds n and IsRange is false.
type Multiplicity struct {
	NodeBase
	Lower   Node
	Upper   Node
	IsRange bool
}

// Definition is a `part def` / `attribute def` (and future kinds) node.
type Definition struct {
	NodeBase
	Prefixes      []*PrefixMetadata
	Kind          DefinitionKind
	IsAbstract    bool
	IsVariation   bool
	Ident         Identification
	Relationships []*Relationship
	Members       []Node
	HasBody       bool
}

// Usage is a `part` / `attribute` usage node.
type Usage struct {
	NodeBase
	Prefixes      []*PrefixMetadata
	Kind          UsageKind
	IsAbstract    bool
	IsReference   bool
	Direction     FeatureDirection
	IsComposite   bool
	IsDerived     bool
	IsOrdered     bool
	IsNonunique   bool
	Ident         Identification
	Relationships []*Relationship
	Multiplicity  *Multiplicity
	Value         Node
	Members       []Node
	HasBody       bool
}
