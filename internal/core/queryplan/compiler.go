package queryplan

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const queryBaseFQN = "DocumentQueries::Query"

type builtin struct {
	operation Operation
}

var builtins = map[string]builtin{
	"DocumentQueries::OwnedElements":   {OperationOwnedElements},
	"DocumentQueries::Descendants":     {OperationDescendants},
	"DocumentQueries::Ancestors":       {OperationAncestors},
	"DocumentQueries::RelatedElements": {OperationRelatedElements},
	"DocumentQueries::WhereType":       {OperationWhereType},
	"DocumentQueries::WhereMetadata":   {OperationWhereMetadata},
	"DocumentQueries::WhereName":       {OperationWhereName},
	"DocumentQueries::WhereFeature":    {OperationWhereFeature},
	"DocumentQueries::OrderBy":         {OperationOrderBy},
	"DocumentQueries::Project":         {OperationProject},
}

type typedExpression struct {
	expression   Expression
	types        []*symbols.Symbol
	multiplicity Multiplicity
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
		return nil, &Error{Kind: ErrorNotQueryDefinition, Query: name, Origin: provenance.Symbol(entry)}
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
			Kind:   ErrorMissingResultParameter,
			Query:  symbols.FQNOf(sym),
			Origin: provenance.Symbol(sym),
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
		expression:   expression.expression,
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
				Origin:    provenance.Symbol(item.Symbol),
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
	return symbols.FQNOf(c.parameterTypeSymbol(sym))
}

