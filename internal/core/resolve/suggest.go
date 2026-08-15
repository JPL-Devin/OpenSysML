package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// suggestKey identifies a suggestion by the name that did not resolve and the
// scope it was written in, which is what decides how a candidate is reached.
type suggestKey struct {
	scope *symbols.Scope
	name  string
}

// suggestFor returns the spellings an unresolvable unqualified name written in
// scope may have meant: the qualified names the index declares that very name
// under — an unimported library name — and the near spellings a typo justifies,
// each scored by how the user would reach it. Memoized per scope and name.
func (r *Resolver) suggestFor(scope *symbols.Scope, name string) []string {
	if r.idx == nil || name == "" || r.suggesting {
		return nil
	}
	key := suggestKey{scope: scope, name: name}
	if cands, ok := r.suggestions[key]; ok {
		return cands
	}
	r.suggesting = true
	defer func() { r.suggesting = false }()

	table := r.suggestTable()
	var cands []suggest.Candidate
	for _, fqn := range table.Qualified(name) {
		cands = append(cands, suggest.Candidate{Spelling: fqn, Library: r.libraryFQN(fqn)})
	}
	for _, near := range table.Neighbours(name) {
		if c, ok := r.candidateFor(scope, near); ok {
			cands = append(cands, c)
		}
	}
	out := suggest.Rank(cands)
	r.suggestions[key] = out
	return out
}

// candidateFor scores one near spelling: what the user would have to write to
// reach it from scope — the name itself when it resolves there, else the
// qualified path that declares it — and whether that is one of their own
// declarations or a bundled library name.
func (r *Resolver) candidateFor(scope *symbols.Scope, near suggest.Neighbour) (suggest.Candidate, bool) {
	if res := r.walkUnqualified(scope, near.Name); res.ok {
		return suggest.Candidate{
			Spelling: near.Name,
			Distance: near.Distance,
			InScope:  true,
			Library:  r.idx.Library(res.sym),
		}, true
	}
	qualified := r.suggestTable().Qualified(near.Name)
	if len(qualified) == 0 {
		return suggest.Candidate{}, false
	}
	return suggest.Candidate{
		Spelling: qualified[0],
		Distance: near.Distance,
		Library:  r.libraryFQN(qualified[0]),
	}, true
}

// libraryFQN reports whether fqn names a bundled library declaration.
func (r *Resolver) libraryFQN(fqn string) bool {
	return r.idx.Library(r.idx.Declaring(fqn))
}

// suggestTable indexes the names the index registers, swept once per resolver:
// a suggestion is a lookup, not a scan of the whole library per unresolved name.
func (r *Resolver) suggestTable() *suggest.Table {
	if r.names == nil {
		r.names = suggest.NewTable(r.idx)
	}
	return r.names
}

// unresolvedMessage is what an unresolved unqualified reference written in scope
// reports. The hint belongs to the diagnostic, not to one renderer, so the CLI,
// the REPL and the LSP all show it.
func (r *Resolver) unresolvedMessage(scope *symbols.Scope, name string) string {
	return suggest.With(unresolvedReferencePrefix+name, name, r.suggestFor(scope, name))
}

// unresolvedReferencePrefix is how a reference that resolves to nothing reads.
const unresolvedReferencePrefix = "unresolved reference: "
