package view

import (
	"fmt"
	"strings"
)

// Text is the human-readable form of a rendering: a header saying what was
// rendered and how, the nodes as an indented tree, the edges beneath it, and
// what the rendering could not represent. It is what the REPL prints.
func (r *Rendering) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s rendering", r.View, r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	} else {
		b.WriteString(" (the view states no rendering; a tree is the default)")
	}
	b.WriteString("\n")
	if r.Empty() {
		b.WriteString("\n" + r.emptyReason() + "\n")
		writeNotices(&b, r.Notices)
		return b.String()
	}
	b.WriteString("\n")
	labels := map[string]string{}
	for _, root := range r.Roots {
		writeNodeText(&b, root, 0, labels)
	}
	if len(r.Edges) > 0 {
		fmt.Fprintf(&b, "\n%s:\n", edgeSectionName(r.Kind))
		for _, edge := range r.Edges {
			line := fmt.Sprintf("  %s %s %s", labels[edge.From], edgeArrow(edge.Kind), labels[edge.To])
			if edge.Label != "" {
				line += ": " + edge.Label
			}
			b.WriteString(line + "\n")
		}
	}
	writeNotices(&b, r.Notices)
	return b.String()
}

// emptyReason says why a rendering shows nothing: a view exposing nothing, or a
// view whose exposed elements this kind of rendering cannot show, which the
// notices then account for one by one.
func (r *Rendering) emptyReason() string {
	if len(r.Notices) > 0 {
		return fmt.Sprintf("the rendering is empty: nothing the view exposes is shown by a %s rendering", r.Kind)
	}
	return "the view exposes nothing; the rendering is empty"
}

// writeNodeText writes one node and its children, and records the label an edge
// names the node by.
func writeNodeText(b *strings.Builder, node *Node, depth int, labels map[string]string) {
	labels[node.ID] = nodeLabel(node)
	line := strings.Repeat("  ", depth) + node.Kind
	if node.Name != "" {
		line += " " + node.Name
	}
	if node.Detail != "" {
		line += " (" + node.Detail + ")"
	}
	b.WriteString(line + "\n")
	for _, child := range node.Children {
		writeNodeText(b, child, depth+1, labels)
	}
}

// writeNotices writes what the rendering could not represent, so nothing is
// dropped silently.
func writeNotices(b *strings.Builder, notices []string) {
	if len(notices) == 0 {
		return
	}
	b.WriteString("\nnot represented:\n")
	for _, notice := range notices {
		b.WriteString("  - " + notice + "\n")
	}
}

// nodeLabel names a node where an edge refers to it: its name, else its kind
// with the identity the rendering gave it.
func nodeLabel(node *Node) string {
	if node.Name != "" {
		return node.Name
	}
	return fmt.Sprintf("%s %s", node.Kind, node.ID)
}

// edgeSectionName heads the edge list with what the edges of that kind are.
func edgeSectionName(kind Kind) string {
	switch kind {
	case KindState:
		return "transitions"
	case KindAction:
		return "flow"
	}
	return "connections"
}

// edgeArrow is how an edge of each kind is drawn in text.
func edgeArrow(kind EdgeKind) string {
	switch kind {
	case EdgeConnection:
		return "—"
	case EdgeFlow:
		return "=>"
	}
	return "->"
}
