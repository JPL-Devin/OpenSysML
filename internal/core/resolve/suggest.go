package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
)

// suggestFor returns the spellings an unresolvable unqualified name may have
// meant: the qualified names the index declares it under — an unimported library
// name — else the nearest spellings, which is what a typo needs. Memoized.
func (r *Resolver) suggestFor(name string) []string {
	if r.idx == nil || name == "" {
		return nil
	}
	if cands, ok := r.suggestions[name]; ok {
		return cands
	}
	table := r.suggestTable()
	cands := table.Qualified(name)
	if len(cands) == 0 {
		for _, near := range table.Nearest(name) {
			if qualified := table.Qualified(near); len(qualified) > 0 {
				cands = append(cands, qualified[0])
			}
		}
	}
	r.suggestions[name] = cands
	return cands
}

// suggestTable indexes the names the index registers, swept once per resolver:
// a suggestion is a lookup, not a scan of the whole library per unresolved name.
func (r *Resolver) suggestTable() *suggest.Table {
	if r.names == nil {
		r.names = suggest.NewTable(r.idx)
	}
	return r.names
}

// unresolvedMessage is what an unresolved unqualified reference reports. The
// hint belongs to the diagnostic, not to one renderer, so the CLI, the REPL and
// the LSP all show it.
func (r *Resolver) unresolvedMessage(name string) string {
	return suggest.With(unresolvedReferencePrefix+name, name, r.suggestFor(name))
}

// unresolvedReferencePrefix is how a reference that resolves to nothing reads.
const unresolvedReferencePrefix = "unresolved reference: "