func (c *compiler) parameterTypeSymbol(sym *symbols.Symbol) *symbols.Symbol {
	for _, candidate := range c.parameterLineage(sym) {
		for _, relationship := range semantics.RelationshipsOf(candidate) {
			if relationship == nil || relationship.Kind != ast.RelTyping || relationship.Target == nil {
				continue
			}
			target := relationship.Target
			if reference, ok := target.(*ast.FeatureReference); ok {
				target = reference.Name
			}
			name, ok := target.(*ast.QualifiedName)
			if !ok {
				continue
			}
			if resolved, ok := c.resolver.ResolveQualified(candidate.OwnerScope, name); ok {
				if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
					return canonical
				}
			}
		}
	}
	return nil
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
			Kind:   ErrorMissingResult,
			Query:  symbols.FQNOf(sym),
			Origin: provenance.Symbol(sym),
		}
	case 1:
		return results[0], nil
	default:
		return effectiveResult{}, &Error{
			Kind:   ErrorConflictingResult,
			Query:  symbols.FQNOf(sym),
			Origin: provenance.Symbol(sym),
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
		return effectiveResult{}, false, &Error{
			Kind:   ErrorUnsupportedResult,
			Query:  name,
			Origin: provenance.Symbol(sym),
		}
	}

	var expression ast.Node
	for _, binding := range lower.ToBindings(sym.Decl, sym.Scope) {
		for i := range binding.Ends {
			if binding.Ends[i].Path != "result" {
				continue
			}
			if expression != nil {
				return effectiveResult{}, false, &Error{
					Kind:   ErrorUnsupportedResult,
					Query:  name,
					Origin: provenance.Node(sym.DocName, binding.Decl),
				}
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
) (typedExpression, error) {
	switch expression := node.(type) {
	case *ast.FeatureReference:
		return c.compileParameterReference(query, owner, expression)
	case *ast.InvocationExpr:
		return c.compileInvocation(query, owner, params, expression, dependency)
	case *ast.SequenceExpr:
		args := make([]Argument, 0, len(expression.Elements))
		var types []*symbols.Symbol
		multiplicity := Multiplicity{Known: true}
		for _, element := range expression.Elements {
			value, err := c.compileExpression(query, owner, params, element, dependency)
			if err != nil {
				return typedExpression{}, err
			}
			args = append(args, Argument{Value: value.expression})
			types = append(types, value.types...)
			multiplicity = sumMultiplicity(multiplicity, value.multiplicity)
		}
		return typedExpression{
			expression: Expression{
				operation: OperationSequence,
				arguments: args,
				origin:    provenance.Node(owner.DocName, node),
			},
			types:        types,
			multiplicity: multiplicity,
		}, nil
	case *ast.LiteralString:
		return c.literalExpression(owner, node, LiteralString, expression.Value, "ScalarValues::String"), nil
	case *ast.LiteralInteger:
		return c.literalExpression(owner, node, LiteralInteger, expression.Value, "ScalarValues::Integer"), nil
	case *ast.LiteralReal:
		return c.literalExpression(owner, node, LiteralReal, expression.Value, "ScalarValues::Real"), nil
	case *ast.LiteralBool:
		return c.literalExpression(
			owner,
			node,
			LiteralBoolean,
			strconv.FormatBool(expression.Value),
			"ScalarValues::Boolean",
		), nil
	case *ast.LiteralInfinity:
		return c.literalExpression(owner, node, LiteralInfinity, "*", ""), nil
	case *ast.NullExpr:
		result := c.literalExpression(owner, node, LiteralNull, "null", "")
		result.multiplicity = Multiplicity{Known: true}
		return result, nil
	default:
		return typedExpression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Origin: provenance.Node(owner.DocName, node),
		}
	}
}

func (c *compiler) compileParameterReference(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	expression *ast.FeatureReference,
) (typedExpression, error) {
	target, ok := c.resolver.ResolveQualified(owner.Scope, expression.Name)
	if ok {
		for _, param := range c.model.BehaviorParametersOf(query) {
			if !param.IsResult && c.parameterIncludes(param.Symbol, target) {
				return typedExpression{
					expression: Expression{
						operation: OperationParameter,
						target:    param.Symbol.Name,
						origin:    provenance.Node(owner.DocName, expression),
					},
					types:        symbolSlice(c.parameterTypeSymbol(param.Symbol)),
					multiplicity: c.parameterMultiplicity(param.Symbol),
				}, nil
			}
		}
	}
	name := qualifiedName(expression.Name)
	return typedExpression{}, &Error{
		Kind:      ErrorUnknownParameter,
		Query:     symbols.FQNOf(query),
		Parameter: name,
		Origin:    provenance.Node(owner.DocName, expression),
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
) (typedExpression, error) {
	name := qualifiedName(expression.Type)
	if expression.Operand != nil {
		return typedExpression{}, &Error{
			Kind:   ErrorUnsupportedExpression,
			Query:  symbols.FQNOf(query),
			Target: name,
			Origin: provenance.Node(owner.DocName, expression),
		}
	}
	target, ok := c.resolver.ResolveQualified(owner.Scope, expression.Type)
	if !ok {
		return typedExpression{}, &Error{
			Kind:   ErrorUnknownInvocation,
			Query:  symbols.FQNOf(query),
			Target: name,
			Origin: provenance.Node(owner.DocName, expression),
		}
	}
	targetName := symbols.FQNOf(target)
	if operation, ok := builtins[targetName]; ok {
		targetParams, targetResult, err := c.signature(target)
		if err != nil {
			return typedExpression{}, err
		}
		args, err := c.compileBuiltinArguments(
			query,
			owner,
			params,
			expression,
			targetName,
			targetParams,
			dependency,
		)
		if err != nil {
			return typedExpression{}, err
		}
		return typedExpression{
			expression: Expression{
				operation: operation.operation,
				target:    targetName,
				arguments: args,
				origin:    provenance.Node(owner.DocName, expression),
			},
			types:        symbolSlice(c.typeSymbol(targetResult.Type)),
			multiplicity: targetResult.Multiplicity,
		}, nil
	}
	if !IsQueryDefinition(c.index, c.model, target) {
		return typedExpression{}, &Error{
			Kind:   ErrorUnknownInvocation,
			Query:  symbols.FQNOf(query),
			Target: targetName,
			Origin: provenance.Node(owner.DocName, expression),
		}
	}
	if len(expression.Args) > 0 {
		return typedExpression{}, &Error{
			Kind:   ErrorPositionalQueryArgs,
			Query:  symbols.FQNOf(query),
			Target: targetName,
			Origin: provenance.Node(owner.DocName, expression),
		}
	}

	targetParams, targetResult, err := c.signature(target)
	if err != nil {
		return typedExpression{}, err
	}
	args, err := c.compileNamedArguments(
		query,
		owner,
		params,
		targetName,
		targetParams,
		expression,
		dependency,
	)
	if err != nil {
		return typedExpression{}, err
	}
	dependency(targetName)
	if err := c.compileDefinition(target); err != nil {
		if planning, ok := err.(*Error); ok && planning.Kind == ErrorCompositionCycle {
			planning.Origin = provenance.Node(owner.DocName, expression)
		}
		return typedExpression{}, err
	}
	return typedExpression{
		expression: Expression{
			operation: OperationInvoke,
			target:    targetName,
			arguments: args,
			origin:    provenance.Node(owner.DocName, expression),
		},
		types:        symbolSlice(c.typeSymbol(targetResult.Type)),
		multiplicity: targetResult.Multiplicity,
	}, nil
}

func (c *compiler) compileBuiltinArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	params []Parameter,
	expression *ast.InvocationExpr,
	target string,
	targetParams []Parameter,
	dependency func(string),
) ([]Argument, error) {
	if len(expression.Args) > 0 {
		if len(expression.Args) != len(targetParams) {
			return nil, &Error{
				Kind:   ErrorArgumentCount,
				Query:  symbols.FQNOf(query),
				Target: qualifiedName(expression.Type),
				Origin: provenance.Node(owner.DocName, expression),
			}
		}
		args := make([]Argument, 0, len(expression.Args))
		for i, node := range expression.Args {
			value, err := c.compileExpression(query, owner, params, node, dependency)
			if err != nil {
				return nil, err
			}
			if err := c.validateArgument(
				query,
				target,
				targetParams[i],
				value,
				provenance.Node(owner.DocName, node),
			); err != nil {
				return nil, err
			}
			args = append(args, Argument{Name: targetParams[i].Name, Value: value.expression})
		}
		return args, nil
	}

	return c.compileNamedArguments(
		query,
		owner,
		params,
		qualifiedName(expression.Type),
		targetParams,
		expression,
		dependency,
	)
}

func (c *compiler) compileNamedArguments(
	query *symbols.Symbol,
	owner *symbols.Symbol,
	callerParams []Parameter,
	target string,
	targetParams []Parameter,
	expression *ast.InvocationExpr,
	dependency func(string),
) ([]Argument, error) {
	named := expression.NamedArgs
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
				Origin:    provenance.Node(owner.DocName, arg.Value),
			}
		}
		if _, ok := known[name]; !ok {
			return nil, &Error{
				Kind:      ErrorUnknownArgument,
				Query:     symbols.FQNOf(query),
				Target:    target,
				Parameter: name,
				Origin:    provenance.Node(owner.DocName, arg.Value),
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
					Origin:    provenance.Node(owner.DocName, expression),
				}
			}
			continue
		}
		value, err := c.compileExpression(query, owner, callerParams, node, dependency)
		if err != nil {
			return nil, err
		}
		if err := c.validateArgument(
			query,
			target,
			param,
			value,
			provenance.Node(owner.DocName, node),
		); err != nil {
			return nil, err
		}
		args = append(args, Argument{Name: param.Name, Named: true, Value: value.expression})
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
		Kind:   ErrorCompositionCycle,
		Query:  symbols.FQNOf(target),
		Path:   path,
		Origin: provenance.Symbol(target),
	}
}

