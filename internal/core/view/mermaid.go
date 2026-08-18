package view

import (
	"fmt"
	"strings"
)

// Mermaid is the machine-readable form of a rendering. Mermaid was chosen over
// DOT because a Mermaid diagram renders where the models are read — in Markdown
// documentation, in the repository's own docs, and in the editors that host the
// language server — without a Graphviz installation, and because it has a
// state-diagram grammar the state rendering maps onto directly.
//
// A graph-shaped rendering is a `flowchart`; a state rendering is a
// `stateDiagram-v2`. What the rendering could not represent is written as
// comments, so no notice is lost in the machine-readable form either.
func (r *Rendering) Mermaid() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%%%% %s — %s rendering", r.View, r.Kind)
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	}
	b.WriteString("\n")
	for _, notice := range r.Notices {
		fmt.Fprintf(&b, "%%%% not represented: %s\n", notice)
	}
	if r.Kind == KindState {
		r.writeStateDiagram(&b)
		return b.String()
	}
	r.writeFlowchart(&b)
	return b.String()
}

// writeFlowchart writes the tree, interconnection and action renderings as a
// Mermaid flowchart: a node with children is a subgraph, containment in a tree
// is an edge, and every other edge is the one the rendering holds.
func (r *Rendering) writeFlowchart(b *strings.Builder) {
	direction := "TD"
	if r.Kind == KindInterconnection {
		direction = "LR"
	}
	fmt.Fprintf(b, "flowchart %s\n", direction)
	if r.Empty() {
		fmt.Fprintf(b, "  empty[\"%s\"]\n", mermaidText(r.emptyReason()))
		return
	}
	for _, root := range r.Roots {
		writeFlowchartNode(b, root, 1, r.Kind == KindTree)
	}
	for _, edge := range r.Edges {
		if edge.Label == "" {
			fmt.Fprintf(b, "  %s %s %s\n", edge.From, mermaidArrow(edge.Kind), edge.To)
			continue
		}
		fmt.Fprintf(b, "  %s %s|\"%s\"| %s\n", edge.From, mermaidArrow(edge.Kind), mermaidText(edge.Label), edge.To)
	}
}

// writeFlowchartNode writes one node: a subgraph when it holds others, a plain
// node otherwise. containment adds an edge from a node to each of its children,
// which is how a tree rendering shows what contains what.
func writeFlowchartNode(b *strings.Builder, node *Node, depth int, containment bool) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 {
		fmt.Fprintf(b, "%s%s[\"%s\"]\n", indent, node.ID, mermaidText(mermaidLabel(node)))
		return
	}
	if containment {
		fmt.Fprintf(b, "%s%s[\"%s\"]\n", indent, node.ID, mermaidText(mermaidLabel(node)))
		for _, child := range node.Children {
			writeFlowchartNode(b, child, depth, containment)
			fmt.Fprintf(b, "%s%s --- %s\n", indent, node.ID, child.ID)
		}
		return
	}
	fmt.Fprintf(b, "%ssubgraph %s [\"%s\"]\n", indent, node.ID, mermaidText(mermaidLabel(node)))
	for _, child := range node.Children {
		writeFlowchartNode(b, child, depth+1, containment)
	}
	fmt.Fprintf(b, "%send\n", indent)
}

// writeStateDiagram writes a state rendering as a Mermaid state diagram: each
// machine and composite state is a composite state, an initial state is entered
// from the start marker, and every transition carries its label.
func (r *Rendering) writeStateDiagram(b *strings.Builder) {
	b.WriteString("stateDiagram-v2\n")
	if r.Empty() {
		// A state diagram takes a note only attached to a state, so the reason
		// is a state of its own.
		fmt.Fprintf(b, "  state \"%s\" as empty\n", mermaidText(r.emptyReason()))
		return
	}
	for _, root := range r.Roots {
		writeStateNode(b, root, 1)
	}
	for _, edge := range r.Edges {
		if edge.Label == "" {
			fmt.Fprintf(b, "  %s --> %s\n", edge.From, edge.To)
			continue
		}
		fmt.Fprintf(b, "  %s --> %s : %s\n", edge.From, edge.To, mermaidText(edge.Label))
	}
}

// writeStateNode writes one state, its substates, and the start marker of an
// initial one.
func writeStateNode(b *strings.Builder, node *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if len(node.Children) == 0 {
		fmt.Fprintf(b, "%sstate \"%s\" as %s\n", indent, mermaidText(mermaidLabel(node)), node.ID)
	} else {
		fmt.Fprintf(b, "%sstate \"%s\" as %s {\n", indent, mermaidText(mermaidLabel(node)), node.ID)
		for _, child := range node.Children {
			writeStateNode(b, child, depth+1)
		}
		fmt.Fprintf(b, "%s}\n", indent)
	}
	if isInitial(node) {
		fmt.Fprintf(b, "%s[*] --> %s\n", indent, node.ID)
	}
}

// isInitial reports whether a state node is the one its machine or region enters
// first.
func isInitial(node *Node) bool {
	for _, detail := range strings.Split(node.Detail, ", ") {
		if detail == "initial" {
			return true
		}
	}
	return false
}

// mermaidLabel is the text a node carries in a diagram: its kind, its name, and
// what else the rendering said about it.
func mermaidLabel(node *Node) string {
	label := node.Kind
	if node.Name != "" {
		label += " " + node.Name
	}
	if node.Detail != "" {
		label += " (" + node.Detail + ")"
	}
	return label
}

// mermaidArrow is how an edge of each kind is drawn in a flowchart.
func mermaidArrow(kind EdgeKind) string {
	switch kind {
	case EdgeConnection:
		return "---"
	case EdgeFlow:
		return "-.->"
	}
	return "-->"
}

// mermaidText escapes what a quoted Mermaid label may not carry literally.
func mermaidText(text string) string {
	replacer := strings.NewReplacer("#", "#35;", "\"", "#quot;", "\n", " ", "<", "#lt;", ">", "#gt;")
	return replacer.Replace(text)
}
