package repl

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// nsDecl is a namespace declaration with a body, located in the text it was
// parsed from: where it starts, its members, and the brace an addition goes at.
type nsDecl struct {
	name    string
	desc    string // "package P", as the summary renders it
	members []ast.Node
	start   int
	brace   int
}

// edit replaces src[start:end] with text. Edits never overlap: each one covers a
// distinct member, or is an insertion at a body's closing brace.
type edit struct {
	start, end int
	text       string
}

// mergeSubmission folds a resubmitted namespace declaration into the same-named
// one in the buffer, so re-typing `package P { ... }` adds to it instead of
// replacing its body. It removes the snippet it merged with, since the returned
// text supersedes it, and reports the members the new body redeclares. An empty
// body is not merged: that is how a namespace is emptied.
//
// The returned spans locate the submitted text inside the merged result, so a
// report still covers what was typed rather than the whole absorbed snippet.
func (s *Session) mergeSubmission(src string, root *ast.RootNamespace, comments string) (string, []source.Span, dropReport, bool) {
	newDecl, ok := soleNamespace(src, root)
	if !ok || len(newDecl.members) == 0 {
		return "", nil, dropReport{}, false
	}
	for i, sn := range s.snippets {
		oldDecl, ok := namedNamespace(sn.src, newDecl.name)
		if !ok {
			continue
		}
		edits, replaced := mergeEdits(sn.src, oldDecl, src, newDecl)
		// Comments typed above the addition document the declaration, so they go
		// above it rather than at the tail of the buffer.
		if comments != "" {
			edits = append(edits, edit{start: oldDecl.start, end: oldDecl.start, text: comments})
		}
		merged, own := applyEdits(sn.src, edits)
		s.snippets = append(s.snippets[:i:i], s.snippets[i+1:]...)
		return merged, own, dropReport{merged: true, decl: newDecl.desc, lost: replaced}, true
	}
	return "", nil, dropReport{}, false
}

// soleNamespace returns the namespace declaration a submission consists of. A
// submission declaring more than one thing is replaced wholesale, since its text
// cannot be split between snippets without losing what sits between them.
func soleNamespace(src string, root *ast.RootNamespace) (nsDecl, bool) {
	if root == nil || len(root.Members) != 1 {
		return nsDecl{}, false
	}
	return namespaceDeclOf(src, root.Members[0])
}

// namedNamespace finds the namespace declaration of the given name in an
// accepted snippet. It reports false unless exactly one member declares that
// name, so an ambiguous snippet keeps the replacement behavior.
func namedNamespace(src, name string) (nsDecl, bool) {
	root := parser.New(source.New(docName, []byte(src))).ParseFile()
	if root == nil {
		return nsDecl{}, false
	}
	var found ast.Node
	for _, m := range root.Members {
		if memberName(m) != name {
			continue
		}
		if found != nil {
			return nsDecl{}, false
		}
		found = m
	}
	if found == nil {
		return nsDecl{}, false
	}
	return namespaceDeclOf(src, found)
}

// namespaceDeclOf describes a member as a namespace declaration with a body,
// reporting false for anything else — a usage, a definition, a `package P;`
// without a body, or a body whose brace the parser never saw.
func namespaceDeclOf(src string, member ast.Node) (nsDecl, bool) {
	node := member
	if mem, ok := member.(*ast.Membership); ok {
		node = mem.Member
	}
	var d nsDecl
	switch n := node.(type) {
	case *ast.Package:
		if !n.HasBody {
			return d, false
		}
		d.name, d.members = n.Ident.Name, n.Members
	case *ast.Namespace:
		if !n.HasBody {
			return d, false
		}
		d.name, d.members = n.Ident.Name, n.Members
	default:
		return d, false
	}
	if d.name == "" {
		return d, false
	}
	brace, ok := closingBrace(src, node)
	if !ok {
		return d, false
	}
	d.start, d.brace = member.Span().Offset, brace
	d.desc = renderMember(member)
	return d, true
}

// bodyMembers returns the members of a namespace member's body. Unlike
// namespaceDeclOf it does not need the body to be terminated, so it can also
// report what an unparseable re-declaration dropped.
func bodyMembers(member ast.Node) ([]ast.Node, bool) {
	node := member
	if mem, ok := member.(*ast.Membership); ok {
		node = mem.Member
	}
	switch n := node.(type) {
	case *ast.Package:
		return n.Members, n.HasBody
	case *ast.Namespace:
		return n.Members, n.HasBody
	}
	return nil, false
}

// closingBrace returns the offset of the brace that closes a declaration's body.
// A span that does not end in one belongs to an unterminated body, which is not
// safe to edit.
func closingBrace(src string, node ast.Node) (int, bool) {
	end := node.Span().Offset + node.Span().Len
	if end > len(src) {
		end = len(src)
	}
	for end > 0 && isSpaceByte(src[end-1]) {
		end--
	}
	if end == 0 || src[end-1] != '}' {
		return 0, false
	}
	return end - 1, true
}

