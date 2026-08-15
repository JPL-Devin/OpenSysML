package semantics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Element filters (KerML 8.2.4 ElementFilterMembership, SysML v2 7.4.4) select
// which of a namespace's imported memberships are memberships of it, and which
// elements an import brings in. A filter condition is a predicate over one
// candidate *element* — not over a value — evaluated with the candidate as the
// implicit `self`, so it is answered here, against symbols, and not by the
// runtime value evaluator, which has no instance to evaluate against while names
// are still being resolved.
//
// Evaluation is in two steps, both memoized: a condition is compiled once to a
// symbols.FilterPredicate whose element references are resolved, and the
// predicate is then run against each candidate. The compiled form is also what a
// library carries in its index cache, so a filtered import selects the same
// elements whether its library was parsed or restored.
//
// Reading a feature bound to nothing yields the empty sequence, which orderings
// (declared over `DataValue[1]`) propagate and `==`/`!=` (over `[0..1]`) decide
// against any value. Nothing is not true, so the candidate is not selected: a
// verdict, not the unevaluable case, and so not reported.

var (
	// ErrFilterUnevaluable reports a filter condition outside the subset of
	// predicates the evaluator implements, or one naming something that does
	// not resolve. Such a condition selects nothing and rejects nothing: it is
	// reported, and the candidate is kept, so that an unevaluable filter never
	// silently hides model content.
	ErrFilterUnevaluable = errors.New("filter condition cannot be evaluated")

	// ErrFilterNotBoolean reports a filter condition that evaluates to
	// something other than a boolean, which cannot select elements.
	ErrFilterNotBoolean = errors.New("filter condition is not boolean-valued")
)

// FilterError is why a filter condition could not decide a candidate. Reason
// describes the part at fault in the terms a diagnostic message uses, and Span
// locates it.
type FilterError struct {
	Reason string
	Span   source.Span
	Err    error
}

func (e *FilterError) Error() string {
	if e.Reason == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + e.Reason
}

func (e *FilterError) Unwrap() error { return e.Err }

// filterKey memoizes one candidate's verdict for one compiled condition.
type filterKey struct {
	pred *symbols.FilterPredicate
	cand *symbols.Symbol
}

// filterVerdict is a decided candidate: the truth value, or why there is none.
type filterVerdict struct {
	value bool
	err   error
}

// SatisfiesElementFilter reports whether cand is selected by the filter
// condition. It is the form the symbol layer's enumeration installs (see
// symbols.Index.SetElementFilter): a condition that cannot be evaluated, or that
// is not boolean-valued, keeps the candidate rather than dropping it — the
// diagnostic is what reports it (see passes.checkElementFilters), because losing
// model content silently is worse than surfacing an element a filter meant to
// hide.
func (m *Model) SatisfiesElementFilter(f symbols.ElementFilter, cand *symbols.Symbol) bool {
	ok, err := m.EvalElementFilter(f, cand)
	if err != nil {
		return true
	}
	return ok
}

// EvalElementFilter evaluates a filter condition for one candidate element,
// returning whether the candidate is selected, or a *FilterError saying why no
// verdict is possible.
func (m *Model) EvalElementFilter(f symbols.ElementFilter, cand *symbols.Symbol) (bool, error) {
	if cand == nil {
		return true, &FilterError{Err: ErrFilterUnevaluable, Reason: "there is no element to evaluate it for", Span: f.Span}
	}
	pred := m.CompileElementFilter(f)
	if pred == nil {
		return true, &FilterError{Err: ErrFilterUnevaluable, Reason: "the condition is empty", Span: f.Span}
	}
	key := filterKey{pred: pred, cand: cand}
	if v, ok := m.filterVerdicts[key]; ok {
		return v.value, v.err
	}
	val, err := m.evalPredicate(pred, cand)
	if err == nil && val.Kind == symbols.FilterValueEmpty {
		val = boolValue(false) // nothing is not true, so it selects nothing
	}
	if err == nil && val.Kind != symbols.FilterValueBool {
		err = &FilterError{Err: ErrFilterNotBoolean, Reason: describeValueKind(val.Kind), Span: pred.Span}
	}
	verdict := filterVerdict{value: err == nil && val.Bool, err: err}
	// Only a decided verdict is remembered. A condition can be unevaluable
	// because the element it names is not indexed yet — an import is expanded
	// while the index is still filling — and re-deciding it later is what keeps
	// the answer from depending on when it was first asked.
	if err == nil {
		m.filterVerdicts[key] = verdict
	}
	return verdict.value, verdict.err
}

