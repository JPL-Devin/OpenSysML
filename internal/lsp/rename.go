package lsp

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// PrepareRename reports the range the client should offer for editing, and
// rejects positions that name nothing renameable before the user types.
func (s *Server) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (*protocol.Range, error) {
	name := uriToName(params.TextDocument.URI)
	_, span, err := s.renameTarget(name, params.Position)
	if err != nil {
		return nil, err
	}
	rng := spanToRange(s.ws.Document(name).Content, span)
	return &rng, nil
}

// Rename renames the declaration under the cursor and every reference to it
// across the workspace's documents, including the qualifier positions of
// multi-segment names (`A::B` when renaming A).
func (s *Server) Rename(ctx context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	name := uriToName(params.TextDocument.URI)
	target, _, err := s.renameTarget(name, params.Position)
	if err != nil {
		return nil, err
	}
	if err := validateNewName(params.NewName); err != nil {
		return nil, err
	}

	changes := map[protocol.DocumentURI][]protocol.TextEdit{}
	// One name occurrence is edited once however many times it is collected: a
	// shorthand redefinition (`part redefines x;`) is both the declaration and a
	// reference at the same span, and clients reject overlapping edits.
	edited := map[protocol.DocumentURI]map[source.Span]bool{}
	addEdit := func(docName string, content []byte, span source.Span) {
		uri := nameToURI(docName)
		if edited[uri] == nil {
			edited[uri] = map[source.Span]bool{}
		}
		if edited[uri][span] {
			return
		}
		edited[uri][span] = true
		changes[uri] = append(changes[uri], protocol.TextEdit{
			Range:   spanToRange(content, span),
			NewText: params.NewName,
		})
	}

	// The declaration itself.
	declDoc := s.ws.Document(target.DocName)
	addEdit(target.DocName, declDoc.Content, target.NameSpan)

	// Every reference, in every document, at whichever segment denotes target.
	for _, docName := range s.ws.DocumentNames() {
		doc := s.ws.Document(docName)
		if doc == nil || doc.Scope == nil {
			continue
		}
		for _, ref := range collectRefs(doc.AST, doc.Scope) {
			segs := s.ws.ResolveReferenceSegmentsInDoc(docName, ref)
			for i, seg := range segs {
				if sameSymbol(seg, target) {
					addEdit(docName, doc.Content, ref.QN.Parts[i].Span)
				}
			}
		}
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

// renameTarget returns the symbol named at pos and the span of the name as
// written there (a declaration's identifier, or one segment of a reference).
func (s *Server) renameTarget(name string, pos protocol.Position) (*symbols.Symbol, source.Span, error) {
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, source.Span{}, fmt.Errorf("no document %q", name)
	}
	offset := positionToOffset(doc.Content, pos)

	// On a reference: rename the symbol the containing segment denotes, so
	// renaming from the `A` of `A::B` renames A, not B.
	if ref := refAtOffset(collectRefs(doc.AST, doc.Scope), offset); ref != nil {
		segs := s.ws.ResolveReferenceSegmentsInDoc(name, *ref)
		for i, part := range ref.QN.Parts {
			if offset < part.Span.Offset || offset >= part.Span.End() {
				continue
			}
			if i < len(segs) && segs[i] != nil {
				return s.renameable(segs[i], part.Span)
			}
			return nil, source.Span{}, fmt.Errorf("cannot rename %q: unresolved", part.Text)
		}
	}

	// On a declaration: the cursor must be on the declared identifier itself,
	// not merely somewhere inside the declaration's body.
	if sym := symbolAtOffset(doc.Scope, offset); sym != nil {
		if sp := sym.NameSpan; sp.Len > 0 && offset >= sp.Offset && offset < sp.End() {
			return s.renameable(sym, sp)
		}
	}
	return nil, source.Span{}, fmt.Errorf("no renameable name at this position")
}

// renameable rejects symbols whose declaration this server cannot edit.
func (s *Server) renameable(sym *symbols.Symbol, span source.Span) (*symbols.Symbol, source.Span, error) {
	if sym.NameSpan.Len == 0 {
		return nil, source.Span{}, fmt.Errorf("cannot rename %q: no declared name", sym.Name)
	}
	if s.ws.Document(sym.DocName) == nil {
		// Standard-library and other out-of-workspace declarations: renaming
		// the references alone would break the model.
		return nil, source.Span{}, fmt.Errorf("cannot rename %q: declared outside the workspace", sym.Name)
	}
	return sym, span, nil
}

// sameSymbol compares symbols across documents. Resolution inside the document
// under the cursor yields Document-tree pointers, while resolution through the
// global index yields the index's own, so identity falls back to the declaring
// document and declaration span.
func sameSymbol(a, b *symbols.Symbol) bool {
	switch {
	case a == nil || b == nil:
		return false
	case a == b:
		return true
	case a.DocName == "" || a.DocName != b.DocName:
		return false
	}
	return a.DeclSpan == b.DeclSpan
}

// validateNewName rejects names that would not lex as the identifier they are
// meant to replace.
func validateNewName(name string) error {
	if name == "" {
		return fmt.Errorf("new name is empty")
	}
	if lexer.IsKeyword(name) {
		return fmt.Errorf("%q is a keyword", name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		ok := c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && c >= '0' && c <= '9')
		if !ok {
			return fmt.Errorf("%q is not a valid identifier", name)
		}
	}
	return nil
}
