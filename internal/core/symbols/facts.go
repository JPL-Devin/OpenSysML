package symbols

// LibraryFacts is the derived analysis of one library symbol: the semantic work
// whose derivation dominates a cold library load, held so that a later load can
// install it instead of repeating it.
//
// Every field is a memo of what the symbol's declaration yields, never a
// replacement for it — a library document is parsed and indexed on every load
// path, so a field left unset simply means "derive it from the declaration".
type LibraryFacts struct {
	// Supers are the fully-qualified names of the semantic direct supertypes,
	// in derivation order. Nil when an edge has no qualified name to restore it
	// by, which leaves the whole set to be derived from the declaration.
	Supers []string

	// Unit is the reduction of a measurement unit to base units. Nil for a
	// symbol that is not a measurement unit.
	Unit *UnitFacts

	// Dimension is the quantity dimension a measurement unit measures in. Nil
	// when the symbol is not a unit or its dimension is undetermined.
	Dimension *DimensionFacts
}

// InstallLibraryFacts installs derived facts on the symbols the named document
// declares, keyed by the fully-qualified name they are declared under. A name
// the document does not declare is ignored: facts are a memo of that document's
// own declarations.
func (idx *Index) InstallLibraryFacts(name string, facts map[string]*LibraryFacts) {
	idx.mustBeWritable("InstallLibraryFacts")
	for _, e := range idx.contributions.at(name) {
		if idx.declaredAt.at(e.sym) != e.fqn {
			continue // a short-name key of a symbol declared under its primary one
		}
		if f, ok := facts[e.fqn]; ok {
			e.sym.Facts = f
		}
	}
}