// CompileElementFilter returns the condition compiled to a predicate over a
// candidate element: for a parsed condition by resolving the elements it names
// against the scope it was written in, and for a restored library by taking the
// compiled predicate its record carried. Returns nil for an empty condition.
func (m *Model) CompileElementFilter(f symbols.ElementFilter) *symbols.FilterPredicate {
	if f.Expr == nil {
		return f.Pred
	}
	if pred, ok := m.filterPreds[f.Expr]; ok {
		return pred
	}
	// The names a condition uses are not subject to the condition itself, so
	// they resolve through the namespace's imports unfiltered.
	var pred *symbols.FilterPredicate
	m.resolver.InCondition(func() { pred = m.compileCondition(f.Scope, f.Expr) })
	m.filterPreds[f.Expr] = pred
	return pred
}

// compileCondition compiles one filter expression, resolving every element it
// names. A part of the condition it does not implement compiles to a
// FilterUnsupported node carrying the reason, so that the whole condition is
// still a predicate — one that reports instead of deciding.
func (m *Model) compileCondition(scope *symbols.Scope, n ast.Node) *symbols.FilterPredicate {
	switch e := n.(type) {
	case *ast.OperatorExpr:
		return m.compileOperator(scope, e)
	case *ast.FeatureChainExpr:
		return m.compileFeatureChain(scope, e)
	case *ast.FeatureReference:
		return m.compileReference(scope, e.Name, spanOf(e))
	case *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralBool, *ast.LiteralInfinity:
		if v, ok := evalConst(n); ok {
			return &symbols.FilterPredicate{Op: symbols.FilterConst, Value: constValue(v), Span: spanOf(n)}
		}
	case *ast.LiteralString:
		return &symbols.FilterPredicate{
			Op:    symbols.FilterConst,
			Value: symbols.FilterValue{Kind: symbols.FilterValueString, Str: unquote(e.Value)},
			Span:  spanOf(e),
		}
	}
	return unsupported(spanOf(n), fmt.Sprintf("%s is not a supported filter condition", describeNode(n)))
}

// compileOperator compiles the operators a filter condition is written with:
// classification against a metadata type, boolean composition, and comparison.
func (m *Model) compileOperator(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	switch e.Operator {
	case ast.OpAt, ast.OpMetaAt:
		return m.compileClassification(scope, e)

	case ast.OpNot:
		if len(e.Operands) != 1 {
			return unsupported(span, "`not` needs one operand")
		}
		return &symbols.FilterPredicate{
			Op:       symbols.FilterNot,
			Operands: []*symbols.FilterPredicate{m.compileCondition(scope, e.Operands[0])},
			Span:     span,
		}

	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr,
		ast.OpXor, ast.OpImplies, ast.OpEq, ast.OpNeq, ast.OpLt, ast.OpLe,
		ast.OpGt, ast.OpGe:
		if len(e.Operands) != 2 {
			return unsupported(span, fmt.Sprintf("`%s` needs two operands", e.Operator))
		}
		return &symbols.FilterPredicate{
			Op: binaryFilterOp(e.Operator),
			Operands: []*symbols.FilterPredicate{
				m.compileCondition(scope, e.Operands[0]),
				m.compileCondition(scope, e.Operands[1]),
			},
			Span: span,
		}
	}
	return unsupported(span, fmt.Sprintf("the `%s` operator is not supported in a filter condition", e.Operator))
}

// compileClassification compiles `@T` and `@@T`. The operand is the element the
// condition is evaluated for, which the syntax leaves out; an explicit `self`
// says the same thing, and anything else names another element, which a filter
// condition cannot reach.
func (m *Model) compileClassification(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	op := symbols.FilterClassify
	if e.Operator == ast.OpMetaAt {
		op = symbols.FilterMetaClassify
	}
	if len(e.Operands) > 1 || (len(e.Operands) == 1 && !isSelfReference(e.Operands[0])) {
		return unsupported(span, fmt.Sprintf("`%s` in a filter condition classifies the element being filtered, so it takes no left operand", e.Operator))
	}
	if e.TypeRef == nil {
		return unsupported(span, fmt.Sprintf("`%s` names no type", e.Operator))
	}
	typeFQN, ok := m.resolveFQN(scope, e.TypeRef)
	if !ok {
		return unsupported(span, fmt.Sprintf("the metadata type %s does not resolve", qnText(e.TypeRef)))
	}
	return &symbols.FilterPredicate{Op: op, TypeFQN: typeFQN, Span: span}
}

