package grpc

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Query property names, as the SysML v2 API & Services standard's clients write
// them. docs/API.md documents what each one reports.
const (
	QueryPropID                = "@id"
	QueryPropType              = "@type"
	QueryPropName              = "name"
	QueryPropDeclaredName      = "declaredName"
	QueryPropQualifiedName     = "qualifiedName"
	QueryPropOwner             = "owner"
	QueryPropIsAbstract        = "isAbstract"
	QueryPropElementType       = "type"
	QueryPropMultiplicityLower = "multiplicityLower"
	QueryPropMultiplicityUpper = "multiplicityUpper"
)

// QueryErrorKind classifies why a query could not be evaluated. Every kind fails
// the call: answering with no elements would read as "nothing matched".
type QueryErrorKind int

const (
	// QueryErrUnknownProperty is a property no queryable property names.
	QueryErrUnknownProperty QueryErrorKind = iota + 1
	// QueryErrMalformedConstraint is a constraint with no form, no operator or
	// no operand.
	QueryErrMalformedConstraint
	// QueryErrUnorderedProperty is > or < on a property that is not ordered.
	QueryErrUnorderedProperty
	// QueryErrUnparsableValue is an operand the operator cannot compare.
	QueryErrUnparsableValue
	// QueryErrUnknownScope is a scope naming an element the model does not have.
	QueryErrUnknownScope
)

// QueryError is a query the service refuses to evaluate. It reports itself as
// INVALID_ARGUMENT, since every kind is a fault in the query, not in the model.
type QueryError struct {
	Kind    QueryErrorKind
	Message string
}

func (e *QueryError) Error() string { return e.Message }

// GRPCStatus maps the error onto the status the RPC fails with.
func (e *QueryError) GRPCStatus() *status.Status {
	return status.New(codes.InvalidArgument, e.Message)
}

func queryErrorf(kind QueryErrorKind, format string, args ...any) *QueryError {
	return &QueryError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// queryProperty is one queryable property: how it reads off an element, and
// whether its values are ordered, which is what > and < require.
type queryProperty struct {
	ordered bool
	// read reports the property's value, and whether the element has one at all.
	read func(*queryEval, *symbols.Symbol) (string, bool)
}

// queryProperties is the single source of truth for what a query may name. Each
// entry reads the existing symbol index and semantic model.
var queryProperties = map[string]queryProperty{
	QueryPropID: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		return present(e.sc.Index.GetFQN(sym))
	}},
	QueryPropQualifiedName: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		return present(e.sc.Index.GetFQN(sym))
	}},
	QueryPropType: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		return present(MetamodelTypeName(sym.Kind))
	}},
	QueryPropName: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		return present(localName(e.sc.Index.GetFQN(sym)))
	}},
	QueryPropDeclaredName: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		if sym.EffectiveName {
			return "", false // the name was borrowed from a referenced feature
		}
		return present(localName(e.sc.Index.GetFQN(sym)))
	}},
	QueryPropOwner: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		if sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil {
			return present(e.sc.Index.GetFQN(sym.OwnerScope.Owner()))
		}
		// A library element restored from cache owns no scope chain, so its owner
		// is the namespace its qualified name names; a top-level one has none.
		return present(owningName(e.sc.Index.GetFQN(sym)))
	}},
	QueryPropIsAbstract: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		switch decl := sym.Decl.(type) {
		case *ast.Usage:
			return strconv.FormatBool(decl.IsAbstract), true
		case *ast.Definition:
			return strconv.FormatBool(decl.IsAbstract), true
		default:
			return "", false
		}
	}},
	QueryPropElementType: {read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		info := e.sc.typeInfoOf(sym)
		if info == nil {
			return "", false
		}
		return present(info.ResolvedId)
	}},
	QueryPropMultiplicityLower: {ordered: true, read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		mult := e.sc.multiplicityOf(sym)
		if mult == nil {
			return "", false
		}
		return present(mult.Lower)
	}},
	QueryPropMultiplicityUpper: {ordered: true, read: func(e *queryEval, sym *symbols.Symbol) (string, bool) {
		mult := e.sc.multiplicityOf(sym)
		if mult == nil {
			return "", false
		}
		return present(mult.Upper)
	}},
}

