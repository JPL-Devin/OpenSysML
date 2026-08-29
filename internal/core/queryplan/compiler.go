package queryplan

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const queryBaseFQN = "DocumentQueries::Query"

type builtin struct {
	operation  Operation
	parameters []string
}

var builtins = map[string]builtin{
	"DocumentQueries::OwnedElements":   {OperationOwnedElements, []string{"source"}},
	"DocumentQueries::Descendants":     {OperationDescendants, []string{"source", "maxDepth"}},
	"DocumentQueries::Ancestors":       {OperationAncestors, []string{"source", "maxDepth"}},
	"DocumentQueries::RelatedElements": {OperationRelatedElements, []string{"source", "relationshipKind", "direction", "maxDepth"}},
	"DocumentQueries::WhereType":       {OperationWhereType, []string{"source", "type"}},
	"DocumentQueries::WhereMetadata":   {OperationWhereMetadata, []string{"source", "metadata"}},
	"DocumentQueries::WhereName":       {OperationWhereName, []string{"source", "operator", "value"}},
	"DocumentQueries::WhereFeature":    {OperationWhereFeature, []string{"source", "feature", "operator", "value"}},
	"DocumentQueries::OrderBy":         {OperationOrderBy, []string{"source", "property", "direction", "missing", "multiple"}},
	"DocumentQueries::Project":         {OperationProject, []string{"source", "properties"}},
}

type compileState uint8

const (
	stateUnseen compileState = iota
	stateVisiting
	stateDone
)

type compiler struct {
	index       *symbols.Index
	model       *semantics.Model
	resolver    *resolve.Resolver
	state       map[*symbols.Symbol]compileState
	stack       []*symbols.Symbol
	definitions []Definition
}

// IsQueryDefinition reports whether sym specializes DocumentQueries::Query.
func IsQueryDefinition(index *symbols.Index, model *semantics.Model, sym *symbols.Symbol) bool {
	base := queryBase(index)
	return base != nil && sym != nil && sym != base &&
		sym.Kind == symbols.SymbolCalcDef && model != nil && model.Conforms(sym, base)
}

// Compile compiles entry and every query it invokes into dependency order.
func Compile(index *symbols.Index, model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*Program, error) {
	if index == nil || model == nil || resolver == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	base := queryBase(index)
	if base == nil {
		return nil, &Error{Kind: ErrorLibraryUnavailable}
	}
	name := symbols.FQNOf(entry)
	if entry == nil || entry == base || entry.Kind != symbols.SymbolCalcDef || !model.Conforms(entry, base) {
		return nil, &Error{Kind: ErrorNotQueryDefinition, Query: name, Span: symbolSpan(entry)}
	}
	c := &compiler{
		index:    index,
		model:    model,
		resolver: resolver,
		state:    make(map[*symbols.Symbol]compileState),
	}
	if err := c.compileDefinition(entry); err != nil {
		return nil, err
	}
	return &Program{entry: name, definitions: c.definitions}, nil
}

