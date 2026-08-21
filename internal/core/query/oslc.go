package query

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

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
	prefixes := clonePrefixes(DefaultPrefixes)
	if prefixText := values.Get("oslc.prefix"); prefixText != "" {
		if err := parsePrefixes(prefixText, prefixes); err != nil {
			return Query{}, err
		}
	}
	if _, present := values["oslc.searchTerms"]; present {
		return Query{}, errorf(ErrUnsupportedSearchTerms, "oslc.searchTerms is not implemented: the implementation performs element identification, not free-text search")
	}
	if properties, present := values["oslc.properties"]; present && len(properties) > 0 && strings.TrimSpace(properties[0]) == "*" {
		return Query{}, wildcardError("oslc.properties")
	}
	selectTerms, err := splitCommaStrict(values.Get("oslc.select"), "oslc.select")
	if err != nil {
		return Query{}, err
	}
	q := Query{Select: selectTerms}
	if order := values.Get("oslc.orderBy"); order != "" {
		terms, err := parseOrder(order, prefixes)
		if err != nil {
			return Query{}, err
		}
		q.OrderBy = terms
	}
	if where := values.Get("oslc.where"); where != "" {
		p := oslcParser{s: where, prefixes: prefixes}
		parsed, err := p.parseCompound()
		if err != nil {
			return Query{}, err
		}
		q.Where = parsed.Where
	}
	for _, name := range q.Select {
		if name == "*" {
			return Query{}, wildcardError("oslc.select")
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

func parsePrefixes(text string, prefixes map[string]string) error {
	bindings, err := splitCommaStrict(text, "oslc.prefix")
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
	terms, err := splitCommaStrict(text, "oslc.orderBy")
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
			return nil, wildcardError("oslc.orderBy")
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
	}
}

func (p *oslcParser) identifier() (string, error) {
	if p.peek('*') {
		p.pos++
		return "*", nil
	}
	start := p.pos
	for p.pos < len(p.s) {
		r := rune(p.s[p.pos])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-.:", r) {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos || !strings.Contains(p.s[start:p.pos], ":") {
		return "", errorf(ErrMalformed, "OSLC term requires a prefixed-name identifier")
	}
	return p.s[start:p.pos], nil
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
		return p.s[start : start+end], nil
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
	iri, err := expandName(name, prefixes)
	if err != nil {
		return "", err
	}
	if property, ok := oslcPropertyMappings[iri]; ok {
		return property, nil
	}
	return "", unknownProperty(name)
}

func resolveValue(name string, prefixes map[string]string) (string, error) {
	iri, err := expandName(name, prefixes)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(iri, rdf.SysML) {
		return strings.TrimPrefix(iri, rdf.SysML), nil
	}
	return iri, nil
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