func (c *compiler) literalExpression(
	query *symbols.Symbol,
	node ast.Node,
	kind LiteralKind,
	value string,
	typeName string,
) typedExpression {
	return typedExpression{
		expression: Expression{
			operation: OperationLiteral,
			literal:   kind,
			value:     value,
			origin:    provenance.Node(query.DocName, node),
		},
		types:        symbolSlice(c.typeSymbol(typeName)),
		multiplicity: Multiplicity{Lower: 1, Upper: 1, Known: true},
	}
}

func (c *compiler) validateArgument(
	query *symbols.Symbol,
	target string,
	param Parameter,
	value typedExpression,
	origin provenance.Origin,
) error {
	if want := c.typeSymbol(param.Type); want != nil && !c.argumentTypesConform(value.types, want) {
		return &Error{
			Kind:      ErrorArgumentType,
			Query:     symbols.FQNOf(query),
			Target:    target,
			Parameter: param.Name,
			Expected:  param.Type,
			Actual:    typeNames(value.types),
			Origin:    origin,
		}
	}
	if !multiplicityConforms(value.multiplicity, param.Multiplicity) {
		return &Error{
			Kind:      ErrorArgumentMultiplicity,
			Query:     symbols.FQNOf(query),
			Target:    target,
			Parameter: param.Name,
			Expected:  multiplicityString(param.Multiplicity),
			Actual:    multiplicityString(value.multiplicity),
			Origin:    origin,
		}
	}
	return nil
}

func (c *compiler) argumentTypesConform(actual []*symbols.Symbol, expected *symbols.Symbol) bool {
	for _, candidate := range actual {
		if candidate == nil || candidate == expected {
			continue
		}
		got, want := c.model.PrimTypeOf(candidate), c.model.PrimTypeOf(expected)
		if got != semantics.PrimUnknown || want != semantics.PrimUnknown {
			if got != semantics.PrimUnknown && want != semantics.PrimUnknown &&
				semantics.PrimConforms(got, want) {
				continue
			}
			return false
		}
		if c.model.Conforms(candidate, expected) {
			continue
		}
		return false
	}
	return true
}

func (c *compiler) typeSymbol(name string) *symbols.Symbol {
	if name == "" {
		return nil
	}
	matches := symbols.PreferDeclared(c.index.LookupQualified(name))
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

func symbolSlice(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	return []*symbols.Symbol{sym}
}

func typeNames(types []*symbols.Symbol) string {
	if len(types) == 0 {
		return "unknown"
	}
	seen := make(map[string]bool, len(types))
	names := make([]string, 0, len(types))
	for _, sym := range types {
		name := symbols.FQNOf(sym)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return "unknown"
	}
	return strings.Join(names, " or ")
}

func sumMultiplicity(left, right Multiplicity) Multiplicity {
	if !left.Known || !right.Known {
		return Multiplicity{}
	}
	sum := Multiplicity{
		Lower: left.Lower + right.Lower,
		Known: true,
	}
	if left.UpperInfinite || right.UpperInfinite {
		sum.UpperInfinite = true
		return sum
	}
	sum.Upper = left.Upper + right.Upper
	return sum
}

func multiplicityConforms(actual, expected Multiplicity) bool {
	if !actual.Known || !expected.Known || actual.Lower < expected.Lower {
		return !actual.Known || !expected.Known
	}
	if expected.UpperInfinite {
		return true
	}
	return !actual.UpperInfinite && actual.Upper <= expected.Upper
}

func multiplicityString(multiplicity Multiplicity) string {
	if !multiplicity.Known {
		return "unknown"
	}
	upper := strconv.FormatInt(multiplicity.Upper, 10)
	if multiplicity.UpperInfinite {
		upper = "*"
	}
	return "[" + strconv.FormatInt(multiplicity.Lower, 10) + ".." + upper + "]"
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
