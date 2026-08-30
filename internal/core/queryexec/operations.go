package queryexec

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (e *executor) evaluateOwned(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		if sym.Scope == nil {
			continue
		}
		for _, member := range sym.Scope.AllMembers() {
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			appendElement(&result, member, seen)
		}
	}
	return result, nil
}

func (e *executor) evaluateDescendants(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	type pending struct {
		sym   *symbols.Symbol
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		key := symbols.KeyOf(sym)
		seen[key] = struct{}{}
		queue = append(queue, pending{sym: sym})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth || next.sym.Scope == nil {
			continue
		}
		for _, member := range next.sym.Scope.AllMembers() {
			if !e.consumeVisit() {
				return sequence{}, e.budgetError(expression)
			}
			key := symbols.KeyOf(member)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			result.values = append(result.values, ElementValue(member))
			queue = append(queue, pending{sym: member, depth: next.depth + 1})
		}
	}
	return result, nil
}

func (e *executor) evaluateAncestors(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	maxDepth, err := e.integerArgument(expression, "maxDepth")
	if err != nil {
		return sequence{}, err
	}
	type pending struct {
		sym   *symbols.Symbol
		depth int64
	}
	queue := make([]pending, 0, len(source.values))
	seen := make(map[symbols.ElementKey]struct{})
	for _, value := range source.values {
		sym, _ := value.Element()
		seen[symbols.KeyOf(sym)] = struct{}{}
		queue = append(queue, pending{sym: sym})
	}
	var result sequence
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next.depth >= maxDepth || next.sym.OwnerScope == nil {
			continue
		}
		owner := next.sym.OwnerScope.Owner()
		if owner == nil {
			continue
		}
		if !e.consumeVisit() {
			return sequence{}, e.budgetError(expression)
		}
		key := symbols.KeyOf(owner)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result.values = append(result.values, ElementValue(owner))
		queue = append(queue, pending{sym: owner, depth: next.depth + 1})
	}
	return result, nil
}

func (e *executor) evaluateWhereType(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	typeName, err := e.stringArgument(expression, "type")
	if err != nil {
		return sequence{}, err
	}
	target := e.resolveClassification(typeName)
	var result sequence
	for i, value := range source.values {
		sym, _ := value.Element()
		matches := query.MetamodelTypeNameOf(sym) == typeName
		if target != nil {
			matches = matches || symbols.SameElement(sym, target) || e.context.Model.Conforms(sym, target)
		}
		if matches {
			appendSelected(&result, source, i)
		}
	}
	if target == nil && len(result.values) == 0 && !knownMetamodelType(typeName) {
		return sequence{}, &Error{
			Kind:      ErrorUnknownClassification,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    typeName,
			Origin:    expression.Origin(),
		}
	}
	return result, nil
}

func (e *executor) evaluateWhereMetadata(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	name, err := e.stringArgument(expression, "metadata")
	if err != nil {
		return sequence{}, err
	}
	target := e.resolveClassification(name)
	if target == nil {
		return sequence{}, &Error{
			Kind:      ErrorUnknownClassification,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    name,
			Origin:    expression.Origin(),
		}
	}
	var result sequence
	for i, value := range source.values {
		sym, _ := value.Element()
		for _, annotation := range e.context.Model.AnnotationFactsOf(sym) {
			types := e.context.Index.LookupQualified(annotation.TypeFQN)
			matches := false
			for _, actual := range types {
				if symbols.SameElement(actual, target) || e.context.Model.Conforms(actual, target) {
					matches = true
					break
				}
			}
			if matches {
				appendSelected(&result, source, i)
				break
			}
		}
	}
	return result, nil
}

func (e *executor) evaluateWhereName(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	operator, err := e.stringArgument(expression, "operator")
	if err != nil {
		return sequence{}, err
	}
	expected, err := e.stringArgument(expression, "value")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for i, value := range source.values {
		sym, _ := value.Element()
		names, _ := e.reader.Values(sym, query.PropertyName)
		if len(names) == 0 {
			continue
		}
		match, compareErr := compareText(names[0], operator, expected)
		if compareErr != nil {
			if compareErr != errComparison {
				return sequence{}, e.invalidArgument(expression, "value", expected)
			}
			return sequence{}, e.operatorError(expression, operator)
		}
		if match {
			appendSelected(&result, source, i)
		}
	}
	return result, nil
}

