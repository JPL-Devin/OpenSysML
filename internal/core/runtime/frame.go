package runtime

// frame is one level of local bindings an evaluation reads: a calc invocation's
// parameter slots, a map of named values, or both.
type frame struct {
	slots *slotFrame
	vars  map[string]Value
}

// mapFrame is a frame holding vars alone.
func mapFrame(vars map[string]Value) frame {
	return frame{vars: vars}
}

// lookup finds name in the frame: a slot binding it, else the map.
func (f frame) lookup(name string) (Value, bool) {
	if f.slots != nil {
		if value, ok := f.slots.lookup(name); ok {
			return value, true
		}
	}
	value, ok := f.vars[name]
	return value, ok
}

// has reports whether the frame binds name.
func (f frame) has(name string) bool {
	_, ok := f.lookup(name)
	return ok
}

// set binds name: in its slot when the frame has one for it, else in the map.
func (f frame) set(name string, value Value) {
	if f.slots != nil && f.slots.set(name, value) {
		return
	}
	f.vars[name] = value
}

// bindParam binds parameter i, named name: by position in the slots, else by name.
func (f frame) bindParam(i int, name string, value Value) {
	if f.slots != nil {
		f.slots.bind(i, value)
		return
	}
	f.vars[name] = value
}

// each calls fn for every name the frame binds.
func (f frame) each(fn func(name string, value Value)) {
	if f.slots != nil {
		f.slots.each(fn)
	}
	for name, value := range f.vars {
		fn(name, value)
	}
}

// width is the number of names the frame binds.
func (f frame) width() int {
	n := len(f.vars)
	if f.slots != nil {
		n += f.slots.width()
	}
	return n
}

// slotFrame holds a calc invocation's parameters by position, the names coming
// from its shape, so binding and reading them index rather than hash.
type slotFrame struct {
	names  []string
	values []Value
	bound  []bool
}

// reset prepares the frame for a calc binding the named parameters, with none
// bound yet, reusing the storage it holds.
func (s *slotFrame) reset(names []string) {
	s.names = names
	n := len(names)
	if cap(s.values) < n {
		s.values = make([]Value, n)
		s.bound = make([]bool, n)
	} else {
		s.values = s.values[:n]
		s.bound = s.bound[:n]
		clear(s.values)
		clear(s.bound)
	}
}

// release drops what the frame holds, keeping its storage for the next reset.
func (s *slotFrame) release() {
	clear(s.values)
	clear(s.bound)
	s.names = nil
	s.values = s.values[:0]
	s.bound = s.bound[:0]
}

// bind binds the parameter in slot i.
func (s *slotFrame) bind(i int, value Value) {
	s.values[i] = value
	s.bound[i] = true
}

// lookup finds a bound parameter by name.
func (s *slotFrame) lookup(name string) (Value, bool) {
	for i, n := range s.names {
		if n == name && s.bound[i] {
			return s.values[i], true
		}
	}
	return Value{}, false
}

// set binds the parameter named name and reports whether there is one.
func (s *slotFrame) set(name string, value Value) bool {
	for i, n := range s.names {
		if n == name {
			s.bind(i, value)
			return true
		}
	}
	return false
}

// each calls fn for every bound parameter.
func (s *slotFrame) each(fn func(name string, value Value)) {
	for i, n := range s.names {
		if s.bound[i] {
			fn(n, s.values[i])
		}
	}
}

// width is the number of bound parameters.
func (s *slotFrame) width() int {
	n := 0
	for _, b := range s.bound {
		if b {
			n++
		}
	}
	return n
}
