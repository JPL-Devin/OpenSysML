package export

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// elementIdentity is one element's effective repository identity, in the
// encoder's terms: keyed by the qualified name the encoder computes.
type elementIdentity struct {
	id       string
	declared bool
	scope    *identity.Scope
}

// identityFacts is the identity side table of one document translated for the
// encoder: effective ids by qualified name, the annotation nodes consumed
// into identity rather than exported as model content, and each ProjectRef
// scope keyed by the qualified name of the namespace that declares it.
type identityFacts struct {
	byFQN      map[string]elementIdentity
	consumed   map[ast.Node]bool
	provenance map[string]*identity.Scope
	// qualified reports a multi-scope document, whose scoped elements get
	// IRIs qualified by their scope so ids repeated across scopes stay apart.
	qualified bool
}

// documentIdentity builds the identity side table for one parsed document and
// refuses what the graph cannot carry: an annotation whose id is not a
// constant string, an empty id, or an id outside the element id alphabet.
func documentIdentity(name string, root *ast.RootNamespace) (*identityFacts, error) {
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	table := identity.Build(model, res, idx.DocumentRoot(name))

	facts := &identityFacts{
		byFQN:      map[string]elementIdentity{},
		consumed:   map[ast.Node]bool{},
		provenance: map[string]*identity.Scope{},
	}
	scopeKeys := map[string]bool{}
	for _, sym := range table.Symbols() {
		info, ok := table.Info(sym)
		if !ok {
			continue
		}
		if info.Annotated {
			if err := exportableID(info); err != nil {
				return nil, err
			}
			for _, d := range info.Declarations {
				facts.consumed[d.Node] = true
			}
		}
		if info.Scope != nil {
			for _, d := range info.Scope.Declarations {
				facts.consumed[d.Node] = true
			}
			facts.provenance[idx.GetFQN(info.Scope.Symbol)] = info.Scope
			scopeKeys[info.Scope.Key()] = true
		}
		facts.byFQN[info.FQN] = elementIdentity{
			id:       info.EffectiveID,
			declared: info.Declared,
			scope:    info.Scope,
		}
	}
	facts.qualified = len(scopeKeys) > 1
	return facts, nil
}

// exportableID rejects an ElementId annotation the graph cannot carry back.
func exportableID(info *identity.Info) error {
	if !info.Declared {
		return &UnsupportedError{
			What: fmt.Sprintf("the ElementId annotation on %s", info.FQN),
			Note: "its id is not a constant string, so the id it declares cannot be carried into the graph",
		}
	}
	if info.DeclaredID == "" {
		return &UnsupportedError{
			What: fmt.Sprintf("the ElementId annotation on %s", info.FQN),
			Note: "its id is empty, and an element IRI cannot be built from an empty id",
		}
	}
	for i := 0; i < len(info.DeclaredID); i++ {
		c := info.DeclaredID[i]
		if !idByte(c) && c != '_' {
			return &UnsupportedError{
				What: fmt.Sprintf("the ElementId annotation on %s", info.FQN),
				Note: fmt.Sprintf("its id %q holds a byte outside [A-Za-z0-9_-], which an element IRI cannot carry", info.DeclaredID),
			}
		}
	}
	return nil
}

// idByte reports whether c may appear in a declared element id.
func idByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}

// subjectFor returns the subject IRI of the element with the given qualified
// name: its effective id, scope-qualified when the document is multi-scope.
func (f *identityFacts) subjectFor(fqn string) rdf.Term {
	el, ok := f.byFQN[fqn]
	if !ok {
		return rdf.ElementIRI(fqn)
	}
	if f.qualified && el.scope != nil {
		return rdf.ScopedElementIRIForID(rdf.ScopeQualifier(el.scope.Org, el.scope.ProjectID), el.id)
	}
	return rdf.ElementIRIForID(el.id)
}

// declaredID reports whether the element's id came from an explicit
// ElementId annotation, which the graph must record: explicitness is not
// recoverable from the value.
func (f *identityFacts) declaredID(fqn string) bool {
	return f.byFQN[fqn].declared
}

// skip reports whether node is an identity annotation consumed into the
// graph's identity rather than exported as model content.
func (f *identityFacts) skip(node ast.Node) bool {
	return f.consumed[node]
}
