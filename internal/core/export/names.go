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

// chooseNames reads notation written with every reference fully qualified and
// picks each a spelling that resolves to its target there; none is a refusal.
func chooseNames(text []byte, wanted map[nameKey]bool) (nameChoices, error) {
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
		key := nameKey{member: e.fqn[ref.Member], target: e.writtenTarget(ref)}
		if _, ok := wanted[key]; ok {
			occurrences[key] = append(occurrences[key], ref)
		}
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
