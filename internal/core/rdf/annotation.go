package rdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Flexo reads a collection from the <AnnotationJSON><key> literal and skips a
// repeated sysml: predicate (ElementApi.kt); its commit path writes both (CommitApi.kt).

// AnnotationJSONTerm returns the annotation predicate carrying key's collection.
func AnnotationJSONTerm(key string) Term { return IRI(AnnotationJSON + key) }

// IsAnnotationJSON reports whether iri is a collection annotation predicate.
func IsAnnotationJSON(iri string) bool { return strings.HasPrefix(iri, AnnotationJSON) }

// AnnotateCollections adds the annotation literal for every sysml: property a subject
// states more than once, in triple order; an annotation already present is kept.
func AnnotateCollections(g *Graph) error {
	type pair struct {
		subject   Term
		predicate string
	}
	var order []pair
	objects := map[pair][]Term{}
	for _, t := range g.Triples() {
		if !strings.HasPrefix(t.Predicate.Value, SysML) {
			continue
		}
		p := pair{t.Subject, t.Predicate.Value}
		if _, seen := objects[p]; !seen {
			order = append(order, p)
		}
		objects[p] = append(objects[p], t.Object)
	}
	for _, p := range order {
		values := objects[p]
		if len(values) < 2 {
			continue
		}
		key := strings.TrimPrefix(p.predicate, SysML)
		if g.HasProperty(p.subject, AnnotationJSON+key) {
			continue
		}
		text, err := CollectionJSON(values)
		if err != nil {
			return fmt.Errorf("annotate %s of <%s>: %w", key, p.subject.Value, err)
		}
		g.Add(p.subject, AnnotationJSONTerm(key), String(text))
	}
	return nil
}

// CollectionJSON spells values as the service stores a collection: a JSON array of
// {"@id": <id>} references and typed primitives.
func CollectionJSON(values []Term) (string, error) {
	members := make([]any, 0, len(values))
	for _, value := range values {
		member, err := jsonValue(value)
		if err != nil {
			return "", err
		}
		members = append(members, member)
	}
	return encodeJSON(members)
}

// ValueJSON spells one term as a collection member; two spellings compare by it.
func ValueJSON(value Term) (string, error) {
	member, err := jsonValue(value)
	if err != nil {
		return "", err
	}
	return encodeJSON(member)
}

// encodeJSON writes compact JSON without HTML escaping, as the service does.
func encodeJSON(value any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// reference is the JSON spelling of an IRI member.
type reference struct {
	ID string `json:"@id"`
}

// jsonValue maps a term onto its JSON member: reference, boolean, number or string.
func jsonValue(value Term) (any, error) {
	if value.IsIRI() {
		return reference{ID: LocalName(value.Value)}, nil
	}
	switch value.Datatype {
	case XSD + "boolean":
		return value.Value == "true" || value.Value == "1", nil
	case XSD + "integer", XSD + "decimal", XSD + "double", XSD + "float":
		number := json.Number(value.Value)
		if _, err := json.Marshal(number); err != nil {
			return nil, fmt.Errorf("%s is not a JSON number", value)
		}
		return number, nil
	}
	return value.Value, nil
}

// CollectionMember is one parsed annotation member: a reference by id, or a literal.
type CollectionMember struct {
	ID      string // element id of a {"@id": …} member; empty for a literal
	Literal Term   // typed literal of a primitive member
}

// JSON spells the member back as it stands in the annotation.
func (m CollectionMember) JSON() (string, error) {
	if m.ID != "" {
		return encodeJSON(reference{ID: m.ID})
	}
	return ValueJSON(m.Literal)
}

var (
	integerLexical = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	decimalLexical = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.[0-9]+$`)
)

// ParseCollectionJSON reads a collection annotation: a JSON array of {"@id": …}
// objects and primitives, which is all the service stores.
func ParseCollectionJSON(text string) ([]CollectionMember, error) {
	if !strings.HasPrefix(strings.TrimSpace(text), "[") {
		return nil, fmt.Errorf("the annotation is not a JSON array: %s", text)
	}
	dec := json.NewDecoder(strings.NewReader(text))
	var raw []json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("the annotation is not a JSON array: %w", err)
	}
	if rest := strings.TrimSpace(text[dec.InputOffset():]); rest != "" {
		return nil, fmt.Errorf("the annotation holds more than one JSON value: %s", rest)
	}
	members := make([]CollectionMember, 0, len(raw))
	for i, item := range raw {
		member, err := parseMember(item)
		if err != nil {
			return nil, fmt.Errorf("member %d: %w", i, err)
		}
		members = append(members, member)
	}
	return members, nil
}

func parseMember(item json.RawMessage) (CollectionMember, error) {
	dec := json.NewDecoder(bytes.NewReader(item))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return CollectionMember{}, err
	}
	switch v := value.(type) {
	case map[string]any:
		id, ok := v["@id"].(string)
		if !ok || len(v) != 1 || id == "" {
			return CollectionMember{}, fmt.Errorf("an object member is a reference {\"@id\": <id>}, not %s", item)
		}
		return CollectionMember{ID: id}, nil
	case string:
		return CollectionMember{Literal: String(v)}, nil
	case bool:
		return CollectionMember{Literal: Bool(v)}, nil
	case json.Number:
		lexical := v.String()
		switch {
		case integerLexical.MatchString(lexical):
			return CollectionMember{Literal: TypedLiteral(lexical, XSD+"integer")}, nil
		case decimalLexical.MatchString(lexical):
			return CollectionMember{Literal: TypedLiteral(lexical, XSD+"decimal")}, nil
		}
		return CollectionMember{Literal: TypedLiteral(lexical, XSD+"double")}, nil
	case nil:
		return CollectionMember{}, fmt.Errorf("a collection member cannot be null")
	}
	return CollectionMember{}, fmt.Errorf("a collection member cannot be %s", item)
}