// localName returns the last segment of a qualified name: a library element
// restored from cache is registered under its qualified name.
func localName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+len("::"):]
	}
	return fqn
}

// owningName returns the namespace part of a qualified name, or "" for a
// top-level one.
func owningName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[:i]
	}
	return ""
}

// present reports a property value, treating an empty one as absent: an element
// with no name has no name to compare, rather than one that is the empty string.
func present(value string) (string, bool) {
	return value, value != ""
}

// QueryPropertyNames returns every queryable property name, sorted. It is what
// an unknown-property error lists, and what docs/API.md's table documents.
func QueryPropertyNames() []string {
	out := make([]string, 0, len(queryProperties))
	for name := range queryProperties {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// metamodelTypeNames maps Systemica's symbol kinds onto the metamodel type names
// the standard's clients write as `@type`, and is the single source of truth for
// that mapping. Where the metamodel has no distinct type — an individual is an
// occurrence with isIndividual set — the closest one it has is used; docs/API.md
// reproduces the table and records those choices.
var metamodelTypeNames = map[symbols.SymbolKind]string{
	symbols.SymbolPackage:               "Package",
	symbols.SymbolNamespace:             "Namespace",
	symbols.SymbolAlias:                 "Membership",
	symbols.SymbolDependency:            "Dependency",
	symbols.SymbolComment:               "Comment",
	symbols.SymbolDocumentation:         "Documentation",
	symbols.SymbolTextualRepresentation: "TextualRepresentation",
	symbols.SymbolPartDef:               "PartDefinition",
	symbols.SymbolAttributeDef:          "AttributeDefinition",
	symbols.SymbolPartUsage:             "PartUsage",
	symbols.SymbolAttributeUsage:        "AttributeUsage",
	symbols.SymbolItemDef:               "ItemDefinition",
	symbols.SymbolOccurrenceDef:         "OccurrenceDefinition",
	symbols.SymbolIndividualDef:         "OccurrenceDefinition",
	symbols.SymbolMetadataDef:           "MetadataDefinition",
	symbols.SymbolMetaclass:             "Metaclass",
	symbols.SymbolEnumerationDef:        "EnumerationDefinition",
	symbols.SymbolViewDef:               "ViewDefinition",
	symbols.SymbolViewpointDef:          "ViewpointDefinition",
	symbols.SymbolRenderingDef:          "RenderingDefinition",
	symbols.SymbolConcernDef:            "ConcernDefinition",
	symbols.SymbolConnectionDef:         "ConnectionDefinition",
	symbols.SymbolFlowDef:               "FlowDefinition",
	symbols.SymbolPortDef:               "PortDefinition",
	symbols.SymbolInterfaceDef:          "InterfaceDefinition",
	symbols.SymbolAllocationDef:         "AllocationDefinition",
	symbols.SymbolActionDef:             "ActionDefinition",
	symbols.SymbolStateDef:              "StateDefinition",
	symbols.SymbolCalcDef:               "CalculationDefinition",
	symbols.SymbolConstraintDef:         "ConstraintDefinition",
	symbols.SymbolRequirementDef:        "RequirementDefinition",
	symbols.SymbolCaseDef:               "CaseDefinition",
	symbols.SymbolAnalysisCaseDef:       "AnalysisCaseDefinition",
	symbols.SymbolVerificationCaseDef:   "VerificationCaseDefinition",
	symbols.SymbolUseCaseDef:            "UseCaseDefinition",
	symbols.SymbolItemUsage:             "ItemUsage",
	symbols.SymbolOccurrenceUsage:       "OccurrenceUsage",
	symbols.SymbolIndividualUsage:       "OccurrenceUsage",
	symbols.SymbolMetadataUsage:         "MetadataUsage",
	symbols.SymbolEnumerationUsage:      "EnumerationUsage",
	symbols.SymbolViewUsage:             "ViewUsage",
	symbols.SymbolViewpointUsage:        "ViewpointUsage",
	symbols.SymbolRenderingUsage:        "RenderingUsage",
	symbols.SymbolConcernUsage:          "ConcernUsage",
	symbols.SymbolConnectionUsage:       "ConnectionUsage",
	symbols.SymbolFlowUsage:             "FlowUsage",
	symbols.SymbolPortUsage:             "PortUsage",
	symbols.SymbolInterfaceUsage:        "InterfaceUsage",
	symbols.SymbolAllocationUsage:       "AllocationUsage",
	symbols.SymbolActionUsage:           "ActionUsage",
	symbols.SymbolStateUsage:            "StateUsage",
	symbols.SymbolCalcUsage:             "CalculationUsage",
	symbols.SymbolConstraintUsage:       "ConstraintUsage",
	symbols.SymbolRequirementUsage:      "RequirementUsage",
	symbols.SymbolCaseUsage:             "CaseUsage",
	symbols.SymbolAnalysisCaseUsage:     "AnalysisCaseUsage",
	symbols.SymbolVerificationCaseUsage: "VerificationCaseUsage",
	symbols.SymbolUseCaseUsage:          "UseCaseUsage",
	symbols.SymbolConnectorEnd:          "Feature",
}

// MetamodelTypeName returns the metamodel type name a symbol kind reports as
// `@type`, or "" for a kind that has none, which is only an unclassified
// declaration. An element with no type name never matches a `@type` comparison.
func MetamodelTypeName(kind symbols.SymbolKind) string {
	return metamodelTypeNames[kind]
}

// Query evaluates a SysML v2 API & Services Query against a parsed model.
func (s *Service) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}
	if req.Query == nil {
		return nil, queryErrorf(QueryErrMalformedConstraint, "query is unset")
	}

	sc := cached.SymbolContext()
	defer sc.Lock()()
	eval := &queryEval{sc: sc}

	candidates, err := eval.candidates(cached, req.Query.Scope)
	if err != nil {
		return nil, err
	}
	projection, err := projectedProperties(req.Query.Select)
	if err != nil {
		return nil, err
	}

	elements := make([]*pb.QueryResultElement, 0, len(candidates))
	for _, sym := range candidates {
		matched, err := eval.matches(sym, req.Query.Where)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		elements = append(elements, eval.project(sym, projection))
	}
	return &pb.QueryResponse{Elements: elements}, nil
}

