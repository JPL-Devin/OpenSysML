package docplan

import (
	"sort"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const (
	documentBaseFQN  = "DocumentQueries::Document"
	sectionBaseFQN   = "DocumentQueries::Section"
	paragraphBaseFQN = "DocumentQueries::Paragraph"
	tableBaseFQN     = "DocumentQueries::Table"
	listBaseFQN      = "DocumentQueries::List"
)

type compiler struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
	document string
	bases    bases
}

type bases struct {
	document  *symbols.Symbol
	section   *symbols.Symbol
	paragraph *symbols.Symbol
	table     *symbols.Symbol
	list      *symbols.Symbol
}

// IsDocumentDefinition reports whether sym specializes DocumentQueries::Document.
func IsDocumentDefinition(index *symbols.Index, model *semantics.Model, sym *symbols.Symbol) bool {
	base := libraryBase(index, documentBaseFQN)
	return base != nil && sym != nil && sym != base &&
		sym.Kind == symbols.SymbolPartDef && model != nil && model.Conforms(sym, base)
}

// QueryTarget resolves the query definition a calc usage is typed by, following
// redefinition lineage; nil when it is not typed by a query definition.
func QueryTarget(index *symbols.Index, model *semantics.Model, resolver *resolve.Resolver, usage *symbols.Symbol) *symbols.Symbol {
	if index == nil || model == nil || resolver == nil || usage == nil || usage.Kind != symbols.SymbolCalcUsage {
		return nil
	}
	c := &compiler{index: index, model: model, resolver: resolver}
	target := c.typingTarget(usage)
	if target == nil || !queryplan.IsQueryDefinition(index, model, target) {
		return nil
	}
	return target
}

// Compile compiles a document definition into an immutable document plan.
func Compile(index *symbols.Index, model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*Plan, error) {
	if index == nil || model == nil || resolver == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	all := bases{
		document:  libraryBase(index, documentBaseFQN),
		section:   libraryBase(index, sectionBaseFQN),
		paragraph: libraryBase(index, paragraphBaseFQN),
		table:     libraryBase(index, tableBaseFQN),
		list:      libraryBase(index, listBaseFQN),
	}
	if all.document == nil || all.section == nil || all.paragraph == nil || all.table == nil || all.list == nil {
		return nil, &Error{Kind: ErrorLibraryUnavailable}
	}
	name := symbols.FQNOf(entry)
	if entry == nil || entry == all.document || entry.Kind != symbols.SymbolPartDef || !model.Conforms(entry, all.document) {
		return nil, &Error{Kind: ErrorNotDocumentDefinition, Document: name, Origin: provenance.Symbol(entry)}
	}
	c := &compiler{index: index, model: model, resolver: resolver, document: name, bases: all}
	title, err := c.requiredText(entry, "title", ErrorMissingTitle)
	if err != nil {
		return nil, err
	}
	content, err := c.compileMembers(entry)
	if err != nil {
		return nil, err
	}
	return &Plan{
		compiled: true,
		name:     name,
		title:    title,
		content:  content,
		origin:   provenance.Symbol(entry),
	}, nil
}

func libraryBase(index *symbols.Index, fqn string) *symbols.Symbol {
	if index == nil {
		return nil
	}
	matches := symbols.PreferDeclared(index.LookupQualified(fqn))
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

// compileMembers compiles the ordered structural members of a document
// definition or a section usage.
func (c *compiler) compileMembers(owner *symbols.Symbol) ([]Content, error) {
	content := make([]Content, 0)
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind == symbols.SymbolPartUsage && c.isContent(member) {
			node, err := c.compileContent(member)
			if err != nil {
				return nil, err
			}
			content = append(content, node)
			continue
		}
		if !c.index.Library(member) {
			if err := c.rejectStructural(owner, member); err != nil {
				return nil, err
			}
		}
	}
	return content, nil
}