func (e *executor) evaluateWhereFeature(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	property, err := e.stringArgument(expression, "feature")
	if err != nil {
		return sequence{}, err
	}
	operator, err := e.stringArgument(expression, "operator")
	if err != nil {
		return sequence{}, err
	}
	expected, err := e.stringArgument(expression, "value")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	known := false
	for i, value := range source.values {
		sym, _ := value.Element()
		values, present, valueErr := e.propertyValues(sym, property)
		if valueErr != nil {
			return sequence{}, e.featureError(expression, property)
		}
		known = known || present
		for _, actual := range values {
			match, compareErr := compareValue(actual, operator, expected)
			if compareErr != nil {
				if compareErr != errComparison {
					return sequence{}, e.invalidArgument(expression, "value", expected)
				}
				return sequence{}, e.operatorError(expression, operator)
			}
			if match {
				appendSelected(&result, source, i)
				break
			}
		}
	}
	if !known {
		return sequence{}, e.unknownProperty(expression, property)
	}
	return result, nil
}

func (e *executor) evaluateOrderBy(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	property, err := e.stringArgument(expression, "property")
	if err != nil {
		return sequence{}, err
	}
	direction, err := e.stringArgument(expression, "direction")
	if err != nil {
		return sequence{}, err
	}
	missing, err := e.stringArgument(expression, "missing")
	if err != nil {
		return sequence{}, err
	}
	multiple, err := e.stringArgument(expression, "multiple")
	if err != nil {
		return sequence{}, err
	}
	if direction != "ascending" && direction != "descending" {
		return sequence{}, e.operatorError(expression, direction)
	}
	if missing != "first" && missing != "last" && missing != "error" {
		return sequence{}, e.operatorError(expression, missing)
	}
	if multiple != "first" && multiple != "last" && multiple != "error" {
		return sequence{}, e.operatorError(expression, multiple)
	}
	type sortable struct {
		value Value
		cells []Cell
		key   Value
		set   bool
	}
	items := make([]sortable, len(source.values))
	known := false
	var orderedKind ValueKind
	for i, value := range source.values {
		sym, _ := value.Element()
		values, present, valueErr := e.propertyValues(sym, property)
		if valueErr != nil {
			return sequence{}, e.featureError(expression, property)
		}
		known = known || present
		items[i].value = value
		if i < len(source.cells) {
			items[i].cells = cloneCells(source.cells[i])
		}
		switch len(values) {
		case 0:
			if missing == "error" {
				return sequence{}, e.featureError(expression, property)
			}
		case 1:
			items[i].key = values[0]
			items[i].set = true
		default:
			if multiple == "error" {
				return sequence{}, e.featureError(expression, property)
			}
			index := 0
			if multiple == "last" {
				index = len(values) - 1
			}
			items[i].key = values[index]
			items[i].set = true
		}
		if items[i].set {
			if orderedKind == "" {
				orderedKind = items[i].key.Kind()
			} else if !orderedKindsCompatible(orderedKind, items[i].key.Kind()) {
				return sequence{}, &Error{
					Kind:      ErrorInvalidOrder,
					Query:     e.definition.Name(),
					Operation: expression.Operation(),
					Property:  property,
					Origin:    expression.Origin(),
				}
			}
		}
	}
	if !known {
		return sequence{}, e.unknownProperty(expression, property)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.set != right.set {
			if missing == "first" {
				return !left.set
			}
			return left.set
		}
		if !left.set {
			return false
		}
		comparison := compareOrdered(left.key, right.key)
		if direction == "descending" {
			comparison = -comparison
		}
		return comparison < 0
	})
	result := sequence{columns: append([]Column(nil), source.columns...)}
	for _, item := range items {
		result.values = append(result.values, item.value)
		result.cells = append(result.cells, item.cells)
	}
	return result, nil
}

func (e *executor) evaluateProject(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	properties, err := e.stringsArgument(expression, "properties")
	if err != nil {
		return sequence{}, err
	}
	if len(properties) == 0 {
		return sequence{}, e.invalidArgument(expression, "properties", "empty")
	}
	result := sequence{
		values:  append([]Value(nil), source.values...),
		columns: make([]Column, len(properties)),
		cells:   make([][]Cell, len(source.values)),
	}
	known := make([]bool, len(properties))
	for i, property := range properties {
		result.columns[i] = Column{name: property, origin: expression.Origin()}
	}
	for row, value := range source.values {
		sym, _ := value.Element()
		result.cells[row] = make([]Cell, len(properties))
		for column, property := range properties {
			values, present, valueErr := e.propertyValues(sym, property)
			if valueErr != nil {
				return sequence{}, e.featureError(expression, property)
			}
			known[column] = known[column] || present
			result.cells[row][column] = Cell{
				values: values,
				origin: value.Origin(),
			}
		}
	}
	for i, property := range properties {
		if !known[i] && len(source.values) > 0 {
			return sequence{}, e.unknownProperty(expression, property)
		}
	}
	return result, nil
}

