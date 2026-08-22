package resolve

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// walkQualified resolves a qualified name segment-by-segment, storing each
// segment's resolved symbol in the resolver's side table.
// hide, when set, makes the bindings of a reference subsetting's own borrowed
// name invisible to the first segment's lookup.
func (r *Resolver) walkQualified(scope *symbols.Scope, qn *ast.QualifiedName, hide *refFilter) resolution {
	if len(qn.Parts) == 0 {
		return resolution{nil, false}
	}

	// Single-segment qualified names (like type references) should use full
	// unqualified lookup (including imports), not just outward scope lookup.
	// Fall back to normal qualified lookup if scope is nil.
	if len(qn.Parts) == 1 && !qn.Global && scope != nil {
		res := r.walkUnqualifiedHiding(scope, qn.Parts[0].Text, hide)
		if res.ok {
			res.sym = r.resolvedPart(qn, 0, res.sym)
		} else {
			r.unresolved(scope, qn)
		}
		return res
	}

	// Resolve the first segment. A non-global name first searches the enclosing
	// scope chain; a global ($::) name starts at the document root. When the
	// local scope tree has no match, fall back to the global qualified-name
	// index so top-level names declared in other documents resolve.
	//
	// For non-global names, use import-aware unqualified lookup so that
	// multi-segment names like TrafficLightColor::green can resolve the first
	// segment via wildcard imports (e.g., import Def1::*).
	first := qn.Parts[0].Text
	var cur *symbols.Symbol
	if qn.Global {
		cur = r.lookupInRoot(scope, first)
	} else {
		// Use import-aware lookup for first segment of multi-part names
		res := r.walkUnqualifiedHiding(scope, first, hide)
		cur = res.sym
	}
	if cur == nil {
		sym, n := r.lookupGlobalTop(scope, first)
		if n > 1 {
			r.ambiguous(qn, n)
			return resolution{nil, false}
		}
		cur = sym
	}
	if cur == nil {
		r.unresolvedNamespace(qn, first)
		return resolution{nil, false}
	}
	cur = r.resolvedPart(qn, 0, cur)

	return r.walkQualifiedTail(scope, qn, cur, 1)
}

// walkQualifiedTail resolves qn's segments from start beneath cur.
func (r *Resolver) walkQualifiedTail(scope *symbols.Scope, qn *ast.QualifiedName, cur *symbols.Symbol, start int) resolution {
	from := r.ReferringNamespaceFQN(scope)
	curFQN := r.registeredFQN(cur)
	for i := start; i < len(qn.Parts); i++ {
		seg := qn.Parts[i]
		var all []*symbols.Symbol

		// Try local scope lookup first if available. A segment names a member of
		// the namespace the walk has reached, so it reaches only the visible ones.
		if cur.Scope != nil {
			all = r.namedThroughNamespaces(symbols.PreferDeclared(cur.Scope.LookupLocalAll(seg.Text)))
		}

		if len(all) == 0 && cur.Scope != nil {
			if sym, ok := r.lookupImportedMember(cur, cur.Scope, scope, seg.Text); ok &&
				r.namedThroughNamespace(sym) {
				all = []*symbols.Symbol{sym}
			}
		}

		// If local lookup fails (or no scope), look the segment up under the FQN
		// walked so far. This handles cases like ScalarValues::Real where
		// ScalarValues is a package from stdlib that was indexed with full FQNs
		// but doesn't have a populated Scope, at any nesting depth.
		memberFQN := curFQN + "::" + seg.Text
		if len(all) == 0 && r.idx != nil {
			found := r.idx.LookupQualifiedFrom(memberFQN, from)
			candidates := r.namedThroughNamespaces(
				r.admittedUnder(r.documentOf(scope), from, memberFQN, found))
			switch {
			case len(candidates) == 1:
				all = candidates
			case len(candidates) > 1:
				r.ambiguous(qn, len(candidates))
				return resolution{nil, false}
			case len(found) > 0:
				// Every candidate the name reaches is filtered out, so it is not a
				// member of the namespace it appears under (KerML 8.2.4) and no
				// other route may recover it.
				r.unresolved(scope, qn)
				return resolution{nil, false}
			}
		}

		// The name exists under cur but only because a private import surfaced it
		// there: it is invisible from here (KerML 8.2.3.3), and the member search
		// below reaches cached symbols by a route that does not know that.
		if len(all) == 0 && r.idx != nil && r.idx.HiddenFrom(memberFQN, from) {
			r.unresolved(scope, qn)
			return resolution{nil, false}
		}

		// A segment may name a member the current symbol inherits rather than
		// declares: `engine::'4cylEngine'` reaches the variants of the type
		// `engine` is typed by.
		if len(all) == 0 {
			if sym, ok := r.lookupMember(cur, seg.Text); ok &&
				r.namedThroughNamespace(sym) {
				all = []*symbols.Symbol{sym}
			}
		}

		if len(all) == 0 {
			r.unresolved(scope, qn)
			return resolution{nil, false}
		}
		if len(all) > 1 {
			r.ambiguous(qn, len(all))
			return resolution{nil, false}
		}
		cur = r.resolvedPart(qn, i, all[0])
		curFQN = r.registeredFQN(cur)
	}
	return resolution{cur, true}
}

