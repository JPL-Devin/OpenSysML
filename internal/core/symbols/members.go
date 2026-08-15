package symbols

// Members returns the distinct symbols declared directly in this scope, in
// definition order. A symbol registered under both its short and primary name
// appears once. Callers must not mutate the returned slice's symbols.
func (s *Scope) Members() []*Symbol {
	seen := map[*Symbol]bool{}
	var out []*Symbol
	for _, name := range s.memberOrder {
		for _, sym := range s.members[name] {
			if seen[sym] {
				continue
			}
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out
}

// DocNameOf is the document a scope belongs to, read from the name SetDocName
// stamped on its symbols. It identifies a scope tree the index does not hold —
// a document builds its own for the editor — by name rather than by identity.
func DocNameOf(scope *Scope) string {
	for s := scope; s != nil; s = s.Parent() {
		if owner := s.Owner(); owner != nil && owner.DocName != "" {
			return owner.DocName
		}
		for _, sym := range s.Members() {
			if sym.DocName != "" {
				return sym.DocName
			}
		}
	}
	return ""
}

// SetDocName stamps name onto every symbol in the scope tree (members and
// all descendant scopes), recording which document declares each symbol.
// Recursion follows the child links, so scopes no symbol owns — loop bodies and
// body-expression parameters — are stamped too.
func SetDocName(scope *Scope, name string) {
	if scope == nil {
		return
	}
	for _, sym := range scope.Members() {
		sym.DocName = name
	}
	for _, child := range scope.Children() {
		SetDocName(child, name)
	}
}