func queryBase(index *symbols.Index) *symbols.Symbol {
	if index == nil {
		return nil
	}
	matches := symbols.PreferDeclared(index.LookupQualified(queryBaseFQN))
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

func (c *compiler) compileDefinition(sym *symbols.Symbol) error {
	switch c.state[sym] {
	case stateDone:
		return nil
	case stateVisiting:
		return c.cycleError(sym)
	}
	c.state[sym] = stateVisiting
	c.stack = append(c.stack, sym)

	params, result, err := c.signature(sym)
	if err != nil {
		return err
	}
	if result.Name == "" {
		return &Error{
			Kind:  ErrorMissingResultParameter,
			Query: symbols.FQNOf(sym),
			Span:  sym.DeclSpan,
		}
	}
	resultExpression, err := c.resultExpression(sym)
	if err != nil {
		return err
	}
	dependencies := make([]string, 0)
	seenDependencies := make(map[string]bool)
	expression, err := c.compileExpression(sym, resultExpression.owner, params, resultExpression.node, func(name string) {
		if !seenDependencies[name] {
			seenDependencies[name] = true
			dependencies = append(dependencies, name)
		}
	})
	if err != nil {
		return err
	}

	c.stack = c.stack[:len(c.stack)-1]
	c.state[sym] = stateDone
	c.definitions = append(c.definitions, Definition{
		name:         symbols.FQNOf(sym),
		parameters:   params,
		result:       result,
		expression:   expression,
		dependencies: dependencies,
		origin:       provenance.Symbol(sym),
	})
	return nil
}

func (c *compiler) signature(sym *symbols.Symbol) ([]Parameter, Parameter, error) {
	effective := c.model.BehaviorParametersOf(sym)
	params := make([]Parameter, 0, len(effective))
	var result Parameter
	for _, item := range effective {
		param := c.parameter(item.Symbol)
		if item.IsResult {
			result = param
			continue
		}
		if item.Direction != ast.DirIn {
			return nil, Parameter{}, &Error{
				Kind:      ErrorInvalidParameter,
				Query:     symbols.FQNOf(sym),
				Parameter: param.Name,
				Span:      item.Symbol.DeclSpan,
			}
		}
		params = append(params, param)
	}
	return params, result, nil
}

func (c *compiler) parameter(sym *symbols.Symbol) Parameter {
	param := Parameter{
		Name:         sym.Name,
		Type:         c.parameterType(sym),
		Multiplicity: c.parameterMultiplicity(sym),
		Origin:       provenance.Symbol(sym),
	}
	for _, candidate := range c.parameterLineage(sym) {
		if usage, ok := candidate.Decl.(*ast.Usage); ok && usage.Value != nil {
			param.HasDefault = true
			break
		}
	}
	return param
}

func (c *compiler) parameterType(sym *symbols.Symbol) string {
	for _, candidate := range c.parameterLineage(sym) {
		for _, relationship := range semantics.RelationshipsOf(candidate) {
			if relationship == nil || relationship.Kind != ast.RelTyping {
				continue
			}
			if target, ok := c.resolver.ResolveTarget(candidate.OwnerScope, relationship.Target); ok {
				return symbols.FQNOf(target)
			}
		}
	}
	return ""
}

func (c *compiler) parameterMultiplicity(sym *symbols.Symbol) Multiplicity {
	rng := semantics.AssumedRange()
	for _, candidate := range c.parameterLineage(sym) {
		if stated, ok := c.model.MultiplicityOf(candidate); ok {
			rng = stated
			break
		}
	}
	return Multiplicity{
		Lower:         rng.Lower.Value,
		Upper:         rng.Upper.Value,
		UpperInfinite: rng.Upper.Infinite,
		Known:         rng.Lower.Known && rng.Upper.Known,
	}
}

func (c *compiler) parameterLineage(sym *symbols.Symbol) []*symbols.Symbol {
	lineage := []*symbols.Symbol{sym}
	for _, candidate := range c.model.AllSupertypes(sym) {
		usage, ok := candidate.Decl.(*ast.Usage)
		if ok && usage.Direction != ast.DirNone {
			lineage = append(lineage, candidate)
		}
	}
	return lineage
}

type effectiveResult struct {
	node  ast.Node
	owner *symbols.Symbol
}

func (c *compiler) resultExpression(sym *symbols.Symbol) (effectiveResult, error) {
	results, err := c.effectiveResults(sym, make(map[*symbols.Symbol]bool))
	if err != nil {
		return effectiveResult{}, err
	}
	switch len(results) {
	case 0:
		return effectiveResult{}, &Error{
			Kind:  ErrorMissingResult,
			Query: symbols.FQNOf(sym),
			Span:  sym.DeclSpan,
		}
	case 1:
		return results[0], nil
	default:
		return effectiveResult{}, &Error{
			Kind:  ErrorConflictingResult,
			Query: symbols.FQNOf(sym),
			Span:  sym.DeclSpan,
		}
	}
}

func (c *compiler) effectiveResults(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) ([]effectiveResult, error) {
	if sym == nil || visiting[sym] {
		return nil, nil
	}
	if result, stated, err := declaredResult(sym); err != nil {
		return nil, err
	} else if stated {
		return []effectiveResult{result}, nil
	}

	visiting[sym] = true
	defer delete(visiting, sym)
	if sym == queryBase(c.index) {
		return nil, nil
	}
	var results []effectiveResult
	for _, general := range c.model.DirectSupertypes(sym) {
		if general == nil || general.Kind != symbols.SymbolCalcDef ||
			(general != queryBase(c.index) && !IsQueryDefinition(c.index, c.model, general)) {
			continue
		}
		inherited, err := c.effectiveResults(general, visiting)
		if err != nil {
			return nil, err
		}
		results = append(results, inherited...)
	}
	return c.mostSpecificResults(results), nil
}

func (c *compiler) mostSpecificResults(results []effectiveResult) []effectiveResult {
	var effective []effectiveResult
	for _, candidate := range results {
		keep := true
		for i := 0; i < len(effective); {
			current := effective[i]
			switch {
			case candidate.owner == current.owner || c.model.Conforms(current.owner, candidate.owner):
				keep = false
			case c.model.Conforms(candidate.owner, current.owner):
				effective = append(effective[:i], effective[i+1:]...)
				continue
			}
			i++
		}
		if keep {
			effective = append(effective, candidate)
		}
	}
	return effective
}

func declaredResult(sym *symbols.Symbol) (effectiveResult, bool, error) {
	name := symbols.FQNOf(sym)
	members := declarationMembers(sym)
	statements := lower.CalcBody(members, sym.Scope)
	if len(statements) == 1 {
		if result, ok := statements[0].(lower.Return); ok && result.Value != nil {
			return effectiveResult{node: result.Value, owner: sym}, true, nil
		}
	}
	if len(statements) > 0 {
		return effectiveResult{}, false, &Error{Kind: ErrorUnsupportedResult, Query: name, Span: sym.DeclSpan}
	}

	var expression ast.Node
	for _, binding := range lower.ToBindings(sym.Decl, sym.Scope) {
		for i := range binding.Ends {
			if binding.Ends[i].Path != "result" {
				continue
			}
			if expression != nil {
				return effectiveResult{}, false, &Error{Kind: ErrorUnsupportedResult, Query: name, Span: binding.Decl.Span()}
			}
			expression = binding.Ends[1-i].Expr
		}
	}
	if expression == nil {
		return effectiveResult{}, false, nil
	}
	return effectiveResult{node: expression, owner: sym}, true, nil
}

func declarationMembers(sym *symbols.Symbol) []ast.Node {
	switch declaration := sym.Decl.(type) {
	case *ast.Definition:
		return declaration.Members
	case *ast.Usage:
		return declaration.Members
	default:
		return nil
	}
}

func (c *compiler) compileExpression(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	node ast.Node,
	dependency func(string),
) (Expression, error) {
	switch expression := node.(type) {
	case *ast.FeatureReference:
		return c.compileParameterReference(query, owner, expression)
	case *ast.InvocationExpr:
		return c.compileInvocation(query, owner, params, expression, dependency)
	case *ast.SequenceExpr:
		args := make([]Argument, 0, len(expression.Elements))
		for _, element := range expression.Elements {
			value, err := c.compileExpression(query, owner, params, element, dependency)
			if err != nil {
				return Expression{}, err
			}
			args = append(args, Argument{Value: value})
		}
		return Expression{
			operation: OperationSequence,
			arguments: args,
			origin:    provenance.Node(owner.DocName, node),
		}, nil
	case *ast.LiteralString:
		return literalExpression(owner, node, LiteralString, expression.Value), nil
	case *ast.LiteralInteger:
		return literalExpression(owner, node, LiteralInteger, expression.Value), nil
	case *ast.LiteralReal:
		return literalExpression(owner, node, LiteralReal, expression.Value), nil
	case *ast.LiteralBool:
		return literalExpression(owner, node, LiteralBoolean, strconv.FormatBool(expression.Value)), nil
	case *ast.LiteralInfinity:
		return literalExpression(owner, node, LiteralInfinity, "*"), nil
	case *ast.NullExpr:
		return literalExpression(owner, node, LiteralNull, "null"), nil
	default:
		return Expression{}, &Error{
			Kind:  ErrorUnsupportedExpression,
			Query: symbols.FQNOf(query),
			Span:  node.Span(),
		}
	}
}

func (c *compiler) compileParameterReference(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	expression *ast.FeatureReference,
) (Expression, error) {
	target, ok := c.resolver.ResolveQualified(owner.Scope, expression.Name)
	if ok {
		for _, param := range c.model.BehaviorParametersOf(query) {
			if !param.IsResult && c.parameterIncludes(param.Symbol, target) {
				return Expression{
					operation: OperationParameter,
					target:    param.Symbol.Name,
					origin:    provenance.Node(owner.DocName, expression),
				}, nil
			}
		}
	}
	name := qualifiedName(expression.Name)
	return Expression{}, &Error{
		Kind:      ErrorUnknownParameter,
		Query:     symbols.FQNOf(query),
		Parameter: name,
		Span:      expression.Span(),
	}
}

func (c *compiler) parameterIncludes(param, target *symbols.Symbol) bool {
	if param == target {
		return true
	}
	for _, inherited := range c.model.AllSupertypes(param) {
		if inherited == target {
			return true
		}
	}
	return false
}

func (c *compiler) compileInvocation(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	expression *ast.InvocationExpr,
	dependency func(string),
) (Expression, error) {
	name := qualifiedName(expression.Type)
	if expression.Operand != nil {
		return Expression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Target: name,
			Span:   expression.Span(),
		}
	}
	target, ok := c.resolver.ResolveQualified(owner.Scope, expression.Type)
	if !ok {
		return Expression{}, &Error{
			Kind:   ErrorUnknownInvocation,
			Query:  symbols.FQNOf(query),
			Target: name,
			Span:   expression.Span(),
		}
	}
	targetName := symbols.FQNOf(target)
	if operation, ok := builtins[targetName]; ok {
		args, err := c.compileBuiltinArguments(query, owner, params, expression, operation, dependency)
		if err != nil {
			return Expression{}, err
		}
		return Expression{
			operation: operation.operation,
			target:    targetName,
			arguments: args,
			origin:    provenance.Node(owner.DocName, expression),
		}, nil
	}
	if !IsQueryDefinition(c.index, c.model, target) {
		return Expression{}, &Error{
			Kind:   ErrorUnknownInvocation,
			Query:  symbols.FQNOf(query),
			Target: targetName,
			Span:   expression.Span(),
		}
	}
	if len(expression.Args) > 0 {
		return Expression{}, &Error{
			Kind:   ErrorPositionalQueryArgs,
			Query:  symbols.FQNOf(query),
			Target: targetName,
			Span:   expression.Span(),
		}
	}

	targetParams, _, err := c.signature(target)
	if err != nil {
		return Expression{}, err
	}
	args, err := c.compileNamedArguments(query, owner, params, targetName, targetParams, expression.NamedArgs, dependency)
	if err != nil {
		return Expression{}, err
	}
	dependency(targetName)
	if err := c.compileDefinition(target); err != nil {
		if planning, ok := err.(*Error); ok && planning.Kind == ErrorCompositionCycle {
			planning.Span = expression.Span()
		}
		return Expression{}, err
	}
	return Expression{
		operation: OperationInvoke,
		target:    targetName,
		arguments: args,
		origin:    provenance.Node(owner.DocName, expression),
	}, nil
}