func (e *executor) propertyValues(sym *symbols.Symbol, property string) ([]Value, bool, error) {
	if isQueryableProperty(property) {
		values, present := e.reader.Values(sym, property)
		if !present {
			return nil, true, nil
		}
		result := make([]Value, 0, len(values))
		for _, value := range values {
			result = append(result, typedPropertyValue(property, value, sym))
		}
		return result, true, nil
	}
	values, present := e.context.Model.ConstantFeatureValues(sym, property)
	if !present {
		return nil, false, nil
	}
	result := make([]Value, 0, len(values))
	for _, value := range values {
		converted, ok := filterValue(value, sym)
		if !ok {
			if value.Kind == symbols.FilterValueEmpty {
				continue
			}
			return nil, true, e.featureError(queryplan.Expression{}, property)
		}
		result = append(result, converted)
	}
	return result, true, nil
}

func isQueryableProperty(property string) bool {
	switch property {
	case query.PropertyID,
		query.PropertyType,
		query.PropertyName,
		query.PropertyDeclaredName,
		query.PropertyQualifiedName,
		query.PropertyOwner,
		query.PropertyElementType,
		query.PropertyIsAbstract,
		query.PropertyMultiplicityLower,
		query.PropertyMultiplicityUpper:
		return true
	default:
		return false
	}
}

func typedPropertyValue(property, value string, sym *symbols.Symbol) Value {
	var result Value
	switch property {
	case query.PropertyIsAbstract:
		boolean, _ := strconv.ParseBool(value)
		result = BooleanValue(boolean)
	case query.PropertyMultiplicityLower, query.PropertyMultiplicityUpper:
		if value == "*" {
			result = Value{kind: ValueInfinity}
		} else {
			integer, _ := strconv.ParseInt(value, 10, 64)
			result = IntegerValue(integer)
		}
	default:
		result = StringValue(value)
	}
	return valueAt(result, ElementValue(sym).Origin())
}

func filterValue(value symbols.FilterValue, sym *symbols.Symbol) (Value, bool) {
	var result Value
	switch value.Kind {
	case symbols.FilterValueBool:
		result = BooleanValue(value.Bool)
	case symbols.FilterValueInt:
		result = IntegerValue(value.Int)
	case symbols.FilterValueReal:
		result = RealValue(value.Real)
	case symbols.FilterValueString:
		result = StringValue(value.Str)
	case symbols.FilterValueRef:
		result = StringValue(value.RefFQN)
	default:
		return Value{}, false
	}
	return valueAt(result, ElementValue(sym).Origin()), true
}

func compareText(actual, operator, expected string) (bool, error) {
	switch operator {
	case "=", "==":
		return actual == expected, nil
	case "!=", "<>":
		return actual != expected, nil
	case "contains":
		return strings.Contains(actual, expected), nil
	case "startsWith", "starts-with":
		return strings.HasPrefix(actual, expected), nil
	case "endsWith", "ends-with":
		return strings.HasSuffix(actual, expected), nil
	case "matches":
		expression, err := regexp.Compile(expected)
		if err != nil {
			return false, err
		}
		return expression.MatchString(actual), nil
	default:
		return false, errComparison
	}
}

var errComparison = &comparisonError{}

type comparisonError struct{}

func (*comparisonError) Error() string { return "unsupported comparison" }

func compareValue(actual Value, operator, expected string) (bool, error) {
	switch actual.Kind() {
	case ValueString:
		value, _ := actual.String()
		return compareText(value, operator, expected)
	case ValueBoolean:
		value, _ := actual.Boolean()
		want, err := strconv.ParseBool(expected)
		if err != nil {
			return false, err
		}
		switch operator {
		case "=", "==":
			return value == want, nil
		case "!=", "<>":
			return value != want, nil
		default:
			return false, errComparison
		}
	case ValueInteger:
		value, _ := actual.Integer()
		want, err := strconv.ParseInt(strings.ReplaceAll(expected, "_", ""), 10, 64)
		if err != nil {
			return false, err
		}
		return compareNumber(float64(value), operator, float64(want))
	case ValueReal:
		value, _ := actual.Real()
		want, err := strconv.ParseFloat(strings.ReplaceAll(expected, "_", ""), 64)
		if err != nil {
			return false, err
		}
		return compareNumber(value, operator, want)
	case ValueInfinity:
		switch operator {
		case "=", "==":
			return expected == "*", nil
		case "!=", "<>":
			return expected != "*", nil
		default:
			return false, errComparison
		}
	default:
		return false, errComparison
	}
}