// compileFeatureChain compiles the value of a feature of an annotation of the
// candidate: `(as Safety).isMandatory` reads isMandatory from the candidate's
// Safety annotation.
func (m *Model) compileFeatureChain(scope *symbols.Scope, e *ast.FeatureChainExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	if e.Member == nil || len(e.Member.Parts) != 1 {
		return unsupported(span, "a filter condition reads a single feature of an annotation")
	}
	feature := e.Member.Parts[0].Text
	switch operand := e.Operand.(type) {
	case *ast.CastExpr:
		if operand.TargetType == nil {
			return unsupported(span, "the cast names no type")
		}
		typeFQN, ok := m.resolveFQN(scope, operand.TargetType)
		if !ok {
			return unsupported(span, fmt.Sprintf("the metadata type %s does not resolve", qnText(operand.TargetType)))
		}
		return &symbols.FilterPredicate{Op: symbols.FilterFeature, TypeFQN: typeFQN, Feature: feature, Span: span}
	case *ast.OperatorExpr:
		// `self.f` and `(self as T).f` are written this way too; only the
		// annotation form carries a metadata type to read the feature from.
		if operand.Operator == ast.OpAs && operand.TypeRef != nil {
			typeFQN, ok := m.resolveFQN(scope, operand.TypeRef)
			if !ok {
				return unsupported(span, fmt.Sprintf("the metadata type %s does not resolve", qnText(operand.TypeRef)))
			}
			return &symbols.FilterPredicate{Op: symbols.FilterFeature, TypeFQN: typeFQN, Feature: feature, Span: span}
		}
	}
	return unsupported(span, "a filter condition reads a feature of an annotation of the filtered element, as in `(as Safety).isMandatory`")
}

// compileReference compiles a name a filter condition uses as a value: a feature
// of a metadata type, read from the candidate's annotation of that type
// (`Safety::level`), or an element compared by identity, such as an enumeration
// literal.
func (m *Model) compileReference(scope *symbols.Scope, qn *ast.QualifiedName, span source.Span) *symbols.FilterPredicate {
	if qn == nil || len(qn.Parts) == 0 {
		return unsupported(span, "the reference names nothing")
	}
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return unsupported(span, fmt.Sprintf("%s does not resolve", qnText(qn)))
	}
	if owner := ownerSymbol(sym); owner != nil && isMetadataType(owner) {
		ownerFQN := m.fqnOf(owner)
		if ownerFQN == "" {
			return unsupported(span, fmt.Sprintf("the metadata type of %s has no qualified name", qnText(qn)))
		}
		return &symbols.FilterPredicate{
			Op:      symbols.FilterFeature,
			TypeFQN: ownerFQN,
			Feature: simpleSymbolName(sym),
			Span:    span,
		}
	}
	fqn := m.fqnOf(sym)
	if fqn == "" {
		return unsupported(span, fmt.Sprintf("%s has no qualified name to compare", qnText(qn)))
	}
	return &symbols.FilterPredicate{
		Op:    symbols.FilterConst,
		Value: symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn},
		Span:  span,
	}
}

