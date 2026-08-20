package edit

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (m Model) deleteSplices(i int, op Operation) ([]splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return nil, err
	}
	referrers := m.referringSymbols(sym)
	if len(referrers) > 0 && !op.Cascade {
		referring := make([]string, 0, len(referrers))
		for _, referrer := range referrers {
			referring = append(referring, m.Index.GetFQN(referrer))
		}
		return nil, &Error{
			Failure:        FailureDeleteReferenced,
			OperationIndex: i,
			Referring:      referring,
			Message: op.Target + " is referenced by " + strings.Join(referring, ", ") +
				"; delete it with cascade to remove those declarations",
		}
	}
	targets := []*symbols.Symbol{sym}
	if op.Cascade {
		targets = append(targets, referrers...)
	}
	out := make([]splice, 0, len(targets))
	for _, target := range targets {
		span := m.deleteSpan(target)
		out = append(out, splice{
			span: span, opIndex: i, target: m.Index.GetFQN(target),
		})
	}
	return out, nil
}

func (m Model) referringSymbols(target *symbols.Symbol) []*symbols.Symbol {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if rootScope == nil {
		return nil
	}
	r := m.resolver()
	seen := map[string]bool{}
	var out []*symbols.Symbol
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for i, part := range ref.QN.Parts {
			seg, ok := r.PartSymbol(ref.QN, i)
			if !ok || !symbols.SameElement(seg, target) ||
				part.Span == target.NameSpan {
				continue
			}
			referrer := m.symbolContaining(part.Span.Offset, target)
			if referrer == nil || symbols.SameElement(referrer, target) {
				continue
			}
			if target.DeclSpan.Offset <= referrer.DeclSpan.Offset &&
				target.DeclSpan.End() >= referrer.DeclSpan.End() {
				continue
			}
			fqn := m.Index.GetFQN(referrer)
			if fqn != "" && !seen[fqn] {
				seen[fqn] = true
				out = append(out, referrer)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return m.Index.GetFQN(out[i]) < m.Index.GetFQN(out[j])
	})
	return out
}

func (m Model) symbolContaining(offset int, target *symbols.Symbol) *symbols.Symbol {
	var found *symbols.Symbol
	for _, fqn := range m.Index.FQNs() {
		for _, candidate := range m.Index.LookupQualified(fqn) {
			if candidate == nil || candidate.DocName != m.Source.Name() ||
				symbols.SameElement(candidate, target) {
				continue
			}
			span := candidate.DeclSpan
			if span.Len == 0 || offset < span.Offset || offset >= span.End() {
				continue
			}
			if found == nil || span.Len < found.DeclSpan.Len {
				found = candidate
			}
		}
	}
	return found
}

func (m Model) deleteSpan(sym *symbols.Symbol) source.Span {
	span := sym.DeclSpan
	if span.Len == 0 && sym.Decl != nil {
		span = sym.Decl.Span()
	}
	content := m.Source.Bytes()
	start := span.Offset
	end := m.declarationEnd(sym)
	if end <= start || end > len(content) {
		end = span.End()
	}
	lineStart := start
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}
	// A declaration on its own line owns that line, including its newline.
	if onlyWhitespace(content[lineStart:start]) && onlyWhitespace(content[end:lineEnd]) {
		if lineEnd < len(content) {
			lineEnd++
		}
		start = lineStart
		end = lineEnd
	}
	// Include contiguous leading comment lines, but stop at a blank line so a
	// neighboring declaration's comment ownership is never consumed.
	for {
		cursor := start - 1
		if cursor < 0 {
			break
		}
		if content[cursor] == '\n' {
			cursor--
		}
		if cursor < 0 {
			break
		}
		prevLineEnd := cursor + 1
		prevLineStart := prevLineEnd
		for prevLineStart > 0 && content[prevLineStart-1] != '\n' {
			prevLineStart--
		}
		line := strings.TrimSpace(string(content[prevLineStart:prevLineEnd]))
		if line == "" {
			// A blank line immediately before a leading comment belongs to
			// that comment block and is removed with it.
			start = prevLineStart
			continue
		}
		if len(line) < 2 || (!strings.HasPrefix(line, "//") &&
			!strings.HasPrefix(line, "/*") && !strings.HasPrefix(line, "*")) {
			break
		}
		start = prevLineStart
	}
	return source.Span{Offset: start, Len: end - start}
}

func (m Model) declarationEnd(sym *symbols.Symbol) int {
	start := sym.NameSpan.End()
	if start <= 0 {
		start = sym.DeclSpan.Offset
	}
	depth := 0
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.End() <= start {
			continue
		}
		switch tok.Kind {
		case lexer.LBrace:
			depth++
		case lexer.RBrace:
			if depth > 0 {
				depth--
				if depth == 0 {
					return tok.Span.End()
				}
			}
		case lexer.Semicolon:
			if depth == 0 {
				return tok.Span.End()
			}
		}
	}
	return 0
}

func onlyWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}
