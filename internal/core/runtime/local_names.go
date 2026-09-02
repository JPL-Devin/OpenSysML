package runtime

import "github.com/Open-MBEE/OpenSysML/internal/core/libnames"

// init derives the by-local-name dispatch tables from libnames once every
// registry is populated (this file sorts after builtins.go and
// library_functions.go), so a name the table lists but no registry implements
// is a build defect rather than a call answered wrongly. A bare name no
// declaration resolves dispatches to its most general declaration, the first listed.
func init() {
	builtinsByLocalName = map[string]func(*EvalContext, []Value) (Value, error){}
	builtinQualifiedNames = map[string]string{}
	for _, local := range libnames.Names() {
		for i, fqn := range libnames.Declarations(local) {
			fn, isLibrary := libraryFunctions[fqn]
			builtin, isBuiltin := builtins[fqn]
			switch {
			case isLibrary:
				if i == 0 {
					libraryFunctionsByLocalName[local] = fn
				}
			case isBuiltin:
				if i == 0 {
					builtinsByLocalName[local] = builtin
					builtinQualifiedNames[local] = fqn
				}
			default:
				panic("runtime: unqualified name " + local + " maps to unregistered library function " + fqn)
			}
		}
	}
	for _, local := range libnames.ExtensionNames() {
		pkg, _ := libnames.ExtensionPackage(local)
		if _, ok := libraryFunctions[pkg+"::"+local]; !ok {
			panic("runtime: extension function " + pkg + "::" + local + " is not registered")
		}
	}
}