// evalPredicate runs a compiled condition against one candidate element.
func (m *Model) evalPredicate(p *symbols.FilterPredicate, cand *symbols.Symbol) (symbols.FilterValue, error) {
	switch p.Op {
	case symbols.FilterUnsupported:
		return symbols.FilterValue{}, &FilterError{Err: ErrFilterUnevaluable, Reason: p.Reason, Span: p.Span}

	case symbols.FilterConst:
		return p.Value, nil

	case symbols.FilterClassify:
		return boolValue(m.annotatedBy(cand, p.TypeFQN)), nil

	case symbols.FilterMetaClassify:
		return boolValue(m.metaclassConforms(cand, p.TypeFQN)), nil

	case symbols.FilterFeature:
		return m.annotationFeatureValue(cand, p)

	case symbols.FilterNot:
		v, err := m.evalBool(p.Operands[0], cand)
		if err != nil {
			return symbols.FilterValue{}, err
		}
		if v.Kind == symbols.FilterValueEmpty {
			return v, nil
		}
		return boolValue(!v.Bool), nil

	case symbols.FilterAnd, symbols.FilterOr, symbols.FilterXor, symbols.FilterImplies:
		left, err := m.evalBool(p.Operands[0], cand)
		if err != nil {
			return symbols.FilterValue{}, err
		}
		// Nothing to decide from yields nothing, leaving the candidate
		// unselected rather than the condition rejected.
		if left.Kind == symbols.FilterValueEmpty {
			return left, nil
		}
		// `and`, `or` and `implies` are decided by their left operand alone
		// where it settles the answer, so the right one is not evaluated. This
		// is what makes the guarded form filters are written in work: in
		// `@Safety and (as Safety).level > 4` the feature is only read from an
		// element the guard established has that annotation to read it from.
		if decided, ok := shortCircuit(p.Op, left.Bool); ok {
			return boolValue(decided), nil
		}
		right, err := m.evalBool(p.Operands[1], cand)
		if err != nil {
			return symbols.FilterValue{}, err
		}
		if right.Kind == symbols.FilterValueEmpty {
			return right, nil
		}
		return boolValue(evalFilterBool(p.Op, left.Bool, right.Bool)), nil

	case symbols.FilterEq, symbols.FilterNeq, symbols.FilterLt, symbols.FilterLe,
		symbols.FilterGt, symbols.FilterGe:
		return m.evalComparison(p, cand)
	}
	return symbols.FilterValue{}, &FilterError{Err: ErrFilterUnevaluable, Reason: "unknown filter operation", Span: p.Span}
}

// evalBool evaluates an operand a boolean operator needs: a boolean, or nothing
// at all, which the operator propagates.
func (m *Model) evalBool(p *symbols.FilterPredicate, cand *symbols.Symbol) (symbols.FilterValue, error) {
	v, err := m.evalPredicate(p, cand)
	if err != nil {
		return symbols.FilterValue{}, err
	}
	if v.Kind != symbols.FilterValueBool && v.Kind != symbols.FilterValueEmpty {
		return symbols.FilterValue{}, &FilterError{Err: ErrFilterNotBoolean, Reason: describeValueKind(v.Kind), Span: p.Span}
	}
	return v, nil
}

// shortCircuit reports the value a boolean operator's left operand settles on
// its own, and whether it does.
func shortCircuit(op symbols.FilterOp, left bool) (bool, bool) {
	switch op {
	case symbols.FilterAnd:
		return false, !left
	case symbols.FilterOr:
		return true, left
	case symbols.FilterImplies:
		return true, !left
	default: // symbols.FilterXor needs both operands
		return false, false
	}
}

// evalFilterBool applies a boolean operator to both operands, for the cases the
// left one does not settle.
func evalFilterBool(op symbols.FilterOp, l, r bool) bool {
	switch op {
	case symbols.FilterAnd:
		return l && r
	case symbols.FilterOr:
		return l || r
	case symbols.FilterXor:
		return l != r
	default: // symbols.FilterImplies
		return !l || r
	}
}

// evalComparison compares two operands of a filter condition: numbers and
// booleans by value, strings by text, and elements — an enumeration literal, or
// an annotation feature bound to one — by identity.
func (m *Model) evalComparison(p *symbols.FilterPredicate, cand *symbols.Symbol) (symbols.FilterValue, error) {
	left, err := m.evalPredicate(p.Operands[0], cand)
	if err != nil {
		return symbols.FilterValue{}, err
	}
	right, err := m.evalPredicate(p.Operands[1], cand)
	if err != nil {
		return symbols.FilterValue{}, err
	}
	equality := p.Op == symbols.FilterEq || p.Op == symbols.FilterNeq
	if left.Kind == symbols.FilterValueEmpty || right.Kind == symbols.FilterValueEmpty {
		if !equality { // an ordering needs a value on each side to compare
			return emptyValue(), nil
		}
		// `==` is declared over `[0..1]`: nothing equals only nothing.
		both := left.Kind == right.Kind
		return boolValue(both == (p.Op == symbols.FilterEq)), nil
	}
	if equality && (left.Kind == symbols.FilterValueRef || right.Kind == symbols.FilterValueRef ||
		left.Kind == symbols.FilterValueString || right.Kind == symbols.FilterValueString ||
		left.Kind == symbols.FilterValueBool || right.Kind == symbols.FilterValueBool) {
		if left.Kind != right.Kind {
			return symbols.FilterValue{}, &FilterError{
				Err:    ErrFilterUnevaluable,
				Reason: fmt.Sprintf("comparing %s with %s", describeValueKind(left.Kind), describeValueKind(right.Kind)),
				Span:   p.Span,
			}
		}
		same := left == right
		return boolValue(same == (p.Op == symbols.FilterEq)), nil
	}
	l, lok := numericValue(left)
	r, rok := numericValue(right)
	if !lok || !rok {
		return symbols.FilterValue{}, &FilterError{
			Err:    ErrFilterUnevaluable,
			Reason: fmt.Sprintf("comparing %s with %s", describeValueKind(left.Kind), describeValueKind(right.Kind)),
			Span:   p.Span,
		}
	}
	switch p.Op {
	case symbols.FilterEq:
		return boolValue(l == r), nil
	case symbols.FilterNeq:
		return boolValue(l != r), nil
	case symbols.FilterLt:
		return boolValue(l < r), nil
	case symbols.FilterLe:
		return boolValue(l <= r), nil
	case symbols.FilterGt:
		return boolValue(l > r), nil
	default: // symbols.FilterGe
		return boolValue(l >= r), nil
	}
}

