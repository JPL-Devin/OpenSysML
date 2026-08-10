package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// lookupSymbol resolves the name a REPL command was given against the session
// document. It is the single lookup path for every symbol-taking command, so
// `%instantiate`, `%eval`, `%calc` and the debuggers all agree on what a name
// denotes.
//
// A simple name (Vehicle) is searched through the whole scope tree, so a member
// of a package is found without qualification; a qualified name
// (Demo::Vehicle) is resolved through the symbol index, the same API the gRPC
// service resolves its symbol ids with. The second result is the
// fully-qualified name the symbol was found under, which is what instances are
// keyed by so the spelling used to create one need not be the spelling used to
// inspect it.
func (s *Session) lookupSymbol(name string) (*symbols.Symbol, string, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil, "", fmt.Errorf("no declarations loaded")
	}
	idx := s.symbolIndex()
	if idx == nil {
		return nil, "", fmt.Errorf("no declarations loaded")
	}

	if strings.Contains(name, "::") {
		matches := idx.LookupQualified(name)
		switch len(matches) {
		case 0:
			return nil, "", fmt.Errorf("symbol %q not found", name)
		case 1:
			// The index owns its own scope tree; map the hit back onto the
			// document's tree so every command sees one symbol per declaration.
			sym := matches[0]
			fqn := idx.GetFQN(sym)
			if fqn == "" {
				fqn = name
			}
			if local := scopeSymbolFor(doc.Scope, sym.Decl); local != nil {
				sym = local
			}
			return sym, fqn, nil
		default:
			return nil, "", ambiguousError(name, matches, idx)
		}
	}

	matches := collectInScopeTree(doc.Scope, name)
	switch len(matches) {
	case 0:
		return nil, "", fmt.Errorf("symbol %q not found", name)
	case 1:
		return matches[0], idx.GetFQN(matches[0]), nil
	default:
		return nil, "", ambiguousError(name, matches, idx)
	}
}

// ambiguousError reports a name that matched more than one declaration, listing
// the candidates' fully-qualified names rather than picking one of them.
func ambiguousError(name string, matches []*symbols.Symbol, idx *symbols.Index) error {
	fqns := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, sym := range matches {
		fqn := idx.GetFQN(sym)
		if !seen[fqn] {
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	sort.Strings(fqns)
	return fmt.Errorf("symbol %q is ambiguous: %s (use a qualified name)", name, strings.Join(fqns, ", "))
}

// collectInScopeTree returns every symbol named name in scope or a nested
// scope. A body-local name is only visible inside its own body and is skipped.
func collectInScopeTree(scope *symbols.Scope, name string) []*symbols.Symbol {
	if scope == nil || scope.BodyLocal() {
		return nil
	}
	var out []*symbols.Symbol
	if syms := scope.LookupLocalAll(name); len(syms) > 0 {
		out = append(out, symbols.PreferDeclared(syms)...)
	}
	for _, child := range scope.Children() {
		out = append(out, collectInScopeTree(child, name)...)
	}
	return out
}

// lookupInScopeTree returns the first symbol named name in scope or a nested
// scope, or nil.
func lookupInScopeTree(scope *symbols.Scope, name string) *symbols.Symbol {
	matches := collectInScopeTree(scope, name)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

// scopeSymbolFor returns the symbol in scope's tree declared by decl, or nil.
func scopeSymbolFor(scope *symbols.Scope, decl ast.Node) *symbols.Symbol {
	if scope == nil || decl == nil {
		return nil
	}
	for _, sym := range scope.Members() {
		if sym.Decl == decl {
			return sym
		}
	}
	for _, child := range scope.Children() {
		if sym := scopeSymbolFor(child, decl); sym != nil {
			return sym
		}
	}
	return nil
}