// registeredFQN is the name a symbol's own members are indexed under: the path
// the index walks for a parsed symbol, and the already-qualified name a symbol
// restored from a cache record carries.
func (r *Resolver) registeredFQN(sym *symbols.Symbol) string {
	if r.idx == nil || sym == nil {
		return ""
	}
	if fqn := withoutEmptySegments(r.idx.GetFQN(sym)); fqn != "" {
		return fqn
	}
	return sym.Name
}

// ReferringNamespaceFQN returns the fully-qualified name of the namespace a
// reference made in scope belongs to, or "" for one made outside any namespace.
// It is the context a qualified lookup is answered in: a name a private wildcard
// import brought into a namespace is a member of it but visible only from
// within (KerML 8.2.3.3), so `Mid::Hidden` resolves inside Mid and nowhere else.
func (r *Resolver) ReferringNamespaceFQN(scope *symbols.Scope) string {
	if r.idx == nil {
		return ""
	}
	for s := scope; s != nil; s = s.Parent() {
		if owner := s.Owner(); owner != nil && owner.Name != "" {
			return withoutEmptySegments(r.idx.GetFQN(owner))
		}
	}
	return ""
}

// withoutEmptySegments drops the empty segments an unnamed enclosing element
// contributes to a fully-qualified name, so "Mid::::inner" reads as
// "Mid::inner" and still tests as nested inside Mid.
func withoutEmptySegments(fqn string) string {
	parts := strings.Split(fqn, "::")
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "::")
}

// lookupInRoot finds a name in the document root scope reachable from scope.
func (r *Resolver) lookupInRoot(scope *symbols.Scope, name string) *symbols.Symbol {
	root := rootOf(scope)
	if root == nil {
		return nil
	}
	sym, _ := root.LookupLocal(name)
	return sym
}

// lookupGlobalTop finds a top-level (single-segment FQN) symbol in the global
// index. Returns the unique match and the total number of matches, so the
// caller can report ambiguity (n > 1) rather than silently degrading to
// "unresolved". A unique symbol is returned only when n == 1.
func (r *Resolver) lookupGlobalTop(scope *symbols.Scope, name string) (*symbols.Symbol, int) {
	if r.idx == nil {
		return nil, 0
	}
	// A name reached here may be one a filtered import surfaced at a document's
	// root, so the conditions of the routes registering it decide it here too.
	syms := r.admittedUnder(r.documentOf(scope), r.ReferringNamespaceFQN(scope), name, r.idx.LookupQualified(name))
	if len(syms) == 1 {
		return syms[0], 1
	}
	return nil, len(syms)
}

// rootOf returns the topmost ancestor of scope (the document root), or nil.
func rootOf(scope *symbols.Scope) *symbols.Scope {
	if scope == nil {
		return nil
	}
	for scope.Parent() != nil {
		scope = scope.Parent()
	}
	return scope
}

// unresolved records an unresolved-reference diagnostic, offering the spellings
// a simple name may have meant. A qualified name already says where to look, so
// only an unqualified one is second-guessed.
func (r *Resolver) unresolved(scope *symbols.Scope, qn *ast.QualifiedName) {
	msg := unresolvedReferencePrefix + qnText(qn)
	var fixes []quickfix.Fix
	if len(qn.Parts) == 1 && !qn.Global {
		name := qn.Parts[0].Text
		msg = r.unresolvedMessage(scope, name)
		fixes = r.unresolvedFixes(scope, name, qn.Span())
	}
	r.report(Diagnostic{
		Span:    qn.Span(),
		Message: msg,
		Fixes:   fixes,
	})
}

// unresolvedNamespace records an unresolved-reference diagnostic for a
// qualified name whose qualifying namespace ns is not loaded at all, naming
// elements of the same simple name found elsewhere — which is what a reference
// into a library the workspace does not have looks like.
func (r *Resolver) unresolvedNamespace(qn *ast.QualifiedName, ns string) {
	msg := "unresolved reference: " + qnText(qn)
	if r.idx != nil && len(qn.Parts) > 1 {
		last := qn.Parts[len(qn.Parts)-1].Text
		if cands := r.idx.FQNsEndingIn(last, 3); len(cands) > 0 {
			msg += fmt.Sprintf(" (no namespace %q is loaded; %q is declared as %s)",
				ns, last, strings.Join(cands, ", "))
		}
	}
	r.report(Diagnostic{Span: qn.Span(), Message: msg})
}

// ambiguous records an ambiguity diagnostic reporting the number of matches.
func (r *Resolver) ambiguous(qn *ast.QualifiedName, n int) {
	r.report(Diagnostic{
		Span:    qn.Span(),
		Message: fmt.Sprintf("ambiguous reference: %s (%d candidates)", qnText(qn), n),
	})
}
