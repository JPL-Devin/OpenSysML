package rdf

import (
	"fmt"
	"sort"
	"strings"
)

// CollectionConflictError is a collection stated as typed triples and as a JSON
// annotation with different members; the reader refuses rather than pick one.
type CollectionConflictError struct {
	Subject    string   // subject IRI
	Key        string   // property key: the local name of the sysml: predicate
	Triples    []string // members as the typed triples state them, as JSON
	Annotation []string // members as the annotation states them, as JSON
}

func (e *CollectionConflictError) Error() string {
	return fmt.Sprintf("cannot convert <%s>: sysml:%s is stated twice and the two disagree: the typed triples hold [%s], the json:%s annotation holds [%s]",
		e.Subject, e.Key, strings.Join(e.Triples, ","), e.Key, strings.Join(e.Annotation, ","))
}

// AnnotationError is a collection annotation the reader cannot take as a
// collection: not exactly one literal, or not the JSON array the service stores.
type AnnotationError struct {
	Subject string // subject IRI
	Key     string // property key: the annotation predicate's local name
	Note    string
}

func (e *AnnotationError) Error() string {
	return fmt.Sprintf("cannot convert the annotation json:%s of <%s>: %s", e.Key, e.Subject, e.Note)
}

// collection is one annotated property of one subject, settled in annotation order.
type collection struct {
	subject Term
	key     string
	members []Term
	bare    bool // the graph also states the members as typed triples
}

// ReconcileCollections returns a graph whose sysml: triples state every annotated
// collection in annotation order: materialized if annotation-only, checked if both.
// A graph without annotations is returned as it is.
func ReconcileCollections(graph *Graph) (*Graph, error) {
	var annotations []Triple
	for _, triple := range graph.Triples() {
		if IsAnnotationJSON(triple.Predicate.Value) {
			annotations = append(annotations, triple)
		}
	}
	if len(annotations) == 0 {
		return graph, nil
	}
	type pair struct {
		subject   Term
		predicate string
	}
	ids := subjectsByID(graph)
	settled := map[pair]collection{}
	for _, triple := range annotations {
		c, err := reconcileCollection(graph, ids, triple.Subject, strings.TrimPrefix(triple.Predicate.Value, AnnotationJSON))
		if err != nil {
			return nil, err
		}
		settled[pair{c.subject, SysML + c.key}] = c
	}
	out := NewGraph()
	for label, ns := range graph.Prefixes {
		out.Prefixes[label] = ns
	}
	emitted := map[pair]bool{}
	for _, triple := range graph.Triples() {
		p := pair{triple.Subject, triple.Predicate.Value}
		if c, ok := settled[p]; ok && c.bare {
			if !emitted[p] {
				emitted[p] = true
				for _, member := range c.members {
					out.Add(c.subject, triple.Predicate, member)
				}
			}
			continue
		}
		out.AddTriple(triple)
		if IsAnnotationJSON(triple.Predicate.Value) {
			c := settled[pair{triple.Subject, SysML + strings.TrimPrefix(triple.Predicate.Value, AnnotationJSON)}]
			if !c.bare {
				for _, member := range c.members {
					out.Add(c.subject, SysMLTerm(c.key), member)
				}
			}
		}
	}
	return out, nil
}

// reconcileCollection settles one annotated property: one JSON-array literal,
// agreeing with the typed triples where the graph states those too.
func reconcileCollection(graph *Graph, ids map[string][]Term, subject Term, key string) (collection, error) {
	c := collection{subject: subject, key: key}
	refuse := func(note string) (collection, error) {
		return c, &AnnotationError{Subject: subject.Value, Key: key, Note: note}
	}
	objects := graph.Objects(subject, AnnotationJSON+key)
	if len(objects) != 1 {
		return refuse(fmt.Sprintf("it has %d objects, and a collection is stated by exactly one JSON literal", len(objects)))
	}
	if !objects[0].IsLiteral() {
		return refuse(fmt.Sprintf("its object %s is not a literal, and a collection is stated by one JSON literal", objects[0]))
	}
	members, err := ParseCollectionJSON(objects[0].Value)
	if err != nil {
		return refuse(err.Error())
	}
	annotation := make([]string, len(members))
	for i, member := range members {
		if annotation[i], err = member.JSON(); err != nil {
			return refuse(err.Error())
		}
	}
	bare := graph.Objects(subject, SysML+key)
	if len(bare) == 0 {
		for _, member := range members {
			c.members = append(c.members, materialize(member, ids))
		}
		return c, nil
	}
	c.bare = true
	spelled := make([]string, len(bare))
	for i, term := range bare {
		if spelled[i], err = ValueJSON(term); err != nil {
			return refuse(fmt.Sprintf("the typed sysml:%s value %s has no JSON spelling: %v", key, term, err))
		}
	}
	if !sameMembers(spelled, annotation) {
		return c, &CollectionConflictError{Subject: subject.Value, Key: key, Triples: spelled, Annotation: annotation}
	}
	// Both agree; the annotation carries the order.
	used := make([]bool, len(bare))
	for _, want := range annotation {
		for i, have := range spelled {
			if !used[i] && have == want {
				used[i] = true
				c.members = append(c.members, bare[i])
				break
			}
		}
	}
	return c, nil
}

// sameMembers compares two spellings as multisets; typed triples carry no order.
func sameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

// subjectsByID indexes IRI subjects by local id.
func subjectsByID(graph *Graph) map[string][]Term {
	ids := map[string][]Term{}
	for _, subject := range graph.Subjects() {
		if subject.IsIRI() {
			id := LocalName(subject.Value)
			ids[id] = append(ids[id], subject)
		}
	}
	return ids
}

// materialize turns an annotation member into the term a typed triple would hold;
// an id no subject carries keeps the element spelling and dangles as usual.
func materialize(member CollectionMember, ids map[string][]Term) Term {
	if member.ID == "" {
		return member.Literal
	}
	if subjects := ids[member.ID]; len(subjects) == 1 {
		return subjects[0]
	}
	return ElementIRIForID(member.ID)
}
