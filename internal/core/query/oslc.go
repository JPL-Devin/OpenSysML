package query

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// Query parameters this package names in more than one place.
const (
	paramWhere      = "oslc.where"
	paramSelect     = "oslc.select"
	paramOrderBy    = "oslc.orderBy"
	paramPrefix     = "oslc.prefix"
	paramProperties = "oslc.properties"
	paramSearch     = "oslc.searchTerms"
)

// oslcParameters is the closed set of query parameters this implementation
// reads. Ignoring any other one would answer a different query than the one
// asked: a misspelt oslc.where would select the whole model rather than fail.
var oslcParameters = map[string]bool{
	paramWhere: true, paramSelect: true, paramOrderBy: true,
	paramPrefix: true, paramProperties: true, paramSearch: true,
}

// ParameterNames returns the query parameters this implementation reads, in
// stable order. It is what an unknown-parameter error lists.
func ParameterNames() []string {
	out := make([]string, 0, len(oslcParameters))
	for name := range oslcParameters {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DefaultPrefixes are the prefixes available without an oslc.prefix binding.
var DefaultPrefixes = map[string]string{
	"sysml": rdf.SysML,
	"rdf":   rdf.RDFNS,
	"xsd":   rdf.XSD,
}

var oslcPropertyMappings = map[string]string{
	rdf.RDFNS + "type":              PropertyType,
	rdf.SysML + "qualifiedName":     PropertyQualifiedName,
	rdf.SysML + "name":              PropertyName,
	rdf.SysML + "declaredName":      PropertyDeclaredName,
	rdf.SysML + "owner":             PropertyOwner,
	rdf.SysML + "isAbstract":        PropertyIsAbstract,
	rdf.SysML + "type":              PropertyElementType,
	rdf.SysML + "multiplicityLower": PropertyMultiplicityLower,
	rdf.SysML + "multiplicityUpper": PropertyMultiplicityUpper,
}

// ParseOSLC parses a where expression. It also accepts a query-parameter
// string, allowing callers to pass an HTTP-style OSLC query directly.
func ParseOSLC(text string) (Query, error) {
	if isParameterText(text) {
		return ParseParameters(text)
	}
	p := oslcParser{s: text, prefixes: clonePrefixes(DefaultPrefixes)}
	q, err := p.parseCompound()
	if err != nil {
		return Query{}, err
	}
	return q, nil
}

func isParameterText(text string) bool {
	if strings.HasPrefix(strings.TrimSpace(text), "oslc.") {
		return true
	}
	for _, part := range strings.Split(text, "&") {
		if strings.HasPrefix(strings.TrimSpace(part), "oslc.") {
			return true
		}
	}
	return false
}

// ParseParameters parses oslc.where, oslc.select, oslc.orderBy, and
// oslc.prefix parameters. Parameter text follows URL query decoding rules.
func ParseParameters(text string) (Query, error) {
	values, err := url.ParseQuery(text)
	if err != nil {
		return Query{}, errorf(ErrMalformed, "malformed OSLC query parameters: %v", err)
	}
	if err := checkParameters(values); err != nil {
		return Query{}, err
	}
	prefixes := clonePrefixes(DefaultPrefixes)
	if prefixText := values.Get(paramPrefix); prefixText != "" {
		if err := parsePrefixes(prefixText, prefixes); err != nil {
			return Query{}, err
		}
	}
	if _, present := values[paramSearch]; present {
		return Query{}, errorf(ErrUnsupportedSearchTerms, "oslc.searchTerms is not implemented: the implementation performs element identification, not free-text search")
	}
	if properties, present := values[paramProperties]; present && len(properties) > 0 {
		if strings.TrimSpace(properties[0]) == "*" {
			return Query{}, wildcardError(paramProperties)
		}
		return Query{}, errorf(ErrMalformed, "oslc.properties is not implemented: name the properties to report with %s", paramSelect)
	}
	selectTerms, err := splitCommaStrict(values.Get(paramSelect), paramSelect)
	if err != nil {
		return Query{}, err
	}
	q := Query{Select: selectTerms}
	if order := values.Get(paramOrderBy); order != "" {
		terms, err := parseOrder(order, prefixes)
		if err != nil {
			return Query{}, err
		}
		q.OrderBy = terms
	}
	if where, present := values[paramWhere]; present {
		p := oslcParser{s: where[0], prefixes: prefixes}
		parsed, err := p.parseCompound()
		if err != nil {
			return Query{}, err
		}
		q.Where = parsed.Where
	}
	for _, name := range q.Select {
		if name == "*" {
			return Query{}, wildcardError(paramSelect)
		}
		if strings.ContainsAny(name, "{}") {
			return Query{}, scopedError()
		}
		resolved, err := resolveProperty(name, prefixes)
		if err != nil {
			return Query{}, err
		}
		q.Select = replace(q.Select, name, resolved)
	}
	return q, nil
}

// omitting names, per parameter, what leaving it out asks for: an empty value
// is a misuse, since the parameter as written narrows nothing.
var omitting = map[string]string{
	paramWhere:   "select every element",
	paramSelect:  "report every property",
	paramOrderBy: "keep declaration order",
	paramPrefix:  "use the default prefixes",
}

// checkParameters refuses a parameter this implementation does not read, one
// given more than once (reading only the first would discard the rest), and
// one given no value.
func checkParameters(values url.Values) error {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !oslcParameters[name] {
			return errorf(ErrMalformed, "unknown OSLC query parameter %q; this implementation reads %s",
				name, strings.Join(ParameterNames(), ", "))
		}
		if len(values[name]) > 1 {
			return errorf(ErrMalformed, "OSLC query parameter %q is given %d times", name, len(values[name]))
		}
		if omit, read := omitting[name]; read && strings.TrimSpace(values[name][0]) == "" {
			return errorf(ErrMalformed, "%s is empty: omit it to %s", name, omit)
		}
	}
	return nil
}

func parsePrefixes(text string, prefixes map[string]string) error {
	bindings, err := splitCommaStrict(text, paramPrefix)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		name, iri, ok := strings.Cut(strings.TrimSpace(binding), "=")
		if !ok {
			return errorf(ErrMalformed, "malformed oslc.prefix binding %q", binding)
		}
		iri = strings.TrimSpace(iri)
		if len(iri) < 2 || iri[0] != '<' || iri[len(iri)-1] != '>' {
			return errorf(ErrMalformed, "oslc.prefix binding %q must use an IRI", binding)
		}
		prefixes[strings.TrimSuffix(strings.TrimSpace(name), ":")] = iri[1 : len(iri)-1]
	}
	return nil
}

