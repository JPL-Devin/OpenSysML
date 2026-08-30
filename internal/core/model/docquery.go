package model

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DocumentDefinition is one native document definition the workspace holds: a
// part def specializing DocumentQueries::Document, and the file declaring it.
type DocumentDefinition struct {
	FQN string
	Doc string
}

// DocumentDefinitions lists the document definitions declared across the
// workspace's own documents, in qualified-name order.
func (w *Workspace) DocumentDefinitions() []DocumentDefinition {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, sem := w.newResolver()
	out := []DocumentDefinition{}
	for name := range w.docs {
		walkScope(w.index.DocumentRoot(name), func(sym *symbols.Symbol) {
			if docplan.IsDocumentDefinition(w.index, sem, sym) {
				out = append(out, DocumentDefinition{FQN: notationFQN(w.index, sym), Doc: name})
			}
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FQN < out[j].FQN })
	return out
}

// RenderDocumentMarkdown compiles the named document definition, evaluates its
// queries against the workspace model, and renders the result as Markdown.
func (w *Workspace) RenderDocumentMarkdown(fqn string) (string, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, sem := w.newResolver()
	matches := symbols.PreferDeclared(w.index.LookupQualified(fqn))
	if len(matches) == 0 {
		return "", fmt.Errorf("no element named %s", fqn)
	}
	sym := matches[0]
	if !docplan.IsDocumentDefinition(w.index, sem, sym) {
		return "", fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", fqn)
	}
	plan, err := docplan.Compile(w.index, sem, resolver, sym)
	if err != nil {
		return "", err
	}
	document, err := docir.Evaluate(plan,
		queryexec.Context{Index: w.index, Resolver: resolver, Model: sem},
		queryexec.Options{})
	if err != nil {
		return "", err
	}
	return docrender.Markdown(document)
}

// QueryBindingParameter resolves the parameter a document query binding names:
// for an `in` member of a calc usage typed by a query definition, the matching
// parameter declaration on that definition.
func (w *Workspace) QueryBindingParameter(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if sym == nil || sym.OwnerScope == nil {
		return nil, false
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Direction != ast.DirIn {
		return nil, false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, sem := w.newResolver()
	target := docplan.QueryTarget(w.index, sem, resolver, sym.OwnerScope.Owner())
	if target == nil {
		return nil, false
	}
	for _, member := range w.memberSymbolsLocked(resolver, sem, sym.OwnerScope, target) {
		if member == nil || member.Name != sym.Name {
			continue
		}
		if md, ok := member.Decl.(*ast.Usage); ok && md.Direction == ast.DirIn {
			return member, true
		}
	}
	return nil, false
}

// QueryUsageParameters lists the `in` parameters of the query definition a calc
// usage is typed by; nil when the usage is not typed by one.
func (w *Workspace) QueryUsageParameters(usage *symbols.Symbol) []*symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, sem := w.newResolver()
	target := docplan.QueryTarget(w.index, sem, resolver, usage)
	if target == nil {
		return nil
	}
	scope := usage.OwnerScope
	if usage.Scope != nil {
		scope = usage.Scope
	}
	var out []*symbols.Symbol
	for _, member := range w.memberSymbolsLocked(resolver, sem, scope, target) {
		if member == nil {
			continue
		}
		if md, ok := member.Decl.(*ast.Usage); ok && md.Direction == ast.DirIn {
			out = append(out, member)
		}
	}
	return out
}

// IsDocumentDefinition reports whether sym is a native document definition: a
// part def specializing DocumentQueries::Document.
func (w *Workspace) IsDocumentDefinition(sym *symbols.Symbol) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, sem := w.newResolver()
	return docplan.IsDocumentDefinition(w.index, sem, sym)
}

// QueryDefinitions filters syms to the query definitions among them: the calc
// defs specializing DocumentQueries::Query.
func (w *Workspace) QueryDefinitions(syms []*symbols.Symbol) []*symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, sem := w.newResolver()
	var out []*symbols.Symbol
	for _, sym := range syms {
		if queryplan.IsQueryDefinition(w.index, sem, sym) {
			out = append(out, sym)
		}
	}
	return out
}
