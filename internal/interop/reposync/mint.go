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
// its owning membership by its exact suffix, and its expression nodes by the
// longest minted owner — never another element whose id merely shares a prefix.
func mintedTerm(term rdf.Term, mints map[string]string) rdf.Term {
	if !term.IsIRI() {
		return term
	}
	switch {
	case strings.HasPrefix(term.Value, rdf.Element):
		id := term.Value[len(rdf.Element):]
		if minted, ok := mints[id]; ok {
			return rdf.IRI(rdf.Element + minted)
		}
		// The only element-namespace satellite is the owning membership,
		// derived by the exact suffix; any longer id is another element's.
		if owner, ok := strings.CutSuffix(id, membershipSuffix); ok {
			if minted, exists := mints[owner]; exists {
				return rdf.IRI(rdf.Element + minted + membershipSuffix)
			}
		}
	case strings.HasPrefix(term.Value, rdf.Expression):
		id := term.Value[len(rdf.Expression):]
		if owner, rest := longestMintedOwner(id, mints); owner != "" {
			return rdf.IRI(rdf.Expression + mints[owner] + rest)
		}
	}
	return term
}

// longestMintedOwner finds the minted id an expression node derives from: the
// longest minted owner followed by the position separator, so a minted id that
// is a prefix of another never captures the other's nodes.
func longestMintedOwner(id string, mints map[string]string) (string, string) {
	best := ""
	for old := range mints {
		if len(old) <= len(best) {
			continue
		}
		if rest, ok := strings.CutPrefix(id, old); ok && strings.HasPrefix(rest, expressionSuffix) {
			best = old
		}
	}
	if best == "" {
		return "", ""
	}
	return best, id[len(best):]
}
