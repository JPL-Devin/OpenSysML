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
	// RequiresImport names the package a model must import for a call by this
	// unqualified name to be legal, and is empty for the OMG libraries, which
	// are in force whatever a model imports.
	RequiresImport string
}

// Builtins returns every library function this build implements, in name order,
// each with the unqualified name a call writes and the import that name needs,
// if any. It is the registry behind the REPL's %builtins listing, so what the
// REPL advertises is what this build implements.
func Builtins() []Builtin {
	out := make([]Builtin, 0,
		len(libraryFunctionsByLocalName)+len(builtinLocalNames)+len(extensionLocalNames))
	for name, fn := range libraryFunctionsByLocalName {
		out = append(out, Builtin{Name: name, FQN: fn.name, Params: fn.params})
	}
	// An extension function is implemented like any other; only dispatch by its
	// unqualified name waits on the import that declares it.
	for name, pkg := range extensionLocalNames {
		fqn := pkg + "::" + name
		fn, ok := libraryFunctions[fqn]
		if !ok {
			continue
		}
		out = append(out, Builtin{
			Name: name, FQN: fqn, Params: fn.params, RequiresImport: pkg,
		})
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