// annotationFeatureValue reads the value the candidate's annotation of the
// predicate's metadata type binds its feature to. A feature nothing binds, or one
// read from an annotation the candidate does not carry, has an empty value
// sequence, which leaves the comparison reading it false rather than unevaluable.
func (m *Model) annotationFeatureValue(cand *symbols.Symbol, p *symbols.FilterPredicate) (symbols.FilterValue, error) {
	typ := m.symbolByFQN(p.TypeFQN)
	for _, a := range m.annotationsOf(cand) {
		if !m.annotationConforms(a, typ, p.TypeFQN) {
			continue
		}
		v, ok := a.values[p.Feature]
		if !ok {
			continue
		}
		if v.Kind == symbols.FilterValueUnknown {
			return symbols.FilterValue{}, &FilterError{
				Err:    ErrFilterUnevaluable,
				Reason: fmt.Sprintf("the value %s::%s is bound to is not a constant", p.TypeFQN, p.Feature),
				Span:   p.Span,
			}
		}
		return v, nil
	}
	return emptyValue(), nil
}

// annotatedBy reports whether the candidate is annotated by metadata conforming
// to the named type: `@Safety` holds for an element annotated with a metadata
// type specializing Safety as well as with Safety itself. The candidate's own
// metaclass counts as such an annotation, which is what makes `@SysML::PartUsage`
// select the part usages among the candidates.
func (m *Model) annotatedBy(cand *symbols.Symbol, typeFQN string) bool {
	typ := m.symbolByFQN(typeFQN)
	for _, a := range m.annotationsOf(cand) {
		if m.annotationConforms(a, typ, typeFQN) {
			return true
		}
	}
	return m.metaclassConforms(cand, typeFQN)
}

// metaclassConforms reports whether the candidate's own metaclass — the KerML
// metaclass of the declaration, `SysML::PartUsage` for a part usage — conforms to
// the named type. It is what `@@T` tests.
func (m *Model) metaclassConforms(cand *symbols.Symbol, typeFQN string) bool {
	meta := m.metaclassOf(cand)
	if meta == nil {
		return false
	}
	if typ := m.symbolByFQN(typeFQN); typ != nil {
		return m.Conforms(meta, typ)
	}
	// The metaclass library is not loaded, so conformance can only be judged on
	// the name the candidate's metaclass has.
	return simpleName(typeFQN) == simpleSymbolName(meta)
}

// annotationConforms reports whether an annotation's metadata type conforms to
// the type a condition names. The types are compared as symbols where both are
// indexed, and by qualified name otherwise, which is what a restored library's
// annotation — recorded as the name of its type — allows.
func (m *Model) annotationConforms(a annotation, typ *symbols.Symbol, typeFQN string) bool {
	if a.typ != nil && typ != nil {
		return m.Conforms(a.typ, typ)
	}
	if a.typFQN != "" && a.typFQN == typeFQN {
		return true
	}
	if a.typ != nil && typeFQN != "" {
		for _, sup := range append([]*symbols.Symbol{a.typ}, m.AllSupertypes(a.typ)...) {
			if m.fqnOf(sup) == typeFQN {
				return true
			}
		}
	}
	return false
}

