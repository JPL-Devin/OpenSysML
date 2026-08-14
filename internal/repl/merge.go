package repl

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// nsDecl is a namespace declaration with a body, located in the text it was
// parsed from: the members it holds and the offset of the brace that closes it,
// which is where an added member goes.
type nsDecl struct {
	name    string
	desc    string // "package P", as the summary renders it
	members []ast.Node
	brace   int
}

// edit replaces src[start:end] with text. Edits never overlap: each one covers a
// distinct member, or is an insertion at a body's closing brace.
type edit struct {
	start, end int
	text       string
}

// mergeSubmission folds a resubmitted namespace declaration into the same-named
// one already in the buffer, so re-typing `package P { ... }` to add a member
// adds to it rather than replacing its body. On success it removes the snippet it
// merged with (the returned text supersedes both that snippet and src) and
// reports what the merge itself replaced, which is only the members the new body
// redeclares.
//
// Merging applies to the plain case a REPL user types: a submission that is one
// namespace declaration with at least one member. An empty body still replaces,
// which is how a namespace is emptied.
func (s *Session) mergeSubmission(src string, root *ast.RootNamespace) (string, dropReport, bool) {
	newDecl, ok := soleNamespace(src, root)
	if !ok || len(newDecl.members) == 0 {
		return "", dropReport{}, false
	}
	for i, sn := range s.snippets {
		oldDecl, ok := namedNamespace(sn.src, newDecl.name)
		if !ok {
			continue
		}
		edits, replaced := mergeEdits(sn.src, oldDecl, src, newDecl)
		merged := applyEdits(sn.src, edits)
		s.snippets = append(s.snippets[:i:i], s.snippets[i+1:]...)
		return merged, dropReport{merged: true, decl: newDecl.desc, lost: replaced}, true
	}
	return "", dropReport{}, false
}

// soleNamespace returns the namespace declaration a submission consists of,
// reporting false for anything else — a submission that declares more than one
// thing is replaced wholesale rather than merged, since its text cannot be split
// between the buffer's snippets without losing what sits between declarations.
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
	d.brace = brace
	d.desc = renderMember(member)
	return d, true
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
		span := om.Span()
		edits = append(edits, edit{start: span.Offset, end: span.Offset + span.Len})
		replaced = append(replaced, renderMember(om))
	}
	var added []string
	for i, nm := range newDecl.members {
		if folded[i] {
			continue
		}
		if text := trimmedText(newSrc, nm); text != "" {
			added = append(added, text)
		}
	}
	if len(added) > 0 {
		edits = append(edits, edit{
			start: oldDecl.brace,
			end:   oldDecl.brace,
			text:  insertion(oldSrc, oldDecl.brace, added),
		})
	}
	return edits, replaced
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

// insertion renders members to add just before a body's closing brace, matching
// the layout of the body they join: one indented line each in a multi-line body,
// space-separated in a one-line body.
func insertion(src string, brace int, members []string) string {
	lineStart := strings.LastIndexByte(src[:brace], '\n') + 1
	prefix := src[lineStart:brace]
	if strings.TrimSpace(prefix) != "" {
		return strings.Join(members, " ") + " "
	}
	var b strings.Builder
	for _, m := range members {
		b.WriteString(prefix)
		b.WriteString("\t")
		b.WriteString(m)
		b.WriteString("\n")
	}
	b.WriteString(prefix)
	return b.String()
}

// applyEdits rewrites src with the given edits applied.
func applyEdits(src string, edits []edit) string {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var b strings.Builder
	pos := 0
	for _, e := range edits {
		if e.start < pos || e.end > len(src) {
			continue
		}
		b.WriteString(src[pos:e.start])
		b.WriteString(e.text)
		pos = e.end
	}
	b.WriteString(src[pos:])
	return b.String()
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