func parseOrder(text string, prefixes map[string]string) ([]OrderTerm, error) {
	var out []OrderTerm
	terms, err := splitCommaStrict(text, paramOrderBy)
	if err != nil {
		return nil, err
	}
	for _, raw := range terms {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, errorf(ErrMalformed, "oslc.orderBy contains an empty term")
		}
		desc := false
		if raw[0] == '+' || raw[0] == '-' {
			desc, raw = raw[0] == '-', strings.TrimSpace(raw[1:])
		}
		if strings.ContainsAny(raw, "{}") {
			return nil, scopedError()
		}
		if raw == "*" {
			return nil, wildcardError(paramOrderBy)
		}
		name, err := resolveProperty(raw, prefixes)
		if err != nil {
			return nil, err
		}
		out = append(out, OrderTerm{Property: name, Desc: desc})
	}
	return out, nil
}

type oslcParser struct {
	s        string
	pos      int
	prefixes map[string]string
}

func (p *oslcParser) parseCompound() (Query, error) {
	var q Query
	for {
		p.skipSpace()
		if p.pos >= len(p.s) {
			if len(q.Where) == 0 {
				return Query{}, errorf(ErrMalformed, "OSLC compound term is empty")
			}
			return q, nil
		}
		identifier, err := p.identifier()
		if err != nil {
			return Query{}, err
		}
		p.skipSpace()
		if p.peek('{') {
			return Query{}, scopedError()
		}
		prop, err := resolveProperty(identifier, p.prefixes)
		if err != nil {
			return Query{}, err
		}
		if p.consumeWord("in") {
			p.skipSpace()
			if !p.consume('[') {
				return Query{}, errorf(ErrMalformed, "OSLC in term for %q must contain a value list", identifier)
			}
			var vals []string
			for {
				p.skipSpace()
				value, err := p.value()
				if err != nil {
					return Query{}, err
				}
				vals = append(vals, value)
				p.skipSpace()
				if p.consume(']') {
					break
				}
				if !p.consume(',') {
					return Query{}, errorf(ErrMalformed, "OSLC in term for %q needs ',' or ']'", identifier)
				}
			}
			q.Where = append(q.Where, Predicate{Property: prop, Operator: In, Values: vals})
		} else {
			op := p.operator()
			if op == "" {
				return Query{}, errorf(ErrMalformed, "OSLC term for %q has no comparison operator", identifier)
			}
			p.skipSpace()
			value, err := p.value()
			if err != nil {
				return Query{}, err
			}
			q.Where = append(q.Where, Predicate{Property: prop, Operator: op, Values: []string{value}})
		}
		p.skipSpace()
		if p.pos >= len(p.s) {
			return q, nil
		}
		if !p.consumeWord("and") {
			return Query{}, errorf(ErrMalformed, "OSLC compound terms may be joined only by \" and \"")
		}
		p.skipSpace()
		if p.pos >= len(p.s) {
			return Query{}, errorf(ErrMalformed, "OSLC compound term has a trailing \"and\"")
		}
	}
}