// symbolByFQN returns the single element registered under a qualified name, or
// nil when the name is unknown or names more than one element.
func (m *Model) symbolByFQN(fqn string) *symbols.Symbol {
	if fqn == "" || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	if sym, ok := m.filterTypes[fqn]; ok {
		return sym
	}
	var found *symbols.Symbol
	if syms := m.resolver.Index().LookupQualified(fqn); len(syms) == 1 {
		found = syms[0]
	}
	m.filterTypes[fqn] = found
	return found
}

// FilterProblem is a fault in a filter condition itself, found without any
// candidate to evaluate it for: a part of it the evaluator cannot decide, or one
// that yields something other than a boolean where the condition needs a truth
// value. Reason describes the fault for a diagnostic message, and Span locates
// the part of the condition at fault.
type FilterProblem struct {
	// NotBoolean distinguishes a condition that has a value but not a boolean
	// one from one that cannot be evaluated at all.
	NotBoolean bool
	Reason     string
	Span       source.Span
}

// CheckElementFilter reports the faults of a filter condition, in the order they
// appear in it. It is what the validation pass reports: the same compiled
// predicate the enumeration evaluates is examined, so a condition diagnosed here
// is exactly one whose verdict is not trusted there.
//
// Only faults that hold for every candidate are reported. Whether an annotation
// actually binds a feature is a property of the candidate, not of the condition,
// so it surfaces as an unevaluable verdict during enumeration rather than here.
func (m *Model) CheckElementFilter(f symbols.ElementFilter) []FilterProblem {
	pred := m.CompileElementFilter(f)
	if pred == nil {
		return []FilterProblem{{Reason: "the condition is empty", Span: f.Span}}
	}
	return appendFilterProblems(nil, pred, true)
}

// appendFilterProblems collects the faults of a compiled condition. wantBool
// says whether the position the predicate occupies needs a truth value: the
// condition as a whole does, as does every operand of a boolean operator.
func appendFilterProblems(out []FilterProblem, p *symbols.FilterPredicate, wantBool bool) []FilterProblem {
	if p == nil {
		return out
	}
	if p.Op == symbols.FilterUnsupported {
		return append(out, FilterProblem{Reason: p.Reason, Span: p.Span})
	}
	if wantBool && filterYieldsBool(p) == filterNotBool {
		out = append(out, FilterProblem{
			NotBoolean: true,
			Reason:     describeValueKind(p.Value.Kind),
			Span:       p.Span,
		})
	}
	operandsWantBool := isFilterBoolOp(p.Op)
	for _, operand := range p.Operands {
		out = appendFilterProblems(out, operand, operandsWantBool)
	}
	return out
}

// filterBoolness is what is known about the kind of value a compiled condition
// yields before it is run against a candidate.
type filterBoolness uint8

const (
	filterBoolUnknown filterBoolness = iota
	filterIsBool
	filterNotBool
)

// filterYieldsBool reports what is known about a predicate's value kind. The
// value of an annotation feature is only known once a candidate is at hand, so a
// feature read is neither.
func filterYieldsBool(p *symbols.FilterPredicate) filterBoolness {
	switch p.Op {
	case symbols.FilterClassify, symbols.FilterMetaClassify, symbols.FilterNot,
		symbols.FilterAnd, symbols.FilterOr, symbols.FilterXor, symbols.FilterImplies,
		symbols.FilterEq, symbols.FilterNeq, symbols.FilterLt, symbols.FilterLe,
		symbols.FilterGt, symbols.FilterGe:
		return filterIsBool
	case symbols.FilterConst:
		if p.Value.Kind == symbols.FilterValueBool {
			return filterIsBool
		}
		return filterNotBool
	default: // symbols.FilterFeature, symbols.FilterUnsupported
		return filterBoolUnknown
	}
}

// isFilterBoolOp reports whether an operation needs boolean operands.
func isFilterBoolOp(op symbols.FilterOp) bool {
	switch op {
	case symbols.FilterNot, symbols.FilterAnd, symbols.FilterOr,
		symbols.FilterXor, symbols.FilterImplies:
		return true
	default:
		return false
	}
}

// unsupported builds the predicate node for a condition the evaluator cannot
// decide, carrying the reason a diagnostic reports.
func unsupported(span source.Span, reason string) *symbols.FilterPredicate {
	return &symbols.FilterPredicate{Op: symbols.FilterUnsupported, Reason: reason, Span: span}
}

