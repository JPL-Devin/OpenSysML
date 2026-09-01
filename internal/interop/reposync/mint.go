package reposync

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// MintUUID returns a random RFC 4122 version 4 UUID, the id minted for an
// element that needs addressing but declares none.
func MintUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("mint a uuid: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Suffixes through which a membership or expression node id derives from its
// element's, re-derived here so satellites follow a minted id.
const (
	membershipSuffix = "_om"
	expressionSuffix = "_p"
)

// WriteBack rewrites a local graph so each minted element declares its minted
// id: subjects and references move to the minted IRI, satellites re-derive,
// and the declaredId marker is set so converting the graph back to notation
// re-materializes the @ElementId annotation. It never runs implicitly: the
// caller asked to write annotations back into the model.
func WriteBack(g *rdf.Graph, mints map[string]string) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range g.Triples() {
		subject := mintedTerm(triple.Subject, mints)
		object := mintedTerm(triple.Object, mints)
		if triple.Predicate.Value == rdf.SysML+"elementId" && subject != triple.Subject {
			object = rdf.String(rdf.LocalName(subject.Value))
		}
		out.Add(subject, triple.Predicate, object)
	}
	for old, minted := range mints {
		subject := rdf.IRI(rdf.Element + minted)
		if _, ok := g.Object(rdf.IRI(rdf.Element+old), rdf.RDFType); ok {
			out.Add(subject, rdf.IRI(rdf.OpenSysML+"declaredId"), rdf.Bool(true))
		}
	}
	return out
}

// mintedTerm maps an IRI onto its minted spelling: the element itself exactly,
// and its memberships and expression nodes by their derivation suffix.
func mintedTerm(term rdf.Term, mints map[string]string) rdf.Term {
	if !term.IsIRI() {
		return term
	}
	var ns, id string
	switch {
	case strings.HasPrefix(term.Value, rdf.Element):
		ns, id = rdf.Element, term.Value[len(rdf.Element):]
	case strings.HasPrefix(term.Value, rdf.Expression):
		ns, id = rdf.Expression, term.Value[len(rdf.Expression):]
	default:
		return term
	}
	for old, minted := range mints {
		if id == old {
			return rdf.IRI(ns + minted)
		}
		rest, ok := strings.CutPrefix(id, old)
		if ok && (strings.HasPrefix(rest, membershipSuffix) || strings.HasPrefix(rest, expressionSuffix)) {
			return rdf.IRI(ns + minted + rest)
		}
	}
	return term
}
