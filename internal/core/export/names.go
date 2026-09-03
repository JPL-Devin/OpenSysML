package export

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// nameKey identifies the references written in one declaration to one element:
// the qualified names of the declaring member and of the element referenced.
type nameKey struct {
	member, target string
}

// nameChoices is the spelling chosen for each reference: one that resolves,
// from where it is written, to the element the graph names.
type nameChoices map[nameKey]string

// segmentKey identifies the chain segments of one name written in one
// declaration after one operand (the element it reaches, "" for none).
type segmentKey struct {
	member, operand, name string
}

// wanted is what the fully qualified rendering notes for chooseNames: the
// references to spell, and the names read in an operand or body to check.
type wanted struct {
	// references holds, per reference, the qualified spelling it was written as.
	references map[nameKey]string
	// segments holds the elements the graph names by each chain segment.
	segments map[segmentKey]map[string]bool
	// starts holds the member each `first` names, keyed by the initial node.
	starts map[string]string
}

func newWanted() *wanted {
	return &wanted{
		references: map[nameKey]string{},
		segments:   map[segmentKey]map[string]bool{},
		starts:     map[string]string{},
	}
}

func (w *wanted) empty() bool {
	return len(w.references) == 0 && len(w.segments) == 0 && len(w.starts) == 0
}

// chooseNames reads notation written with every reference fully qualified, as
// the language name says, and picks each a spelling that resolves to its target
// there; none is a refusal.
func chooseNames(name string, text []byte, want *wanted) (nameChoices, error) {
	names := nameChoices{}
	if want.empty() {
		return names, nil
	}
	file, root, ok := readNotation(name, text)
	if !ok {
		return names, nil
	}
	e, err := newEncoder(file, root)
	if err != nil {
		return names, nil
	}
	declared := make(map[string]ast.Node, len(e.fqn))
	for node, fqn := range e.fqn {
		declared[fqn] = node
	}
	written := writtenKeys(want.references)
	occurrences := map[nameKey][]resolve.Reference{}
	var chains []resolve.Reference
	for _, ref := range resolve.References(root, e.res.Index().DocumentRoot(file.Name())) {
		if ref.QN == nil || ref.Member == nil {
			continue
		}
		if ref.Chain != nil {
			chains = append(chains, ref)
			continue
		}
		member := e.fqn[ref.Member]
		key := nameKey{member: member, target: e.writtenTarget(ref)}
		if _, ok := want.references[key]; !ok {
			// The spelling written may read as another element here; it is
			// still the occurrence of the reference written that way.
			key, ok = written[nameKey{member: member, target: qualifiedText(ref.QN)}]
			if !ok {
				continue
			}
		}
		occurrences[key] = append(occurrences[key], ref)
	}
	if err := e.checkStarts(want.starts, declared); err != nil {
		return nil, err
	}
	for key := range want.references {
		if _, ok := occurrences[key]; !ok {
			// A reference written but never read back cannot be checked to reach
			// its element, so it is not spelled by guess.
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the reference to %s from %s", key.target, key.member),
				Note: "the notation written for it does not read back as a reference, so the spelling cannot be checked",
			}
		}
	}
	chosen := map[*ast.QualifiedName]string{}
	for key, refs := range occurrences {
		target, ok := declared[key.target]
		if !ok {
			continue
		}
		// Each occurrence was written with the same fully qualified spelling.
		spelling, ok := spellingFor(e.res, refs, qualifiedText(refs[0].QN), target)
		if !ok {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the reference to %s from %s", key.target, key.member),
				Note: "no spelling of the name resolves to that element from where it is written, so the notation cannot state it",
			}
		}
		names[key] = spelling
		for _, ref := range refs {
			chosen[ref.QN] = spelling
		}
	}
	// A chain reads from its root, so its segments are checked once the root is
	// spelled: the fully qualified first spelling may reach nothing from there.
	for _, ref := range chains {
		if err := e.checkSegment(ref, want.segments, chosen); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// writtenKeys indexes the wanted references by the member and the spelling
// they were written as; a spelling two targets share in one member is left out.
func writtenKeys(references map[nameKey]string) map[nameKey]nameKey {
	written := map[nameKey]nameKey{}
	shared := map[nameKey]bool{}
	for key, spelled := range references {
		as := nameKey{member: key.member, target: spelled}
		if _, dup := written[as]; dup {
			shared[as] = true
			continue
		}
		written[as] = key
	}
	for as := range shared {
		delete(written, as)
	}
	return written
}

// checkSegment refuses a chain segment that does not read, from the operand it
// is written after, as the one element the graph names by it there. The chain
// is read with its root spelled as chosen.
func (e *encoder) checkSegment(ref resolve.Reference, segments map[segmentKey]map[string]bool, chosen map[*ast.QualifiedName]string) error {
	read := ref
	read.Chain = respelled(ref.Chain, chosen)
	read.QN = read.Chain.Member
	key := segmentKey{member: e.fqn[ref.Member], name: qualifiedText(ref.QN)}
	var targets map[string]bool
	for _, operand := range e.operandKeys(read) {
		key.operand = operand
		if targets = segments[key]; len(targets) > 0 {
			break
		}
	}
	if len(targets) == 0 {
		return nil
	}
	if len(targets) > 1 {
		return &UnsupportedError{
			What: fmt.Sprintf("the feature chains in %s reaching %s", key.member, key.name),
			Note: "written after the same operand, the segments name different elements in the graph, which one spelling cannot state",
		}
	}
	if _, reached, ok := e.reads(read); ok && targets[reached] {
		return nil
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the feature chain in %s reaching %s", key.member, key.name),
		Note: "the segment does not read as the element the graph names from the operand it is written after, so the notation cannot state it",
	}
}

// operandKeys are the operands a chain segment may be noted after: the element
// the operand reads as, then the operand as written, which is all the graph
// keeps where it links the operand to no element.
func (e *encoder) operandKeys(ref resolve.Reference) []string {
	operand := ref
	if inner, ok := ref.Chain.Operand.(*ast.FeatureChainExpr); ok {
		operand.Chain, operand.QN = inner, inner.Member
	} else {
		operand.Chain, operand.QN = nil, ast.AsQualifiedName(ref.Chain.Operand)
	}
	if operand.QN == nil {
		return []string{""}
	}
	_, fqn, _ := e.reads(operand)
	return []string{fqn, qualifiedText(operand.QN)}
}

// reads is the element a reference reads as where it is written.
func (e *encoder) reads(ref resolve.Reference) (ast.Node, string, bool) {
	sym, ok := e.res.ProbeReference(ref)
	return e.linkedElement(ref.QN, sym, ok)
}

// respelled is chain with its root written as chosen, on fresh nodes so the
// resolver's memo of the first spelling is kept; chain itself when unchanged.
func respelled(chain *ast.FeatureChainExpr, chosen map[*ast.QualifiedName]string) *ast.FeatureChainExpr {
	operand, changed := respelledOperand(chain.Operand, chosen)
	if !changed {
		return chain
	}
	return &ast.FeatureChainExpr{Operand: operand, Member: spelledName(qualifiedText(chain.Member))}
}

func respelledOperand(node ast.Node, chosen map[*ast.QualifiedName]string) (ast.Node, bool) {
	switch n := node.(type) {
	case *ast.FeatureChainExpr:
		inner := respelled(n, chosen)
		return inner, inner != n
	case *ast.IndexExpr:
		operand, changed := respelledOperand(n.Operand, chosen)
		if !changed {
			return n, false
		}
		return &ast.IndexExpr{Operand: operand, Index: n.Index, Bracket: n.Bracket}, true
	case *ast.FeatureReference:
		if spelling, ok := chosen[n.Name]; ok {
			return &ast.FeatureReference{Name: spelledName(spelling)}, true
		}
	case *ast.QualifiedName:
		if spelling, ok := chosen[n]; ok {
			return spelledName(spelling), true
		}
	}
	return node, false
}

// checkStarts refuses a `first` whose start does not read as the member the
// graph names in the body it is written in.
func (e *encoder) checkStarts(starts map[string]string, declared map[string]ast.Node) error {
	for fqn, target := range starts {
		initial, ok := declared[fqn].(*ast.InitialNode)
		if !ok {
			return &UnsupportedError{
				What: fmt.Sprintf("the initial node %s", fqn),
				Note: "the notation written for it does not read back as an initial node, so its start cannot be checked",
			}
		}
		if _, reached, ok := e.linked(e.res.InitialSymbol(initial)); ok && reached == target {
			continue
		}
		return &UnsupportedError{
			What: fmt.Sprintf("the initial node %s", fqn),
			Note: fmt.Sprintf("`first %s` does not name %s in the body it is written in, so the notation cannot state it", nameText(initial.Name), target),
		}
	}
	return nil
}

// writtenTarget is the qualified name of the element a written reference names:
// the alias it is written through, else what it resolves to, else its text.
func (e *encoder) writtenTarget(ref resolve.Reference) string {
	if alias, ok := e.res.PartAlias(ref.QN, len(ref.QN.Parts)-1); ok {
		if fqn, ok := e.fqn[alias.Decl]; ok {
			return fqn
		}
	}
	if sym, ok := e.res.ProbeReference(ref); ok && sym != nil {
		if fqn, ok := e.fqn[sym.Decl]; ok {
			return fqn
		}
	}
	return qualifiedText(ref.QN)
}

// readNotation parses text as the language name's extension says, or as an
// extensionless buffer when it has none.
func readNotation(name string, text []byte) (*source.SourceFile, *ast.RootNamespace, bool) {
	file := source.New(name, text)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		return nil, nil, false
	}
	return file, root, true
}