// mergeEdits computes the edits on oldSrc that fold newDecl's members into
// oldDecl, plus a description of each member the new body replaces. A member
// redeclared as a namespace on both sides is merged in turn, so adding to a
// nested package keeps the nested members too.
func mergeEdits(oldSrc string, oldDecl nsDecl, newSrc string, newDecl nsDecl) ([]edit, []string) {
	var (
		edits    []edit
		replaced []string
		folded   = make(map[int]bool, len(newDecl.members))
	)
	for _, om := range oldDecl.members {
		name := memberName(om)
		if name == "" {
			continue
		}
		i, nm := findNamed(newDecl.members, name)
		if nm == nil {
			continue
		}
		if oldSub, ok := namespaceDeclOf(oldSrc, om); ok {
			if newSub, ok := namespaceDeclOf(newSrc, nm); ok {
				subEdits, subReplaced := mergeEdits(oldSrc, oldSub, newSrc, newSub)
				edits = append(edits, subEdits...)
				replaced = append(replaced, subReplaced...)
				folded[i] = true
				continue
			}
		}
		// The new member is not a body to add to, so it supersedes the old one:
		// drop the old text and let the new member be added below.
		start, end := memberCut(oldSrc, om)
		edits = append(edits, edit{start: start, end: end})
		replaced = append(replaced, renderMember(om))
	}
	var added []string
	for i, nm := range newDecl.members {
		if folded[i] {
			continue
		}
		text := trimmedText(newSrc, nm)
		if text == "" {
			continue
		}
		// A member declaring no name (an import, a comment) matches only on its
		// text, so an identical one in the body is it re-typed, not a second copy.
		if memberName(nm) == "" && hasText(oldSrc, oldDecl.members, text) {
			continue
		}
		added = append(added, text)
	}
	if len(added) > 0 {
		edits = append(edits, insertion(oldSrc, oldDecl.brace, added))
	}
	return edits, replaced
}

// memberCut is the range to delete to remove a member: its text (its span runs
// on to the next token, so the trimmed text bounds it) plus the line it owns, so
// no blank line is left behind and the closing brace keeps its indentation.
func memberCut(src string, member ast.Node) (start, end int) {
	start = member.Span().Offset
	end = start + len(trimmedText(src, member))
	lineStart := strings.LastIndexByte(src[:start], '\n') + 1
	if strings.TrimSpace(src[lineStart:start]) != "" {
		return start, end
	}
	rest := src[end:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 || strings.TrimSpace(rest[:nl]) != "" {
		return start, end
	}
	return lineStart, end + nl + 1
}

// hasText reports whether one of the members is the given source text.
func hasText(src string, members []ast.Node, text string) bool {
	for _, m := range members {
		if trimmedText(src, m) == text {
			return true
		}
	}
	return false
}

// findNamed returns the first member declaring name, and its index. Only the
// first is taken: a submission that declares one name twice keeps both members,
// so it still reports a duplicate declaration.
func findNamed(members []ast.Node, name string) (int, ast.Node) {
	for i, m := range members {
		if memberName(m) == name {
			return i, m
		}
	}
	return -1, nil
}

// insertion is the edit adding members to a body, laid out like it: one indented
// line each in a multi-line body, space-separated in a one-line body. Lines go
// above a closing brace that owns its line, so that brace keeps its indentation.
func insertion(src string, brace int, members []string) edit {
	lineStart := strings.LastIndexByte(src[:brace], '\n') + 1
	prefix := src[lineStart:brace]
	if strings.TrimSpace(prefix) != "" {
		return edit{start: brace, end: brace, text: strings.Join(members, " ") + " "}
	}
	var b strings.Builder
	for _, m := range members {
		b.WriteString(prefix)
		b.WriteString("\t")
		b.WriteString(m)
		b.WriteString("\n")
	}
	return edit{start: lineStart, end: lineStart, text: b.String()}
}

// applyEdits rewrites src with the given edits applied, and returns where each
// edit's text landed in the result. An insertion inside an already-rewritten
// range is written at the point reached rather than dropped, so no added text is
// ever silently lost.
func applyEdits(src string, edits []edit) (string, []source.Span) {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var (
		b     strings.Builder
		added []source.Span
		pos   int
	)
	for _, e := range edits {
		if e.end > len(src) || (e.start < pos && e.start != e.end) {
			continue
		}
		if e.start > pos {
			b.WriteString(src[pos:e.start])
			pos = e.start
		}
		if e.text != "" {
			added = append(added, source.Span{Offset: b.Len(), Len: len(e.text)})
			b.WriteString(e.text)
		}
		if e.end > pos {
			pos = e.end
		}
	}
	b.WriteString(src[pos:])
	return b.String(), added
}

// trimmedText returns a node's source text without the whitespace its span runs
// on to the next member.
func trimmedText(src string, node ast.Node) string {
	span := node.Span()
	start, end := span.Offset, span.Offset+span.Len
	if end > len(src) {
		end = len(src)
	}
	if start > end {
		return ""
	}
	return strings.TrimRight(src[start:end], " \t\r\n")
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