func (p *oslcParser) identifier() (string, error) {
	if p.peek('*') {
		p.pos++
		return "*", nil
	}
	start := p.pos
	// A leading "@" scans as a name so resolveProperty can name its OSLC spelling.
	if p.peek('@') {
		p.pos++
	}
	for p.pos < len(p.s) {
		r := rune(p.s[p.pos])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-.:", r) {
			p.pos++
			continue
		}
		break
	}
	word := p.s[start:p.pos]
	if start == p.pos || !(strings.Contains(word, ":") || strings.HasPrefix(word, "@")) {
		return "", errorf(ErrMalformed, "OSLC term requires a prefixed-name identifier")
	}
	return word, nil
}

func (p *oslcParser) operator() Operator {
	for _, candidate := range []struct {
		text string
		op   Operator
	}{{">=", GreaterEqual}, {"<=", LessEqual}, {"!=", NotEqual}, {"=", Equal}, {">", Greater}, {"<", Less}} {
		if strings.HasPrefix(p.s[p.pos:], candidate.text) {
			p.pos += len(candidate.text)
			return candidate.op
		}
	}
	return ""
}

func (p *oslcParser) value() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return "", errorf(ErrMalformed, "OSLC term has no value")
	}
	if p.s[p.pos] == '<' {
		start := p.pos + 1
		end := strings.IndexByte(p.s[start:], '>')
		if end < 0 {
			return "", errorf(ErrMalformed, "unterminated URI value")
		}
		p.pos = start + end + 1
		return localValue(p.s[start : start+end]), nil
	}
	if p.s[p.pos] == '"' {
		p.pos++
		var b strings.Builder
		for p.pos < len(p.s) {
			c := p.s[p.pos]
			p.pos++
			if c == '"' {
				if p.pos < len(p.s) && p.s[p.pos] == '@' {
					return "", errorf(ErrUnsupportedLiteral, "language-tagged literals are not supported")
				}
				if p.pos+1 < len(p.s) && strings.HasPrefix(p.s[p.pos:], "^^") {
					p.pos += 2
					dt, err := p.identifier()
					if err != nil {
						return "", err
					}
					if !strings.HasPrefix(dt, "xsd:") {
						return "", errorf(ErrUnsupportedLiteral, "non-xsd datatype %q is not supported", dt)
					}
					switch strings.TrimPrefix(dt, "xsd:") {
					case "string", "boolean", "integer", "decimal", "double":
					default:
						return "", errorf(ErrUnsupportedLiteral, "xsd datatype %q is not supported", dt)
					}
				}
				return b.String(), nil
			}
			if c == '\\' {
				if p.pos >= len(p.s) || (p.s[p.pos] != '\\' && p.s[p.pos] != '"') {
					return "", errorf(ErrMalformed, "literal contains unsupported escape")
				}
				b.WriteByte(p.s[p.pos])
				p.pos++
				continue
			}
			b.WriteByte(c)
		}
		return "", errorf(ErrMalformed, "unterminated string literal")
	}
	if p.s[p.pos] == '*' {
		p.pos++
		return "*", nil
	}
	start := p.pos
	for p.pos < len(p.s) && !unicode.IsSpace(rune(p.s[p.pos])) && !strings.ContainsRune(",]}", rune(p.s[p.pos])) {
		p.pos++
	}
	raw := p.s[start:p.pos]
	if raw == "true" || raw == "false" {
		return raw, nil
	}
	if _, err := strconv.ParseFloat(raw, 64); err == nil {
		return raw, nil
	}
	if strings.Contains(raw, ":") {
		return resolveValue(raw, p.prefixes)
	}
	return "", errorf(ErrMalformed, "invalid OSLC value %q", raw)
}

