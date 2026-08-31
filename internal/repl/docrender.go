package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// renderDocumentUsage is what %render-document accepts: a document's name.
const renderDocumentUsage = "usage: %render-document <name>"

// RenderDocumentMarkdown compiles the named document definition, evaluates its
// queries against the session's model, and renders the result as Markdown. A
// document binds its queries' parameters in the model, so the invocation is
// the document's name alone.
func (s *Session) RenderDocumentMarkdown(invocation string) (string, error) {
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 {
		return "", fmt.Errorf("a document to render must be named")
	}
	if len(fields) > 1 {
		return "", fmt.Errorf("a document binds its queries' parameters in the model; unexpected argument %q", fields[1])
	}
	sym, fqn, err := s.lookupSymbol(fields[0])
	if err != nil {
		return "", err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return "", fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	sem, resolver := ctx.Model(), ctx.Resolver()
	if !docplan.IsDocumentDefinition(idx, sem, sym) {
		return "", fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", notationName(fqn))
	}
	plan, err := docplan.Compile(idx, sem, resolver, sym)
	if err != nil {
		return "", err
	}
	document, err := docir.EvaluateLinked(plan,
		model.SiblingDocumentPlans(idx, sem, resolver, sym),
		queryexec.Context{Index: idx, Resolver: resolver, Model: sem},
		queryexec.Options{},
		s.sessionSourceText())
	if err != nil {
		return "", err
	}
	return docrender.Markdown(document)
}

// RenderedDocument is one document of a rendered multi-document set.
type RenderedDocument struct {
	Name     string
	FileName string
	Markdown string
}

// RenderDocumentSetMarkdown compiles every document definition the session's
// model declares, evaluates them together, and renders each as Markdown with
// its deterministic file name, so cross-document references link on disk.
func (s *Session) RenderDocumentSetMarkdown() ([]RenderedDocument, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	sem, resolver := ctx.Model(), ctx.Resolver()
	syms := s.symbolsInLoadOrder(func(scope *symbols.Scope) []*symbols.Symbol {
		return model.DeclaredDocumentDefinitions(idx, sem, scope)
	})
	sort.SliceStable(syms, func(i, j int) bool {
		return symbols.FQNOf(syms[i]) < symbols.FQNOf(syms[j])
	})
	plans := make([]*docplan.Plan, 0, len(syms))
	names := map[string]bool{}
	// Filenames are compared case-folded so the set stays writable on
	// case-insensitive filesystems.
	files := map[string]string{}
	for _, sym := range syms {
		plan, err := docplan.Compile(idx, sem, resolver, sym)
		if err != nil {
			return nil, err
		}
		if names[plan.Name()] {
			return nil, fmt.Errorf("%s names more than one document; rename one so the name is unambiguous", notationName(plan.Name()))
		}
		names[plan.Name()] = true
		file := docrender.DocumentFileName(plan.Name())
		if other, ok := files[strings.ToLower(file)]; ok {
			return nil, fmt.Errorf("%s and %s render to file names that differ only by letter case; rename one so both files can coexist on a case-insensitive filesystem", notationName(other), notationName(plan.Name()))
		}
		files[strings.ToLower(file)] = plan.Name()
		plans = append(plans, plan)
	}
	documents, err := docir.EvaluateSet(plans,
		queryexec.Context{Index: idx, Resolver: resolver, Model: sem},
		queryexec.Options{},
		s.sessionSourceText())
	if err != nil {
		return nil, err
	}
	out := make([]RenderedDocument, 0, len(documents))
	for _, document := range documents {
		markdown, err := docrender.Markdown(document)
		if err != nil {
			return nil, err
		}
		out = append(out, RenderedDocument{
			Name:     document.Name(),
			FileName: docrender.DocumentFileName(document.Name()),
			Markdown: markdown,
		})
	}
	return out, nil
}

// doRenderDocument carries out %render-document, printing the rendered
// Markdown or reporting a document that could not be rendered.
func (s *Session) doRenderDocument(invocation string) ([]string, bool, error) {
	if strings.TrimSpace(invocation) == "" {
		return []string{renderDocumentUsage}, false, nil
	}
	markdown, err := s.RenderDocumentMarkdown(invocation)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return strings.Split(strings.TrimRight(markdown, "\n"), "\n"), false, nil
}
