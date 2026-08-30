package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
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
	model, resolver := ctx.Model(), ctx.Resolver()
	if !docplan.IsDocumentDefinition(idx, model, sym) {
		return "", fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", notationName(fqn))
	}
	plan, err := docplan.Compile(idx, model, resolver, sym)
	if err != nil {
		return "", err
	}
	document, err := docir.Evaluate(plan,
		queryexec.Context{Index: idx, Resolver: resolver, Model: model},
		queryexec.Options{},
		s.sessionSourceText())
	if err != nil {
		return "", err
	}
	return docrender.Markdown(document)
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