// queryEval evaluates one query over a model's symbol context. It holds no
// element state of its own: every property is read from the index and semantic
// model on demand.
type queryEval struct {
	sc *SymbolContext
}

// candidates returns the elements a query considers, in declaration order: the
// whole loaded document when the scope is empty, else each scoped element and
// everything nested inside it.
func (e *queryEval) candidates(cached *CachedModel, scope []string) ([]*symbols.Symbol, error) {
	if len(scope) == 0 {
		root := e.sc.Index.DocumentRoot(cached.Source.Name())
		if root == nil {
			return nil, nil
		}
		return e.collect(root), nil
	}

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, fqn := range scope {
		roots := e.sc.Index.LookupQualified(fqn)
		if len(roots) == 0 {
			return nil, queryErrorf(QueryErrUnknownScope,
				"query scope names an element the model does not have: %q", fqn)
		}
		for _, root := range roots {
			if seen[root] {
				continue
			}
			seen[root] = true
			out = append(out, root)
			for _, nested := range e.nested(root) {
				if !seen[nested] {
					seen[nested] = true
					out = append(out, nested)
				}
			}
		}
	}
	return out, nil
}

// nested returns every element declared inside an element, in declaration
// order. It is what a scope entry expands to.
func (e *queryEval) nested(sym *symbols.Symbol) []*symbols.Symbol {
	w := &elementWalk{eval: e, seen: map[*symbols.Symbol]bool{sym: true}, walked: map[*symbols.Scope]bool{}}
	w.members(sym)
	return w.out
}

// collect returns every element declared in a scope tree, in declaration order,
// parents before the elements they own.
func (e *queryEval) collect(scope *symbols.Scope) []*symbols.Symbol {
	w := &elementWalk{eval: e, seen: map[*symbols.Symbol]bool{}, walked: map[*symbols.Scope]bool{}}
	w.scope(scope)
	return w.out
}

