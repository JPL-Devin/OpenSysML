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

// nameChoices is the spelling chosen for each reference: one that resolves to
// the target from where it is written, so the notation names the same element
// the graph does.
type nameChoices map[nameKey]string

// chooseNames reads notation written with every graph reference fully
// qualified and finds, for each reference in wanted, a spelling that resolves
// to its target there: the scope-relative name wanted proposes when it does,
// else the shortest suffix of the qualified name that does, else the global
// form. A reference that no spelling reaches is refused: writing it would
// change what the model means. Text the parser cannot read yields no choices.
func chooseNames(text []byte, wanted map[nameKey]string) (nameChoices, error) {
	names := nameChoices{}
	if len(wanted) == 0 {
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
	// A chain's member segments are looked up in the operand, not the writing
	// scope, so only their root is a reference whose spelling is chosen here.
	occurrences := map[nameKey][]resolve.Reference{}
	for _, ref := range resolve.References(root, e.res.Index().DocumentRoot(file.Name())) {
		if ref.QN == nil || ref.Chain != nil || ref.Member == nil {
			continue
		}
		key := nameKey{member: e.fqn[ref.Member], target: qualifiedText(ref.QN)}
		if _, ok := wanted[key]; ok {
			occurrences[key] = append(occurrences[key], ref)
		}
	}
	for key, refs := range occurrences {
		target, ok := declared[key.target]
		if !ok {
			continue
		}
		spelling, ok := spellingFor(e.res, refs, wanted[key], key.target, target)
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

// spellingFor returns the first of the preferred spelling, the suffixes of
// qname from shortest to longest, and the global form that every occurrence
// resolves to target, each read the way the resolver reads a name there.
func spellingFor(res *resolve.Resolver, refs []resolve.Reference, preferred, qname string, target ast.Node) (string, bool) {
	if resolvesTo(res, refs, strings.Split(preferred, "::"), false, target) {
		return preferred, true
	}
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
		ref.QN = qn
		sym, ok := res.ProbeReference(ref)
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
