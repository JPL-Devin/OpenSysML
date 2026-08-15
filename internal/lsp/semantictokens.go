package lsp

import (
	"bytes"
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/highlight"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// semanticTokensProvider is the semanticTokensProvider capability. The protocol
// library's options type predates the legend, so the shape LSP 3.16 defines is
// declared here; delta requests are not supported, so full is plain true.
type semanticTokensProvider struct {
	Legend protocol.SemanticTokensLegend `json:"legend"`
	Full   bool                          `json:"full"`
	Range  bool                          `json:"range"`
}

// semanticTokensLegend is the legend the server advertises: the token types and
// modifiers highlight emits, in the order an encoded token indexes them by.
func semanticTokensLegend() protocol.SemanticTokensLegend {
	classes := highlight.Classes()
	types := make([]protocol.SemanticTokenTypes, len(classes))
	for i, c := range classes {
		types[i] = protocol.SemanticTokenTypes(c.String())
	}
	mods := highlight.Modifiers()
	modifiers := make([]protocol.SemanticTokenModifiers, len(mods))
	for i, m := range mods {
		modifiers[i] = protocol.SemanticTokenModifiers(m.String())
	}
	return protocol.SemanticTokensLegend{TokenTypes: types, TokenModifiers: modifiers}
}

// SemanticTokensFull answers the semantic tokens of a whole document.
func (s *Server) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return &protocol.SemanticTokens{}, nil
	}
	return &protocol.SemanticTokens{
		Data: encodeTokens(doc.Content, s.ws.HighlightTokens(name)),
	}, nil
}

// SemanticTokensRange answers the semantic tokens overlapping a range, which is
// the full document's tokens filtered: highlighting a name needs the whole
// document resolved either way.
func (s *Server) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return &protocol.SemanticTokens{}, nil
	}
	want := rangeToSpan(doc.Content, params.Range)
	var in []highlight.Token
	for _, tok := range s.ws.HighlightTokens(name) {
		if tok.Span.Offset < want.End() && tok.Span.End() > want.Offset {
			in = append(in, tok)
		}
	}
	return &protocol.SemanticTokens{Data: encodeTokens(doc.Content, in)}, nil
}

// encodeTokens encodes tokens in the protocol's relative form: line delta,
// character delta, length, type index and modifier bitset, all in UTF-16 units.
// A token spanning lines is split per line, which the encoding requires.
func encodeTokens(content []byte, toks []highlight.Token) []uint32 {
	data := make([]uint32, 0, 5*len(toks))
	prevLine, prevChar := uint32(0), uint32(0)
	for _, tok := range toks {
		for _, part := range splitLines(content, tok.Span) {
			pos := offsetToPosition(content, part.Offset)
			length := utf16Len(content[part.Offset:part.End()])
			deltaLine := pos.Line - prevLine
			deltaChar := pos.Character
			if deltaLine == 0 {
				deltaChar = pos.Character - prevChar
			}
			data = append(data, deltaLine, deltaChar, uint32Clamp(length),
				uint32Clamp(int(tok.Class)), uint32(tok.Modifiers))
			prevLine, prevChar = pos.Line, pos.Character
		}
	}
	return data
}

// splitLines cuts a span into one span per line it covers, dropping the line
// terminators and any resulting empty piece.
func splitLines(content []byte, sp source.Span) []source.Span {
	if sp.Len <= 0 || sp.Offset < 0 || sp.End() > len(content) {
		return nil
	}
	var out []source.Span
	start := sp.Offset
	for start < sp.End() {
		end := sp.End()
		if nl := bytes.IndexByte(content[start:end], '\n'); nl >= 0 {
			end = start + nl
		}
		trimmed := end
		if trimmed > start && content[trimmed-1] == '\r' {
			trimmed--
		}
		if trimmed > start {
			out = append(out, source.Span{Offset: start, Len: trimmed - start})
		}
		start = end + 1
	}
	return out
}