// spellingFor tries qname's suffixes shortest first, then the global form,
// returning the first every occurrence resolves to target.
func spellingFor(res *resolve.Resolver, refs []resolve.Reference, qname string, target ast.Node) (string, bool) {
	segments := strings.Split(qname, "::")
	for i := len(segments) - 1; i >= 0; i-- {
		if spelling := strings.Join(segments[i:], "::"); resolvesTo(res, refs, spelling, target) {
			return spelling, true
		}
	}
	if spelling := "$::" + qname; resolvesTo(res, refs, spelling, target) {
		return spelling, true
	}
	return "", false
}

// spelledName is the qualified name written as text, on a fresh node: the
// resolver memoizes by node.
func spelledName(text string) *ast.QualifiedName {
	qn := &ast.QualifiedName{}
	if rest, ok := strings.CutPrefix(text, "$::"); ok {
		qn.Global, text = true, rest
	}
	for _, segment := range strings.Split(text, "::") {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: segment})
	}
	return qn
}

func resolvesTo(res *resolve.Resolver, refs []resolve.Reference, spelling string, target ast.Node) bool {
	for _, ref := range refs {
		qn := spelledName(spelling)
		sym, ok := res.ProbeReference(ref.Spelled(qn))
		if !ok || sym == nil {
			return false
		}
		// The graph links a name written through an alias to that alias.
		alias, aliased := res.PartAlias(qn, len(qn.Parts)-1)
		if sym.Decl != target && !(aliased && alias.Decl == target) {
			return false
		}
	}
	return true
}
