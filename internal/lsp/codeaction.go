package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// CodeAction answers the quick fixes for the diagnostics in a range: the fixes
// the pass that reported a diagnostic attached to it, as workspace edits.
func (s *Server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return nil, nil
	}
	if !wantsQuickFix(params.Context.Only) {
		return nil, nil
	}
	want := rangeToSpan(doc.Content, params.Range)
	uri := nameToURI(name)
	out := []protocol.CodeAction{}
	for _, diag := range s.ws.Diagnostics(name) {
		if len(diag.Fixes) == 0 || !overlaps(diag.Span, want) {
			continue
		}
		reported := protocol.Diagnostic{
			Range:    spanToRange(doc.Content, diag.Span),
			Severity: protocol.DiagnosticSeverity(int(diag.Severity) + 1),
			Message:  diag.Message,
			Code:     diag.Code,
			Source:   diag.Source,
		}
		for _, fix := range diag.Fixes {
			out = append(out, protocol.CodeAction{
				Title:       fix.Title,
				Kind:        protocol.QuickFix,
				Diagnostics: []protocol.Diagnostic{reported},
				IsPreferred: fix.Preferred,
				Edit:        workspaceEdit(uri, doc.Content, fix.Edits),
			})
		}
	}
	return out, nil
}

// wantsQuickFix reports whether a client filtering by kind asks for quick fixes;
// an unfiltered request asks for everything.
func wantsQuickFix(only []protocol.CodeActionKind) bool {
	if len(only) == 0 {
		return true
	}
	for _, kind := range only {
		if kind == protocol.QuickFix || kind == "" {
			return true
		}
	}
	return false
}

// overlaps reports whether two spans share any position, an empty span counting
// where it sits: a cursor on a diagnostic asks for its fixes.
func overlaps(a, b source.Span) bool {
	if a.Len == 0 || b.Len == 0 {
		return a.Offset <= b.End() && b.Offset <= a.End()
	}
	return a.Offset < b.End() && b.Offset < a.End()
}

// workspaceEdit renders a fix's edits as an edit of the document it applies to.
func workspaceEdit(uri protocol.DocumentURI, content []byte, edits []quickfix.Edit) *protocol.WorkspaceEdit {
	out := make([]protocol.TextEdit, 0, len(edits))
	for _, edit := range edits {
		span, text := edit.Render(content)
		out = append(out, protocol.TextEdit{
			Range:   spanToRange(content, span),
			NewText: text,
		})
	}
	return &protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentURI][]protocol.TextEdit{uri: out},
	}
}
