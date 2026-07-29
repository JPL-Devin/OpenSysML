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