func compareNumber(actual float64, operator string, expected float64) (bool, error) {
	switch operator {
	case "=", "==":
		return actual == expected, nil
	case "!=", "<>":
		return actual != expected, nil
	case "<":
		return actual < expected, nil
	case "<=":
		return actual <= expected, nil
	case ">":
		return actual > expected, nil
	case ">=":
		return actual >= expected, nil
	default:
		return false, errComparison
	}
}

func compareOrdered(left, right Value) int {
	if left.Kind() == ValueInteger && right.Kind() == ValueReal {
		l, _ := left.Integer()
		r, _ := right.Real()
		return compareFloat(float64(l), r)
	}
	if left.Kind() == ValueReal && right.Kind() == ValueInteger {
		l, _ := left.Real()
		r, _ := right.Integer()
		return compareFloat(l, float64(r))
	}
	if left.Kind() != right.Kind() {
		return strings.Compare(string(left.Kind()), string(right.Kind()))
	}
	switch left.Kind() {
	case ValueString:
		l, _ := left.String()
		r, _ := right.String()
		return strings.Compare(l, r)
	case ValueInteger:
		l, _ := left.Integer()
		r, _ := right.Integer()
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	case ValueReal:
		l, _ := left.Real()
		r, _ := right.Real()
		return compareFloat(l, r)
	case ValueBoolean:
		l, _ := left.Boolean()
		r, _ := right.Boolean()
		if !l && r {
			return -1
		}
		if l && !r {
			return 1
		}
	}
	return 0
}

func compareFloat(left, right float64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func orderedKindsCompatible(left, right ValueKind) bool {
	if left == right {
		return true
	}
	return (left == ValueInteger || left == ValueReal) &&
		(right == ValueInteger || right == ValueReal)
}

func (e *executor) resolveClassification(name string) *symbols.Symbol {
	if matches := e.context.Index.LookupQualified(name); len(matches) == 1 {
		return matches[0]
	}
	var match *symbols.Symbol
	for _, fqn := range e.context.Index.FQNs() {
		if fqn != name && !strings.HasSuffix(fqn, "::"+name) {
			continue
		}
		candidates := e.context.Index.LookupQualified(fqn)
		for _, candidate := range candidates {
			if match != nil && !symbols.SameElement(match, candidate) {
				return nil
			}
			match = candidate
		}
	}
	return match
}

func knownMetamodelType(name string) bool {
	for kind := symbols.SymbolUnknown; kind <= symbols.SymbolRelationship; kind++ {
		if query.MetamodelTypeName(kind) == name {
			return true
		}
	}
	return false
}

func appendSelected(result *sequence, source sequence, index int) {
	result.values = append(result.values, source.values[index])
	if len(result.columns) == 0 && len(source.columns) > 0 {
		result.columns = append([]Column(nil), source.columns...)
	}
	if index < len(source.cells) {
		result.cells = append(result.cells, cloneCells(source.cells[index]))
	}
}

func appendElement(result *sequence, sym *symbols.Symbol, seen map[symbols.ElementKey]struct{}) {
	key := symbols.KeyOf(sym)
	if _, duplicate := seen[key]; duplicate {
		return
	}
	seen[key] = struct{}{}
	result.values = append(result.values, ElementValue(sym))
}

func (e *executor) consumeVisit() bool {
	if e.remaining <= 0 {
		return false
	}
	e.remaining--
	return true
}

func (e *executor) budgetError(expression queryplan.Expression) error {
	return &Error{
		Kind:      ErrorVisitBudget,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Origin:    expression.Origin(),
	}
}

func (e *executor) operatorError(expression queryplan.Expression, operator string) error {
	return &Error{
		Kind:      ErrorInvalidOperator,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Actual:    operator,
		Origin:    expression.Origin(),
	}
}

func (e *executor) unknownProperty(expression queryplan.Expression, property string) error {
	return &Error{
		Kind:      ErrorUnknownProperty,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Property:  property,
		Origin:    expression.Origin(),
	}
}

func (e *executor) featureError(expression queryplan.Expression, property string) error {
	return &Error{
		Kind:      ErrorUnevaluableFeature,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Property:  property,
		Origin:    expression.Origin(),
	}
}
