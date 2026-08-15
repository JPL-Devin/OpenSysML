package runtime

import "sort"

// Builtin is one library function this runtime implements directly, described
// for a caller that lists them: the unqualified name a call may use, the
// library declaration that name denotes, and the parameter names of that
// declaration when the registry knows them.
type Builtin struct {
	Name   string
	FQN    string
	Params []string
	// Collection is true for the operations over sequences, collections and
	// bodies, which are the ones written in the postfix `x->name()` form.
	Collection bool
}

// Builtins returns every library function callable by its unqualified name, in
// name order. It is the registry behind the REPL's %builtins listing, so what
// the REPL advertises is what dispatch actually implements.
func Builtins() []Builtin {
	out := make([]Builtin, 0, len(libraryFunctionsByLocalName)+len(builtinLocalNames))
	for name, fn := range libraryFunctionsByLocalName {
		out = append(out, Builtin{Name: name, FQN: fn.name, Params: fn.params})
	}
	for name, fqn := range builtinLocalNames {
		out = append(out, Builtin{Name: name, FQN: fqn, Collection: true})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].FQN < out[j].FQN
	})
	return out
}