func (c *compiler) compileBuiltinArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	expression *ast.InvocationExpr,
	operation builtin,
	dependency func(string),
) ([]Argument, error) {
	if len(expression.Args) > 0 {
		if len(expression.Args) != len(operation.parameters) {
			return nil, &Error{
				Kind:   ErrorArgumentCount,
				Query:  symbols.FQNOf(query),
				Target: qualifiedName(expression.Type),
				Span:   expression.Span(),
			}
		}
		args := make([]Argument, 0, len(expression.Args))
		for i, node := range expression.Args {
			value, err := c.compileExpression(query, owner, params, node, dependency)
			if err != nil {
				return nil, err
			}
			args = append(args, Argument{Name: operation.parameters[i], Value: value})
		}
		return args, nil
	}

	spec := make([]Parameter, 0, len(operation.parameters))
	for _, name := range operation.parameters {
		spec = append(spec, Parameter{Name: name, Multiplicity: Multiplicity{Lower: 1, Upper: 1, Known: true}})
	}
	return c.compileNamedArguments(
		query,
		owner,
		params,
		qualifiedName(expression.Type),
		spec,
		expression.NamedArgs,
		dependency,
	)
}

func (c *compiler) compileNamedArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	callerParams []Parameter,
	target string,
	targetParams []Parameter,
	named []ast.NamedArg,
	dependency func(string),
) ([]Argument, error) {
	bound := make(map[string]ast.Node, len(named))
	known := make(map[string]Parameter, len(targetParams))
	for _, param := range targetParams {
		known[param.Name] = param
	}
	for _, arg := range named {
		name := qualifiedName(arg.Name)
		if _, exists := bound[name]; exists {
			return nil, &Error{
				Kind:      ErrorDuplicateArgument,
				Query:     symbols.FQNOf(query),
				Target:    target,
				Parameter: name,
				Span:      arg.Value.Span(),
			}
		}
		if _, ok := known[name]; !ok {
			return nil, &Error{
				Kind:      ErrorUnknownArgument,
				Query:     symbols.FQNOf(query),
				Target:    target,
				Parameter: name,
				Span:      arg.Value.Span(),
			}
		}
		bound[name] = arg.Value
	}

	args := make([]Argument, 0, len(bound))
	for _, param := range targetParams {
		node, ok := bound[param.Name]
		if !ok {
			if !param.HasDefault {
				return nil, &Error{
					Kind:      ErrorMissingArgument,
					Query:     symbols.FQNOf(query),
					Target:    target,
					Parameter: param.Name,
					Span:      symbolSpan(query),
				}
			}
			continue
		}
		value, err := c.compileExpression(query, owner, callerParams, node, dependency)
		if err != nil {
			return nil, err
		}
		args = append(args, Argument{Name: param.Name, Named: true, Value: value})
	}
	return args, nil
}

func (c *compiler) cycleError(target *symbols.Symbol) error {
	start := 0
	for i, sym := range c.stack {
		if sym == target {
			start = i
			break
		}
	}
	path := make([]string, 0, len(c.stack)-start+1)
	for _, sym := range c.stack[start:] {
		path = append(path, symbols.FQNOf(sym))
	}
	path = append(path, symbols.FQNOf(target))
	return &Error{
		Kind:  ErrorCompositionCycle,
		Query: symbols.FQNOf(target),
		Path:  path,
		Span:  target.DeclSpan,
	}
}

func literalExpression(query *symbols.Symbol, node ast.Node, kind LiteralKind, value string) Expression {
	return Expression{
		operation: OperationLiteral,
		literal:   kind,
		value:     value,
		origin:    provenance.Node(query.DocName, node),
	}
}

func qualifiedName(name *ast.QualifiedName) string {
	if name == nil {
		return ""
	}
	out := ""
	for i, part := range name.Parts {
		if i > 0 {
			out += "::"
		}
		out += part.Text
	}
	return out
}

func symbolSpan(sym *symbols.Symbol) source.Span {
	if sym == nil {
		return source.Span{}
	}
	return sym.DeclSpan
}