// elementWalk enumerates elements over the scope tree, without duplicates.
type elementWalk struct {
	eval   *queryEval
	out    []*symbols.Symbol
	seen   map[*symbols.Symbol]bool
	walked map[*symbols.Scope]bool
}

// scope walks a scope's members. A scope no symbol owns — a loop body, a body
// expression's parameters — is walked through, since it still declares elements.
func (w *elementWalk) scope(s *symbols.Scope) {
	if s == nil || w.walked[s] {
		return
	}
	w.walked[s] = true
	for _, sym := range append(s.Members(), s.AnonymousMembers()...) {
		w.visit(sym)
	}
	for _, child := range s.Children() {
		w.scope(child)
	}
}

// visit reports an element and walks what it declares. One with no queryable
// identity is walked through but not reported: the standard identifies an
// element by `@id`, and it has none to be told apart or named by.
func (w *elementWalk) visit(sym *symbols.Symbol) {
	if w.seen[sym] {
		return
	}
	w.seen[sym] = true
	if w.eval.identifies(sym) {
		w.out = append(w.out, sym)
	}
	w.members(sym)
}

// identifies reports whether an element's qualified name is the `@id` the
// standard's clients expect: a real qualified name that names this element back,
// so a later query may use it as a scope.
func (e *queryEval) identifies(sym *symbols.Symbol) bool {
	fqn := e.sc.Index.GetFQN(sym)
	if !hasQualifiedIdentity(fqn) {
		return false
	}
	return slices.Contains(e.sc.Index.LookupQualified(fqn), sym)
}

// hasQualifiedIdentity reports whether a qualified name identifies an element.
// An unnamed one — a doc note, an anonymous usage — has an empty segment, so its
// name is neither unique nor a name a scope could use.
func hasQualifiedIdentity(fqn string) bool {
	if fqn == "" {
		return false
	}
	for _, segment := range strings.Split(fqn, "::") {
		if segment == "" {
			return false
		}
	}
	return true
}

// members walks what an element declares. A library element restored from cache
// owns no scope, so its members are reached through the index by qualified name.
func (w *elementWalk) members(sym *symbols.Symbol) {
	if sym.Scope != nil {
		w.scope(sym.Scope)
		return
	}
	for _, child := range w.eval.sc.Index.LookupDirectChildren(w.eval.sc.Index.GetFQN(sym)) {
		w.visit(child)
	}
}

// projectedProperties returns the properties to report, validating each name.
// An empty selection reports every property.
func projectedProperties(selected []string) ([]string, error) {
	if len(selected) == 0 {
		return QueryPropertyNames(), nil
	}
	out := make([]string, 0, len(selected))
	for _, name := range selected {
		if _, ok := queryProperties[name]; !ok {
			return nil, unknownProperty(name)
		}
		out = append(out, name)
	}
	return out, nil
}

func unknownProperty(name string) *QueryError {
	return queryErrorf(QueryErrUnknownProperty,
		"unknown query property %q; queryable properties are %s",
		name, strings.Join(QueryPropertyNames(), ", "))
}

// project builds the record of a matched element: its identity and type always,
// plus the selected properties it has.
func (e *queryEval) project(sym *symbols.Symbol, selected []string) *pb.QueryResultElement {
	element := &pb.QueryResultElement{
		Id:         e.sc.Index.GetFQN(sym),
		Type:       MetamodelTypeName(sym.Kind),
		Properties: make(map[string]string, len(selected)),
	}
	for _, name := range selected {
		if value, ok := queryProperties[name].read(e, sym); ok {
			element.Properties[name] = value
		}
	}
	return element
}

// matches reports whether an element satisfies a constraint. An unset
// constraint matches every element, as a query with no `where` selects its
// whole scope.
func (e *queryEval) matches(sym *symbols.Symbol, constraint *pb.Constraint) (bool, error) {
	if constraint == nil {
		return true, nil
	}
	switch form := constraint.Constraint.(type) {
	case *pb.Constraint_Primitive:
		return e.matchesPrimitive(sym, form.Primitive)
	case *pb.Constraint_Composite:
		return e.matchesComposite(sym, form.Composite)
	default:
		return false, queryErrorf(QueryErrMalformedConstraint,
			"constraint is neither a PrimitiveConstraint nor a CompositeConstraint")
	}
}

