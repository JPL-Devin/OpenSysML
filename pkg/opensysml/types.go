package opensysml

import (
	"slices"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

// Capability names a client can require of a ServerInfo, re-stated from the
// service so negotiating on them needs no other import. Capabilities are only
// ever added, never renamed or removed with their behaviour intact.
const (
	CapabilityTypeFacts         = sysmlgrpc.CapabilityTypeFacts
	CapabilityConvert           = sysmlgrpc.CapabilityConvert
	CapabilityVerification      = sysmlgrpc.CapabilityVerification
	CapabilityQuery             = sysmlgrpc.CapabilityQuery
	CapabilityOSLCQuery         = sysmlgrpc.CapabilityOSLCQuery
	CapabilityEnumValues        = sysmlgrpc.CapabilityEnumValues
	CapabilityEvaluateSubject   = sysmlgrpc.CapabilityEvaluateSubject
	CapabilitySymbolAttributes  = sysmlgrpc.CapabilitySymbolAttributes
	CapabilityUnsetValue        = sysmlgrpc.CapabilityUnsetValue
	CapabilityFeatureValues     = sysmlgrpc.CapabilityFeatureValues
	CapabilityApplyEdits        = sysmlgrpc.CapabilityApplyEdits
	CapabilityAuthoring         = sysmlgrpc.CapabilityAuthoring
	CapabilityInlineLanguage    = sysmlgrpc.CapabilityInlineLanguage
	CapabilityStrictConformance = sysmlgrpc.CapabilityStrictConformance
)

// ServerInfo describes the implementation answering a Client's calls.
type ServerInfo struct {
	// Version is the build version, informational only: feature decisions
	// belong on Capabilities.
	Version string
	// Capabilities are the named capabilities this implementation supports.
	Capabilities []string
}

// Has reports whether the implementation names the capability. Negotiate on
// capabilities, not on versions: the service does not refuse a call it lacks
// the capability for, so require the name before relying on the behaviour.
func (si *ServerInfo) Has(capability string) bool {
	return si != nil && slices.Contains(si.Capabilities, capability)
}

// Model is a parsed model, the handle the other calls take. It is data the
// caller owns: mutating it changes nothing in the engine or the service.
type Model struct {
	// Hash is the content-addressed cache key the service assigned the parse.
	Hash string
	// Root is the model's root namespace.
	Root *Symbol
	// Diagnostics from parsing and semantic analysis. A syntax error is a
	// diagnostic here, not a failed call.
	Diagnostics []Diagnostic
}

// Symbol is one element of a parsed model.
type Symbol struct {
	// ID is the fully qualified name, the identifier the other calls take.
	ID   string
	Name string
	// Kind of the element ("PartDefinition", "AttributeUsage", …).
	Kind     string
	Metadata map[string]string
	// ChildIDs are the fully qualified names of the element's children.
	ChildIDs []string
	// Attributes of the element, absent without the symbol_attributes
	// capability.
	Attributes []Attribute
	// Type facts, nil when the element is not a def/usage or the type_facts
	// capability is absent.
	Type *TypeInfo
	// Multiplicity as declared, nil when none is declared.
	Multiplicity *Multiplicity
	// Specializations are every generalization edge declared, in declaration
	// order.
	Specializations []Specialization
	// WithheldLibraryAttributes counts attributes inherited from
	// standard-library content, which Attributes leaves out.
	WithheldLibraryAttributes int
}

// Attribute is one attribute of a symbol, with its declared value.
type Attribute struct {
	Name string
	Type string
	// Value is the declared value, nil when none is declared.
	Value Value
	Unit  string
}

// TypeInfo is the static type of a usage, or the classification of a
// definition, as far as the service derives it without running the model.
type TypeInfo struct {
	// Declared is the type name as written; empty when none is declared.
	Declared string
	// ResolvedID is the FQN of the resolved type; empty when unresolved.
	ResolvedID string
	// ResolvedKind is the symbol kind of the resolved type ("partDef", …).
	ResolvedKind string
	// Primitive is the library scalar the type reduces to ("Boolean",
	// "Integer", "Real", …); empty when it is not a scalar value type.
	Primitive string
	// PrimitiveSource is the origin of Primitive: "declared", "value", or
	// empty.
	PrimitiveSource string
	// Quantity reports that values carry a measurement unit.
	Quantity bool
	// Unit as written when the default value names one ("m/s"), else empty.
	Unit string
}

// Multiplicity is a declared multiplicity range. A bound the service cannot
// evaluate statically is empty.
type Multiplicity struct {
	Lower string // "0", "1", … or "*"
	Upper string // "1", "*", …
}

// Specialization is one generalization edge an element declares.
type Specialization struct {
	// Kind of relationship: "specializes", "subsets", "redefines" or "typing".
	Kind string
	// Declared is the target as written ("Engine", "Demo::Engine").
	Declared string
	// TargetID is the FQN of the resolved target; empty when unresolved.
	TargetID string
	// TargetKind is the symbol kind of the resolved target; empty when
	// unresolved.
	TargetKind string
}

// Diagnostic is one parse or semantic finding.
type Diagnostic struct {
	// Severity is "error", "warning" or "info".
	Severity string
	Message  string
	// Span locates the finding in its source, nil when it has no location.
	Span *Span
}

// Span is a source location.
type Span struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// Instance is a runtime instance of a part or usage.
type Instance struct {
	// ID names the instance within its Instantiation; an InstanceID value
	// elsewhere in the answer refers to it.
	ID int64
	// TypeSymbolID is the FQN of the def/usage this is an instance of.
	TypeSymbolID string
	// FeatureValues are what the object holds for each feature of its type,
	// by feature name.
	FeatureValues map[string]FeatureValue
}

// FeatureValue is what an object holds for one feature of its type.
type FeatureValue struct {
	FeatureName string
	// Value for a single-valued feature, nil when Values or Error applies.
	Value Value
	// Values for a multi-valued feature.
	Values []Value
	// Materialized reports whether the value was computed rather than read.
	Materialized bool
	// Error is set when evaluating the feature failed; Value is then nil.
	Error string
}

// Instantiation is the result of instantiating a symbol: the root instance and
// every instance reachable from it, so an InstanceID resolves without another
// call.
type Instantiation struct {
	// Root is the instance of the symbol that was instantiated.
	Root *Instance
	// Instances is every instance reachable from Root, including Root itself,
	// in the order the service reported them.
	Instances []*Instance
	// Diagnostics the answer carried, if any.
	Diagnostics []Diagnostic
}

// Language selects the notation of inline content.
type Language string

// The languages ParseSource accepts.
const (
	LanguageSysML Language = "sysml"
	LanguageKerML Language = "kerml"
)