// rejectStructural rejects declarations that cannot appear in a structural
// scope, letting attributes, annotations, and other non-content members
// through.
func (c *compiler) rejectStructural(owner *symbols.Symbol, member *symbols.Symbol) error {
	if member.Kind == symbols.SymbolCalcUsage || member.Kind == symbols.SymbolPartUsage {
		return &Error{
			Kind:     ErrorInvalidContent,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return nil
}

// isContent reports whether a part usage conforms to a document content kind.
func (c *compiler) isContent(member *symbols.Symbol) bool {
	return c.model.Conforms(member, c.bases.document) ||
		c.model.Conforms(member, c.bases.section) ||
		c.model.Conforms(member, c.bases.paragraph) ||
		c.model.Conforms(member, c.bases.table) ||
		c.model.Conforms(member, c.bases.list)
}

func (c *compiler) compileContent(member *symbols.Symbol) (Content, error) {
	switch {
	case c.model.Conforms(member, c.bases.document):
		return Content{}, &Error{
			Kind:     ErrorNestedDocument,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	case c.model.Conforms(member, c.bases.section):
		return c.compileSection(member)
	case c.model.Conforms(member, c.bases.paragraph):
		return c.compileParagraph(member)
	case c.model.Conforms(member, c.bases.table):
		return c.compileTable(member)
	case c.model.Conforms(member, c.bases.list):
		return c.compileList(member)
	default:
		return Content{}, &Error{
			Kind:     ErrorInvalidContent,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
}

func (c *compiler) compileSection(member *symbols.Symbol) (Content, error) {
	title, err := c.requiredText(member, "title", ErrorMissingTitle)
	if err != nil {
		return Content{}, err
	}
	children, err := c.compileMembers(member)
	if err != nil {
		return Content{}, err
	}
	return Content{
		kind:     ContentSection,
		name:     c.effectiveName(member),
		title:    title,
		children: children,
		origin:   provenance.Symbol(member),
	}, nil
}

func (c *compiler) compileParagraph(member *symbols.Symbol) (Content, error) {
	text, stated, err := c.optionalText(member, "text")
	if err != nil {
		return Content{}, err
	}
	query, err := c.compileQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	if stated && query != nil {
		return Content{}, &Error{
			Kind:     ErrorConflictingText,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if !stated && query == nil {
		return Content{}, &Error{
			Kind:     ErrorMissingText,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:   ContentParagraph,
		name:   c.effectiveName(member),
		text:   text,
		query:  query,
		origin: provenance.Symbol(member),
	}, nil
}

func (c *compiler) compileTable(member *symbols.Symbol) (Content, error) {
	caption, _, err := c.optionalText(member, "caption")
	if err != nil {
		return Content{}, err
	}
	query, err := c.requiredQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:    ContentTable,
		name:    c.effectiveName(member),
		caption: caption,
		query:   query,
		origin:  provenance.Symbol(member),
	}, nil
}

func (c *compiler) compileList(member *symbols.Symbol) (Content, error) {
	style, stated, err := c.optionalText(member, "style")
	if err != nil {
		return Content{}, err
	}
	listStyle := ListBullet
	if stated {
		listStyle = ListStyle(style)
		if listStyle != ListBullet && listStyle != ListNumber {
			return Content{}, &Error{
				Kind:     ErrorInvalidStyle,
				Document: c.document,
				Content:  c.contentName(member),
				Actual:   style,
				Origin:   provenance.Symbol(member),
			}
		}
	}
	query, err := c.requiredQueryRef(member)
	if err != nil {
		return Content{}, err
	}
	if err := c.rejectNestedContent(member); err != nil {
		return Content{}, err
	}
	return Content{
		kind:   ContentList,
		name:   c.effectiveName(member),
		style:  listStyle,
		query:  query,
		origin: provenance.Symbol(member),
	}, nil
}

// rejectNestedContent rejects content blocks nested inside a content block.
func (c *compiler) rejectNestedContent(owner *symbols.Symbol) error {
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind == symbols.SymbolPartUsage && c.isContent(member) {
			return &Error{
				Kind:     ErrorInvalidContent,
				Document: c.document,
				Content:  c.contentName(member),
				Origin:   provenance.Symbol(member),
			}
		}
	}
	return nil
}

// requiredQueryRef compiles the single query reference a table or list needs.
func (c *compiler) requiredQueryRef(member *symbols.Symbol) (*QueryRef, error) {
	query, err := c.compileQueryRef(member)
	if err != nil {
		return nil, err
	}
	if query == nil {
		return nil, &Error{
			Kind:     ErrorMissingQuery,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return query, nil
}

// compileQueryRef compiles the calc usage inside a content block into a
// planned query reference, or nil when the block declares none.
func (c *compiler) compileQueryRef(owner *symbols.Symbol) (*QueryRef, error) {
	var usage *symbols.Symbol
	for _, member := range c.effectiveMembers(owner) {
		if member.Kind != symbols.SymbolCalcUsage {
			continue
		}
		if usage != nil {
			return nil, &Error{
				Kind:     ErrorConflictingQuery,
				Document: c.document,
				Content:  c.contentName(owner),
				Origin:   provenance.Symbol(member),
			}
		}
		usage = member
	}
	if usage == nil {
		return nil, nil
	}
	target := c.typingTarget(usage)
	if target == nil || !queryplan.IsQueryDefinition(c.index, c.model, target) {
		return nil, &Error{
			Kind:     ErrorUnknownQuery,
			Document: c.document,
			Content:  c.contentName(owner),
			Query:    symbols.FQNOf(target),
			Origin:   provenance.Symbol(usage),
		}
	}
	entry := symbols.FQNOf(target)
	program, err := queryplan.Compile(c.index, c.model, c.resolver, target)
	if err != nil {
		return nil, &Error{
			Kind:     ErrorQueryPlanning,
			Document: c.document,
			Content:  c.contentName(owner),
			Query:    entry,
			Origin:   provenance.Symbol(usage),
			Err:      err,
		}
	}
	bindings, err := c.compileBindings(owner, usage, entry, program)
	if err != nil {
		return nil, err
	}
	return &QueryRef{
		entry:    entry,
		program:  program,
		bindings: bindings,
		origin:   provenance.Symbol(usage),
	}, nil
}

// typingTarget resolves the declared type of a usage, following redefinition
// lineage when the declaration omits an explicit type.
func (c *compiler) typingTarget(sym *symbols.Symbol) *symbols.Symbol {
	return c.typingTargetSeen(sym, make(map[*symbols.Symbol]bool))
}

func (c *compiler) typingTargetSeen(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) *symbols.Symbol {
	if sym == nil || seen[sym] {
		return nil
	}
	seen[sym] = true
	declared := false
	for _, relationship := range semantics.RelationshipsOf(sym) {
		if relationship == nil || relationship.Kind != ast.RelTyping || relationship.Target == nil {
			continue
		}
		declared = true
		target := relationship.Target
		if reference, ok := target.(*ast.FeatureReference); ok {
			target = reference.Name
		}
		name, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if resolved, ok := c.resolver.ResolveQualified(sym.OwnerScope, name); ok {
			if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
				return canonical
			}
		}
	}
	if declared {
		return nil // an explicit type that fails to resolve must not inherit one
	}
	for _, target := range c.model.RedefinedFeatures(sym) {
		if resolved := c.typingTargetSeen(target, seen); resolved != nil {
			return resolved
		}
	}
	return nil
}

// compileBindings compiles the `in` members of a query usage against the
// compiled signature of the entry query.
func (c *compiler) compileBindings(
	content *symbols.Symbol,
	usage *symbols.Symbol,
	entry string,
	program *queryplan.Program,
) ([]Binding, error) {
	parameters := entryParameters(program, entry)
	known := make(map[string]queryplan.Parameter, len(parameters))
	for _, parameter := range parameters {
		known[parameter.Name] = parameter
	}
	bindings := make([]Binding, 0)
	bound := make(map[string]bool)
	for _, member := range localMembers(usage) {
		declaration, ok := member.Decl.(*ast.Usage)
		if !ok || declaration.Direction != ast.DirIn {
			continue
		}
		name := c.effectiveName(member)
		if _, ok := known[name]; !ok {
			return nil, &Error{
				Kind:      ErrorUnknownParameter,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: name,
				Origin:    provenance.Symbol(member),
			}
		}
		if bound[name] {
			return nil, &Error{
				Kind:      ErrorDuplicateBinding,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: name,
				Origin:    provenance.Symbol(member),
			}
		}
		bound[name] = true
		values, err := c.bindingValues(content, member, entry, name, declaration.Value)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, Binding{
			parameter: name,
			values:    values,
			origin:    provenance.Symbol(member),
		})
	}
	for _, parameter := range parameters {
		if bound[parameter.Name] {
			binding := bindingFor(bindings, parameter.Name)
			if err := c.validateBinding(content, entry, parameter, binding); err != nil {
				return nil, err
			}
			continue
		}
		if parameter.HasDefault {
			return nil, &Error{
				Kind:      ErrorDefaultUnavailable,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: parameter.Name,
				Origin:    provenance.Symbol(usage),
			}
		}
		if parameter.Multiplicity.Known && parameter.Multiplicity.Lower == 0 {
			continue
		}
		return nil, &Error{
			Kind:      ErrorMissingBinding,
			Document:  c.document,
			Content:   c.contentName(content),
			Query:     entry,
			Parameter: parameter.Name,
			Origin:    provenance.Symbol(usage),
		}
	}
	return bindings, nil
}

func entryParameters(program *queryplan.Program, entry string) []queryplan.Parameter {
	for _, definition := range program.Definitions() {
		if definition.Name() == entry {
			return definition.Parameters()
		}
	}
	return nil
}

func bindingFor(bindings []Binding, name string) Binding {
	for _, binding := range bindings {
		if binding.parameter == name {
			return binding
		}
	}
	return Binding{}
}

// bindingValues compiles one binding expression into planned values.
func (c *compiler) bindingValues(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	node ast.Node,
) ([]BindingValue, error) {
	if node == nil {
		return nil, c.unsupportedBinding(content, member, entry, parameter)
	}
	if sequenceExpr, ok := node.(*ast.SequenceExpr); ok {
		values := make([]BindingValue, 0, len(sequenceExpr.Elements))
		for _, element := range sequenceExpr.Elements {
			value, err := c.bindingValue(content, member, entry, parameter, element)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	}
	value, err := c.bindingValue(content, member, entry, parameter, node)
	if err != nil {
		return nil, err
	}
	return []BindingValue{value}, nil
}

func (c *compiler) bindingValue(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	node ast.Node,
) (BindingValue, error) {
	origin := provenance.Node(member.DocName, node)
	switch expression := node.(type) {
	case *ast.FeatureReference:
		return c.elementBinding(content, member, entry, parameter, expression.Name, origin)
	case *ast.QualifiedName:
		return c.elementBinding(content, member, entry, parameter, expression, origin)
	case *ast.LiteralString:
		text, err := strconv.Unquote(expression.Value)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingString, text: text, origin: origin}, nil
	case *ast.LiteralInteger:
		integer, err := strconv.ParseInt(expression.Value, 10, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingInteger, integer: integer, origin: origin}, nil
	case *ast.LiteralReal:
		real, err := strconv.ParseFloat(expression.Value, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingReal, real: real, origin: origin}, nil
	case *ast.LiteralBool:
		return BindingValue{kind: BindingBoolean, boolean: expression.Value, origin: origin}, nil
	case *ast.OperatorExpr:
		return c.signedBinding(content, member, entry, parameter, expression, origin)
	default:
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
}

// signedBinding compiles a unary-signed numeric literal binding.
func (c *compiler) signedBinding(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	expression *ast.OperatorExpr,
	origin provenance.Origin,
) (BindingValue, error) {
	if (expression.Operator != ast.OpNeg && expression.Operator != ast.OpPos) || len(expression.Operands) != 1 {
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
	sign := ""
	if expression.Operator == ast.OpNeg {
		sign = "-"
	}
	switch operand := expression.Operands[0].(type) {
	case *ast.LiteralInteger:
		integer, err := strconv.ParseInt(sign+operand.Value, 10, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingInteger, integer: integer, origin: origin}, nil
	case *ast.LiteralReal:
		real, err := strconv.ParseFloat(sign+operand.Value, 64)
		if err != nil {
			return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
		}
		return BindingValue{kind: BindingReal, real: real, origin: origin}, nil
	default:
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
}

func (c *compiler) elementBinding(
	content *symbols.Symbol,
	member *symbols.Symbol,
	entry string,
	parameter string,
	name *ast.QualifiedName,
	origin provenance.Origin,
) (BindingValue, error) {
	resolved, ok := c.resolver.ResolveQualified(member.OwnerScope, name)
	if !ok || resolved == nil {
		return BindingValue{}, c.unsupportedBinding(content, member, entry, parameter)
	}
	if canonical, ok := c.resolver.ResolveAliasTarget(resolved); ok {
		resolved = canonical
	}
	return BindingValue{kind: BindingElement, element: resolved, origin: origin}, nil
}

func (c *compiler) unsupportedBinding(content, member *symbols.Symbol, entry, parameter string) error {
	return &Error{
		Kind:      ErrorUnsupportedBinding,
		Document:  c.document,
		Content:   c.contentName(content),
		Query:     entry,
		Parameter: parameter,
		Origin:    provenance.Symbol(member),
	}
}

// validateBinding checks one bound parameter against its compiled signature.
func (c *compiler) validateBinding(
	content *symbols.Symbol,
	entry string,
	parameter queryplan.Parameter,
	binding Binding,
) error {
	if !withinMultiplicity(int64(len(binding.values)), parameter.Multiplicity) {
		return &Error{
			Kind:      ErrorBindingMultiplicity,
			Document:  c.document,
			Content:   c.contentName(content),
			Query:     entry,
			Parameter: parameter.Name,
			Expected:  multiplicityString(parameter.Multiplicity),
			Actual:    strconv.Itoa(len(binding.values)),
			Origin:    binding.origin,
		}
	}
	for _, value := range binding.values {
		if !c.valueConforms(value, parameter.Type) {
			actual := string(value.Kind())
			if element, ok := value.Element(); ok {
				actual = symbols.FQNOf(element)
			}
			return &Error{
				Kind:      ErrorBindingType,
				Document:  c.document,
				Content:   c.contentName(content),
				Query:     entry,
				Parameter: parameter.Name,
				Expected:  parameter.Type,
				Actual:    actual,
				Origin:    value.Origin(),
			}
		}
	}
	return nil
}

func withinMultiplicity(count int64, multiplicity queryplan.Multiplicity) bool {
	if !multiplicity.Known {
		return true
	}
	if count < multiplicity.Lower {
		return false
	}
	return multiplicity.UpperInfinite || count <= multiplicity.Upper
}

func multiplicityString(multiplicity queryplan.Multiplicity) string {
	if !multiplicity.Known {
		return "unknown"
	}
	upper := strconv.FormatInt(multiplicity.Upper, 10)
	if multiplicity.UpperInfinite {
		upper = "*"
	}
	return strconv.FormatInt(multiplicity.Lower, 10) + ".." + upper
}

// valueConforms mirrors execution-time binding conformance for planned values.
func (c *compiler) valueConforms(value BindingValue, expected string) bool {
	if element, ok := value.Element(); ok {
		for _, target := range c.index.LookupQualified(expected) {
			if symbols.SameElement(element, target) || c.model.Conforms(element, target) {
				return true
			}
		}
		return expected == "Element" || expected == "KerML::Root::Element"
	}
	actual, ok := scalarBindingType(value)
	if !ok {
		return false
	}
	for _, target := range c.index.LookupQualified(expected) {
		expectedType := c.model.PrimTypeOf(target)
		if expectedType != semantics.PrimUnknown && semantics.PrimConforms(actual, expectedType) {
			return true
		}
	}
	return false
}

func scalarBindingType(value BindingValue) (semantics.PrimType, bool) {
	switch value.Kind() {
	case BindingBoolean:
		return semantics.PrimBoolean, true
	case BindingString:
		return semantics.PrimString, true
	case BindingInteger:
		return semantics.PrimInteger, true
	case BindingReal:
		return semantics.PrimReal, true
	default:
		return semantics.PrimUnknown, false
	}
}

// requiredText reads a required string attribute of one structural member.
func (c *compiler) requiredText(member *symbols.Symbol, attribute string, missing ErrorKind) (string, error) {
	text, stated, err := c.optionalText(member, attribute)
	if err != nil {
		return "", err
	}
	if !stated {
		return "", &Error{
			Kind:     missing,
			Document: c.document,
			Content:  c.contentName(member),
			Origin:   provenance.Symbol(member),
		}
	}
	return text, nil
}

// optionalText reads an optional string attribute of one structural member.
func (c *compiler) optionalText(member *symbols.Symbol, attribute string) (string, bool, error) {
	for _, candidate := range c.effectiveMembers(member) {
		if candidate.Kind != symbols.SymbolAttributeUsage || c.effectiveName(candidate) != attribute {
			continue
		}
		text, stated, err := c.attributeText(member, candidate, attribute, make(map[*symbols.Symbol]bool))
		if err != nil || stated {
			return text, stated, err
		}
	}
	return "", false, nil
}

// attributeText reads a declaration's string value, following redefinition
// lineage when the declaration itself is unvalued.
func (c *compiler) attributeText(
	member *symbols.Symbol,
	candidate *symbols.Symbol,
	attribute string,
	seen map[*symbols.Symbol]bool,
) (string, bool, error) {
	if candidate == nil || seen[candidate] {
		return "", false, nil
	}
	seen[candidate] = true
	if declaration, ok := candidate.Decl.(*ast.Usage); ok && declaration.Value != nil {
		literal, ok := declaration.Value.(*ast.LiteralString)
		if !ok {
			return "", false, c.invalidAttribute(member, candidate, attribute)
		}
		text, err := strconv.Unquote(literal.Value)
		if err != nil {
			return "", false, c.invalidAttribute(member, candidate, attribute)
		}
		return text, true, nil
	}
	for _, target := range c.model.RedefinedFeatures(candidate) {
		text, stated, err := c.attributeText(member, target, attribute, seen)
		if err != nil || stated {
			return text, stated, err
		}
	}
	return "", false, nil
}

func (c *compiler) invalidAttribute(member, candidate *symbols.Symbol, attribute string) error {
	return &Error{
		Kind:      ErrorInvalidAttribute,
		Document:  c.document,
		Content:   c.contentName(member),
		Parameter: attribute,
		Origin:    provenance.Symbol(candidate),
	}
}

func (c *compiler) contentName(member *symbols.Symbol) string {
	if fqn := symbols.FQNOf(member); fqn != "" {
		return fqn
	}
	return member.Name
}

// localMembers returns a scope's named and anonymous declarations in source order.
func localMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	members := sym.Scope.Members()
	if sym.Scope.HasAnonymousMembers() {
		members = append(members, sym.Scope.AnonymousMembers()...)
		sort.SliceStable(members, func(i, j int) bool {
			return members[i].DeclSpan.Offset < members[j].DeclSpan.Offset
		})
	}
	return members
}

// effectiveMembers returns local declarations in source order followed by
// unmasked inherited members.
func (c *compiler) effectiveMembers(sym *symbols.Symbol) []*symbols.Symbol {
	local := localMembers(sym)
	seen := make(map[*symbols.Symbol]bool, len(local))
	for _, member := range local {
		seen[member] = true
	}
	members := local
	for _, member := range c.model.MembersOf(sym) {
		if !seen[member] {
			members = append(members, member)
		}
	}
	return members
}

// effectiveName returns a member's declared or redefinition-inherited name.
func (c *compiler) effectiveName(sym *symbols.Symbol) string {
	if name := c.model.EffectiveNameOf(sym); name != "" {
		return name
	}
	return sym.Name
}