func (p *oslcParser) skipSpace() {
	for p.pos < len(p.s) && unicode.IsSpace(rune(p.s[p.pos])) {
		p.pos++
	}
}

func (p *oslcParser) consume(c byte) bool {
	if p.pos < len(p.s) && p.s[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *oslcParser) peek(c byte) bool { return p.pos < len(p.s) && p.s[p.pos] == c }

func (p *oslcParser) consumeWord(word string) bool {
	if !strings.HasPrefix(p.s[p.pos:], word) {
		return false
	}
	end := p.pos + len(word)
	if end < len(p.s) && !unicode.IsSpace(rune(p.s[end])) {
		return false
	}
	p.pos = end
	return true
}

func resolveProperty(name string, prefixes map[string]string) (string, error) {
	if name == "*" {
		return "", wildcardError("oslc.where")
	}
	// The Go API names these two "@type" and "@id"; OSLC query text cannot.
	switch name {
	case PropertyType:
		return "", errorf(ErrMalformed, "OSLC query text spells the metamodel type \"rdf:type\", not %q", name)
	case PropertyID:
		return "", errorf(ErrMalformed,
			"%q is not an OSLC query property: element identity is reported for every result", name)
	}
	iri, err := expandName(name, prefixes)
	if err != nil {
		return "", err
	}
	if property, ok := oslcPropertyMappings[iri]; ok {
		return property, nil
	}
	return "", unknownOSLCProperty(name, prefixes)
}

// propertyNamespaces are the namespaces the OSLC property mapping spells its
// properties in.
var propertyNamespaces = []string{rdf.RDFNS, rdf.SysML}

// PrefixedPropertyNames returns the property names OSLC query text spells
// under prefixes, in stable order, along with the local names of every
// namespace those bindings leave unnamed, keyed by namespace. An oslc.prefix
// binding decides what the parser accepts, so a diagnostic reads the same map.
func PrefixedPropertyNames(prefixes map[string]string) (names []string, unbound map[string][]string) {
	unbound = make(map[string][]string)
	for iri := range oslcPropertyMappings {
		namespace, local := splitPropertyIRI(iri)
		if prefix, bound := namingPrefix(prefixes, namespace); bound {
			names = append(names, prefix+":"+local)
			continue
		}
		unbound[namespace] = append(unbound[namespace], local)
	}
	sort.Strings(names)
	for _, locals := range unbound {
		sort.Strings(locals)
	}
	return names, unbound
}

func splitPropertyIRI(iri string) (namespace, local string) {
	for _, namespace := range propertyNamespaces {
		if strings.HasPrefix(iri, namespace) {
			return namespace, strings.TrimPrefix(iri, namespace)
		}
	}
	return "", iri
}

// namingPrefix returns the alphabetically first prefix bound to namespace, so
// one namespace is always offered under one spelling.
func namingPrefix(prefixes map[string]string, namespace string) (string, bool) {
	bound := make([]string, 0, len(prefixes))
	for prefix, iri := range prefixes {
		if iri == namespace {
			bound = append(bound, prefix)
		}
	}
	if len(bound) == 0 {
		return "", false
	}
	sort.Strings(bound)
	return bound[0], true
}

func unknownOSLCProperty(name string, prefixes map[string]string) *Error {
	names, unbound := PrefixedPropertyNames(prefixes)
	var b strings.Builder
	fmt.Fprintf(&b, "unknown OSLC query property %q", name)
	if len(names) > 0 {
		fmt.Fprintf(&b, "; this implementation reads %s", strings.Join(names, ", "))
	}
	namespaces := make([]string, 0, len(unbound))
	for namespace := range unbound {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		fmt.Fprintf(&b, "; it also reads %s of <%s>, which no %s binding names",
			strings.Join(unbound[namespace], ", "), namespace, paramPrefix)
	}
	return errorf(ErrUnknownProperty, "%s", b.String())
}

func resolveValue(name string, prefixes map[string]string) (string, error) {
	// A model qualified name separates its first two segments with "::", where a
	// prefixed name separates prefix from local part with one ":".
	if _, local, _ := strings.Cut(name, ":"); strings.HasPrefix(local, ":") {
		return "", errorf(ErrMalformed,
			"OSLC value %q is a model qualified name, not a prefixed name; write it as a quoted literal %q", name, name)
	}
	iri, err := expandName(name, prefixes)
	if err != nil {
		return "", err
	}
	return localValue(iri), nil
}

// localValue reads the value an IRI denotes: a term of the SysML namespace
// denotes the model value its local name spells. A `<uri>` and the prefixed
// name expanding to it must reduce alike.
func localValue(iri string) string {
	if strings.HasPrefix(iri, rdf.SysML) {
		return strings.TrimPrefix(iri, rdf.SysML)
	}
	return iri
}

func expandName(name string, prefixes map[string]string) (string, error) {
	prefix, local, ok := strings.Cut(name, ":")
	if !ok {
		return "", errorf(ErrMalformed, "OSLC name %q is not a prefixed name", name)
	}
	namespace, ok := prefixes[prefix]
	if !ok {
		return "", errorf(ErrMalformed, "OSLC prefix %q is unbound", prefix)
	}
	return namespace + local, nil
}

func clonePrefixes(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func splitCommaStrict(text, parameter string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errorf(ErrMalformed, "%s contains an empty term", parameter)
		}
		out = append(out, part)
	}
	return out, nil
}

func replace(values []string, old, new string) []string {
	out := append([]string(nil), values...)
	for i, value := range out {
		if value == old {
			out[i] = new
		}
	}
	return out
}

func scopedError() *Error {
	return errorf(ErrUnsupportedScopedTerm, "scoped_term is not implemented: OSLC scoped terms are intended to match a resource based on a property of a related resource; this evaluator performs element identification over the parsed model")
}

func wildcardError(property string) *Error {
	return errorf(ErrUnsupportedWildcard, "the OSLC '*' wildcard is not implemented for %q: this evaluator identifies concrete model elements", property)
}
