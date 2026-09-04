// Package rename refuses renames that would change what a name means: the new
// name taken where the element is declared, or a rewritten reference captured.
package rename

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Occurrence is one reference segment written with the name being renamed.
type Occurrence struct {
	// Span is the segment's bytes in its document.
	Span source.Span
	// Scope is where the reference is read; an unqualified segment is checked here.
	Scope *symbols.Scope
	// Qualifier is what the preceding segment reaches; nil for a first segment.
	Qualifier *symbols.Symbol
}

// Conflict is why a rename is refused: what the new name would mean instead.
type Conflict struct {
	// Subject is the qualified name of the element being renamed.
	Subject string
	// NewName is the name refused.
	NewName string
	// Means is the qualified name NewName already means, or would at a reference.
	Means string
	// Site is the namespace of the captured reference; empty for a taken name.
	Site string
}

// Error describes the conflict, naming what the new name would mean.
func (c *Conflict) Error() string {
	if c.Site == "" {
		return fmt.Sprintf("%s cannot be renamed to %q: that name already means %s where"+
			" %s is declared, so the rename would make it ambiguous or silently rebind"+
			" what reads that name", c.Subject, c.NewName, c.Means, c.Subject)
	}
	return fmt.Sprintf("%s cannot be renamed to %q: the reference to it in %s"+
		" would read %s instead, so the rename would change what that reference means",
		c.Subject, c.NewName, c.Site, c.Means)
}

// Check reports the first conflict renaming sym's written name to newName would
// create: the declaration's own scope first, then each occurrence in order.
func Check(r *resolve.Resolver, sym *symbols.Symbol, name, newName string, occurrences []Occurrence) *Conflict {
	subject := qualifiedName(r.Index(), sym)
	if means, ok := taken(r, sym, newName); ok {
		return &Conflict{Subject: subject, NewName: newName, Means: means}
	}
	for _, occ := range occurrences {
		if means, ok := capturedAt(r, sym, occ, newName); ok {
			return &Conflict{Subject: subject, NewName: newName, Means: means, Site: site(r, occ, name)}
		}
	}
	return nil
}

// taken names what newName already means where sym is declared, sym's own
// bindings hidden: a sibling made ambiguous, or an outer/inherited name shadowed.
func taken(r *resolve.Resolver, sym *symbols.Symbol, newName string) (string, bool) {
	if sym.OwnerScope == nil {
		return "", false
	}
	other, ok := r.LookupNameExcluding(sym.OwnerScope, newName, sym.Decl)
	return otherThan(r, sym, other, ok)
}

// capturedAt names what a rewritten reference would read instead of sym: an
// unqualified segment in its own scope, a qualified one as a member of its qualifier.
func capturedAt(r *resolve.Resolver, sym *symbols.Symbol, occ Occurrence, newName string) (string, bool) {
	if occ.Qualifier != nil {
		prefix := r.Index().GetFQN(occ.Qualifier)
		if prefix == "" {
			return "", false
		}
		candidate := prefix + "::" + newName
		for _, other := range r.Index().LookupQualifiedFrom(candidate, prefix) {
			if other != nil && !symbols.SameElement(other, sym) {
				return candidate, true
			}
		}
		return "", false
	}
	if occ.Scope == nil {
		return "", false
	}
	other, ok := r.LookupNameExcluding(occ.Scope, newName, sym.Decl)
	return otherThan(r, sym, other, ok)
}

// otherThan names a lookup's result when it is an element other than sym.
func otherThan(r *resolve.Resolver, sym, other *symbols.Symbol, ok bool) (string, bool) {
	if !ok || symbols.SameElement(other, sym) {
		return "", false
	}
	return qualifiedName(r.Index(), other), true
}

// qualifiedName is sym's FQN, or its name where the index records none.
func qualifiedName(idx *symbols.Index, sym *symbols.Symbol) string {
	if fqn := idx.GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}

// site names the namespace a reference is made in, or the name as written where
// that namespace has no FQN.
func site(r *resolve.Resolver, occ Occurrence, name string) string {
	if fqn := r.ReferringNamespaceFQN(occ.Scope); fqn != "" {
		return fqn
	}
	return name
}