// binaryFilterOp maps a binary operator of a filter condition to its predicate
// operation. Only the operators compileOperator admits reach here.
func binaryFilterOp(op ast.OperatorKind) symbols.FilterOp {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return symbols.FilterAnd
	case ast.OpOr, ast.OpConditionalOr:
		return symbols.FilterOr
	case ast.OpXor:
		return symbols.FilterXor
	case ast.OpImplies:
		return symbols.FilterImplies
	case ast.OpEq:
		return symbols.FilterEq
	case ast.OpNeq:
		return symbols.FilterNeq
	case ast.OpLt:
		return symbols.FilterLt
	case ast.OpLe:
		return symbols.FilterLe
	case ast.OpGt:
		return symbols.FilterGt
	default: // ast.OpGe
		return symbols.FilterGe
	}
}

// constValue converts a folded constant to the form a filter predicate holds.
func constValue(v Value) symbols.FilterValue {
	switch v.Kind {
	case ValInt:
		return symbols.FilterValue{Kind: symbols.FilterValueInt, Int: v.Int}
	case ValReal:
		return symbols.FilterValue{Kind: symbols.FilterValueReal, Real: v.Real}
	case ValBool:
		return symbols.FilterValue{Kind: symbols.FilterValueBool, Bool: v.Bool}
	default:
		return symbols.FilterValue{}
	}
}

func boolValue(b bool) symbols.FilterValue {
	return symbols.FilterValue{Kind: symbols.FilterValueBool, Bool: b}
}

// emptyValue is the empty sequence, the value of a feature bound to nothing.
func emptyValue() symbols.FilterValue {
	return symbols.FilterValue{Kind: symbols.FilterValueEmpty}
}

// numericValue returns a value as a float64 for comparison, and whether it is a
// number at all.
func numericValue(v symbols.FilterValue) (float64, bool) {
	switch v.Kind {
	case symbols.FilterValueInt:
		return float64(v.Int), true
	case symbols.FilterValueReal:
		return v.Real, true
	default:
		return 0, false
	}
}

// describeValueKind names a value kind for a diagnostic message.
func describeValueKind(k symbols.FilterValueKind) string {
	switch k {
	case symbols.FilterValueBool:
		return "a boolean"
	case symbols.FilterValueInt:
		return "an integer"
	case symbols.FilterValueReal:
		return "a real"
	case symbols.FilterValueString:
		return "a string"
	case symbols.FilterValueRef:
		return "an element reference"
	case symbols.FilterValueEmpty:
		return "nothing"
	default:
		return "no value"
	}
}

// describeNode names a syntactic form for a diagnostic message.
func describeNode(n ast.Node) string {
	switch n.(type) {
	case nil:
		return "an empty condition"
	case *ast.CastExpr:
		return "a cast on its own"
	case *ast.InvocationExpr:
		return "an invocation"
	case *ast.IndexExpr:
		return "an indexed expression"
	case *ast.BodyExpr:
		return "a body expression"
	case *ast.ErrorNode:
		return "a malformed expression"
	default:
		return "this expression"
	}
}

// isSelfReference reports whether a node is the `self` keyword used as the
// operand of a classification, which names the element being filtered.
func isSelfReference(n ast.Node) bool {
	ref, ok := n.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) != 1 {
		return false
	}
	return ref.Name.Parts[0].Text == "self"
}

// resolveFQN resolves a qualified name written in scope to the name the element
// it names is indexed under.
func (m *Model) resolveFQN(scope *symbols.Scope, qn *ast.QualifiedName) (string, bool) {
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return "", false
	}
	fqn := m.fqnOf(sym)
	return fqn, fqn != ""
}

// spanOf is a node's span, or the zero span for a missing node.
func spanOf(n ast.Node) source.Span {
	if n == nil {
		return source.Span{}
	}
	return n.Span()
}

// qnText renders a qualified name as written.
func qnText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return strings.Join(parts, "::")
}

// simpleName is the last segment of a qualified name.
func simpleName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+2:]
	}
	return fqn
}

// simpleSymbolName is a symbol's own name, without the qualification an indexed
// symbol's name may carry.
func simpleSymbolName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	return simpleName(sym.Name)
}

// ownerSymbol returns the element a symbol is a member of.
func ownerSymbol(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// isMetadataType reports whether a symbol is a metadata definition or a KerML
// metaclass — the types an annotation can have.
func isMetadataType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	return sym.Kind == symbols.SymbolMetadataDef || sym.Kind == symbols.SymbolMetaclass
}

// unquote strips the quotes from a string literal's text.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