// matchesPrimitive compares one property of an element, negating the verdict
// when the constraint is inverse.
func (e *queryEval) matchesPrimitive(sym *symbols.Symbol, c *pb.PrimitiveConstraint) (bool, error) {
	if c == nil {
		return false, queryErrorf(QueryErrMalformedConstraint, "PrimitiveConstraint is unset")
	}
	property, ok := queryProperties[c.Property]
	if !ok {
		return false, unknownProperty(c.Property)
	}
	if len(c.Value) == 0 {
		return false, queryErrorf(QueryErrMalformedConstraint,
			"PrimitiveConstraint on %q has no value to compare against", c.Property)
	}

	value, has := property.read(e, sym)
	var verdict bool
	switch c.Operator {
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:
		verdict = has && equalsAny(value, c.Value)
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER,
		pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:
		var err error
		verdict, err = compareOrdered(c, property, value, has)
		if err != nil {
			return false, err
		}
	default:
		return false, queryErrorf(QueryErrMalformedConstraint,
			"PrimitiveConstraint on %q has no operator", c.Property)
	}
	if c.Inverse {
		return !verdict, nil
	}
	return verdict, nil
}

// equalsAny reports whether the property's value is one of the constraint's.
// The standard writes a single value, and its clients also write a list for
// `@type`; one value is the degenerate case of the same rule.
func equalsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

// compareOrdered evaluates > or <. A non-ordered property or an operand that is
// not one number is a fault in the query; an element whose own value is absent or
// unparsable simply fails the comparison, which is a fact about the element.
func compareOrdered(c *pb.PrimitiveConstraint, property queryProperty, value string, has bool) (bool, error) {
	if !property.ordered {
		return false, queryErrorf(QueryErrUnorderedProperty,
			"query property %q is not ordered, so %s cannot compare it",
			c.Property, operatorText(c.Operator))
	}
	if len(c.Value) != 1 {
		return false, queryErrorf(QueryErrMalformedConstraint,
			"%s on %q compares against exactly one value, got %d",
			operatorText(c.Operator), c.Property, len(c.Value))
	}
	operand, err := parseOrdered(c.Value[0])
	if err != nil {
		return false, queryErrorf(QueryErrUnparsableValue,
			"%s on %q needs a number to compare against, got %q",
			operatorText(c.Operator), c.Property, c.Value[0])
	}
	if !has {
		return false, nil
	}
	actual, err := parseOrdered(value)
	if err != nil {
		return false, nil
	}
	if c.Operator == pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER {
		return actual > operand, nil
	}
	return actual < operand, nil
}

// parseOrdered reads a number, accepting the unbounded multiplicity "*" as the
// infinity it denotes.
func parseOrdered(text string) (float64, error) {
	if text == "*" {
		return math.Inf(1), nil
	}
	return strconv.ParseFloat(text, 64)
}

// operatorText renders an operator as the standard writes it.
func operatorText(op pb.PrimitiveOperator) string {
	switch op {
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:
		return "="
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER:
		return ">"
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:
		return "<"
	default:
		return "(no operator)"
	}
}

// matchesComposite combines the verdicts of nested constraints. An empty list
// is a fault in the query: neither verdict is defensible.
func (e *queryEval) matchesComposite(sym *symbols.Symbol, c *pb.CompositeConstraint) (bool, error) {
	if c == nil {
		return false, queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint is unset")
	}
	if len(c.Constraint) == 0 {
		return false, queryErrorf(QueryErrMalformedConstraint,
			"CompositeConstraint has no constraints to combine")
	}
	switch c.Operator {
	case pb.CompositeOperator_COMPOSITE_OPERATOR_AND:
		for _, nested := range c.Constraint {
			matched, err := e.matches(sym, nested)
			if err != nil || !matched {
				return false, err
			}
		}
		return true, nil
	case pb.CompositeOperator_COMPOSITE_OPERATOR_OR:
		for _, nested := range c.Constraint {
			matched, err := e.matches(sym, nested)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, queryErrorf(QueryErrMalformedConstraint,
			"CompositeConstraint has no operator")
	}
}
