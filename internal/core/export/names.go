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
	references map[nameKey]bool
	// segments holds the elements the graph names by each chain segment.
	segments map[segmentKey]map[string]bool
	// starts holds the member each `first` names, keyed by the initial node.
	starts map[string]string
}

func newWanted() *wanted {
	return &wanted{
		references: map[nameKey]bool{},
		segments:   map[segmentKey]map[string]bool{},
		starts:     map[string]string{},
	}
}

func (w *wanted) empty() bool {
	return len(w.references) == 0 && len(w.segments) == 0 && len(w.starts) == 0
}

// chooseNames reads notation written with every reference fully qualified and
// picks each a spelling that resolves to its target there; none is a refusal.
func chooseNames(text []byte, want *wanted) (nameChoices, error) {
	names := nameChoices{}
	if want.empty() {
		return names, nil
	}
	file, root, ok := readNotation(text)
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
	occurrences := map[nameKey][]resolve.Reference{}
	for _, ref := range resolve.References(root, e.res.Index().DocumentRoot(file.Name())) {
		if ref.QN == nil || ref.Member == nil {
			continue
		}
		if ref.Chain != nil {
			if err := e.checkSegment(ref, want.segments); err != nil {
				return nil, err
			}
			continue
		}
		key := nameKey{member: e.fqn[ref.Member], target: e.writtenTarget(ref)}
		if _, ok := want.references[key]; ok {
			occurrences[key] = append(occurrences[key], ref)
		}
	}
	if err := e.checkStarts(want.starts, declared); err != nil {
		return nil, err
	}
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
	}
	return names, nil
}

// checkSegment refuses a chain segment that does not read, from the operand it
// is written after, as the one element the graph names by it there.
func (e *encoder) checkSegment(ref resolve.Reference, segments map[segmentKey]map[string]bool) error {
	key := segmentKey{member: e.fqn[ref.Member], operand: e.operandElement(ref.Chain), name: qualifiedText(ref.QN)}
	targets := segments[key]
	if len(targets) == 0 {
		return nil
	}
	if len(targets) > 1 {
		return &UnsupportedError{
			What: fmt.Sprintf("the feature chains in %s reaching %s", key.member, key.name),
			Note: "written after the same operand, the segments name different elements in the graph, which one spelling cannot state",
		}
	}
	if _, reached, ok := e.referent(ref.QN); ok && targets[reached] {
		return nil
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the feature chain in %s reaching %s", key.member, key.name),
		Note: "the segment does not read as the element the graph names from the operand it is written after, so the notation cannot state it",
	}
}

// operandElement is the qualified name of the element the graph links a chain's
// operand to: the name it is, or the segment an inner chain reaches.
func (e *encoder) operandElement(chain *ast.FeatureChainExpr) string {
	var name *ast.QualifiedName
	if inner, ok := chain.Operand.(*ast.FeatureChainExpr); ok {
		name = inner.Member
	} else {
		name = ast.AsQualifiedName(chain.Operand)
	}
	if name == nil {
		return ""
	}
	_, fqn, _ := e.referent(name)
	return fqn
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

// readNotation parses text in whichever of the two languages reads it clean,
// since the graph does not record which one it was written in.
func readNotation(text []byte) (*source.SourceFile, *ast.RootNamespace, bool) {
	for _, kind := range []source.Kind{source.KindSysML, source.KindKerML} {
		file := source.NewWithKind("<converted>", text, kind)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) == 0 {
			return file, root, true
		}
	}
	return nil, nil, false
}

// spellingFor tries qname's suffixes shortest first, then the global form,
// returning the first every occurrence resolves to target.
func spellingFor(res *resolve.Resolver, refs []resolve.Reference, qname string, target ast.Node) (string, bool) {
	segments := strings.Split(qname, "::")
	for i := len(segments) - 1; i >= 0; i-- {
		if resolvesTo(res, refs, segments[i:], false, target) {
			return strings.Join(segments[i:], "::"), true
		}
	}
	if resolvesTo(res, refs, segments, true, target) {
		return "$::" + qname, true
	}
	return "", false
}

func resolvesTo(res *resolve.Resolver, refs []resolve.Reference, segments []string, global bool, target ast.Node) bool {
	for _, ref := range refs {
		// A fresh node per reading: the resolver memoizes by node.
		qn := &ast.QualifiedName{Global: global}
		for _, segment := range segments {
			qn.Parts = append(qn.Parts, ast.NameSegment{Text: segment})
		}
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
