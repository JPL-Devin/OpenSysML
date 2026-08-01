package ast

// DefinitionKind discriminates the concrete definition taxonomy element.
type DefinitionKind int

const (
	DefPart DefinitionKind = iota
	DefAttribute
	// Tier A — pure keyword-swaps of the part/attribute pattern.
	DefItem
	DefOccurrence
	DefIndividual
	DefMetaclass
	DefMetadata
	DefEnumeration
	DefView
	DefViewpoint
	DefRendering
	DefConcern
	// Tier B — distinctive declaration grammar.
	DefConnection
	DefFlow
	DefPort
	DefInterface
	DefAllocation
	DefBinding
	// Tier C — nested behavioral bodies (generic body this cycle).
	DefAction
	DefState
	DefCalc
	DefConstraint
	DefRequirement
	DefCase
	DefAnalysisCase
	DefVerificationCase
	DefUseCase
	// KerML structural kinds
	DefBehavior
	DefAssoc
	DefStruct
)

func (k DefinitionKind) String() string {
	switch k {
	case DefPart:
		return "part"
	case DefAttribute:
		return "attribute"
	case DefItem:
		return "item"
	case DefOccurrence:
		return "occurrence"
	case DefIndividual:
		return "individual"
	case DefMetaclass:
		return "metaclass"
	case DefMetadata:
		return "metadata"
	case DefEnumeration:
		return "enum"
	case DefView:
		return "view"
	case DefViewpoint:
		return "viewpoint"
	case DefRendering:
		return "rendering"
	case DefConcern:
		return "concern"
	case DefConnection:
		return "connection"
	case DefFlow:
		return "flow"
	case DefPort:
		return "port"
	case DefInterface:
		return "interface"
	case DefAllocation:
		return "allocation"
	case DefBinding:
		return "binding"
	case DefAction:
		return "action"
	case DefState:
		return "state"
	case DefCalc:
		return "calc"
	case DefConstraint:
		return "constraint"
	case DefRequirement:
		return "requirement"
	case DefCase:
		return "case"
	case DefAnalysisCase:
		return "analysis case"
	case DefVerificationCase:
		return "verification case"
	case DefUseCase:
		return "use case"
	case DefBehavior:
		return "behavior"
	case DefAssoc:
		return "assoc"
	case DefStruct:
		return "struct"
	default:
		return "unknown"
	}
}

// UsageKind discriminates the concrete usage taxonomy element.
type UsageKind int

const (
	UsagePart UsageKind = iota
	UsageAttribute
	// Tier A.
	UsageItem
	UsageOccurrence
	UsageIndividual
	UsageMetadata
	UsageEnumeration
	UsageView
	UsageViewpoint
	UsageRendering
	UsageConcern
	// Tier B.
	UsageConnection
	UsageFlow
	UsagePort
	UsageInterface
	UsageAllocation
	UsageBinding
	// Tier C.
	UsageAction
	UsageState
	UsageCalc
	UsageConstraint
	UsageRequirement
	UsageCase
	UsageAnalysisCase
	UsageVerificationCase
	UsageUseCase
	// KerML structural kinds
	UsageBehavior
	UsageAssoc
	UsageStruct
)

func (k UsageKind) String() string {
	switch k {
	case UsagePart:
		return "part"
	case UsageAttribute:
		return "attribute"
	case UsageItem:
		return "item"
	case UsageOccurrence:
		return "occurrence"
	case UsageIndividual:
		return "individual"
	case UsageMetadata:
		return "metadata"
	case UsageEnumeration:
		return "enum"
	case UsageView:
		return "view"
	case UsageViewpoint:
		return "viewpoint"
	case UsageRendering:
		return "rendering"
	case UsageConcern:
		return "concern"
	case UsageConnection:
		return "connection"
	case UsageFlow:
		return "flow"
	case UsagePort:
		return "port"
	case UsageInterface:
		return "interface"
	case UsageAllocation:
		return "allocation"
	case UsageBinding:
		return "binding"
	case UsageAction:
		return "action"
	case UsageState:
		return "state"
	case UsageCalc:
		return "calc"
	case UsageConstraint:
		return "constraint"
	case UsageRequirement:
		return "requirement"
	case UsageCase:
		return "case"
	case UsageAnalysisCase:
		return "analysis case"
	case UsageVerificationCase:
		return "verification case"
	case UsageUseCase:
		return "use case"
	case UsageBehavior:
		return "behavior"
	case UsageAssoc:
		return "assoc"
	case UsageStruct:
		return "struct"
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
	IsAll         bool // 'all' multiplicity propagation modifier
	Ident         Identification
	Relationships []*Relationship
	Members       []Node
	HasBody       bool
}

// Usage is a `part` / `attribute` usage node (and all other usage kinds).
type Usage struct {
	NodeBase
	Prefixes      []*PrefixMetadata
	Kind          UsageKind
	IsAbstract    bool
	IsReference   bool
	IsAll         bool // 'all' multiplicity propagation modifier
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

	// Tier B connection/flow/port grammar. These are nil/zero for kinds
	// that do not use them.
	ConnectorEnds []*QualifiedName // connection / interface / allocation usage ends
	FlowEnds      *FlowEnds        // flow usage ends
	IsConjugated  bool             // `~` conjugation on port / interface
}

// FlowEnds holds the ends of a flow usage: the `from`/`to` targets and an
// optional payload from the `of` clause. It embeds NodeBase (spannable) but is
// only ever reached through the *ast.Usage traversal case.
type FlowEnds struct {
	NodeBase
	From    *QualifiedName
	To      *QualifiedName
	Payload *QualifiedName // optional; from the `of` clause
}
