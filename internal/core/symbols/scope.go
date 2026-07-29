package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Scope is a node in the immutable per-document scope tree. It owns the
// symbols declared directly within a namespace-like construct and links to
// its parent and child scopes.
type Scope struct {
	parent      *Scope
	node        ast.Node             // the owning declaration node (nil for the doc root)
	members     map[string][]*Symbol // name key -> symbols defined under that key (in definition order)
	memberOrder []string             // name keys in first-seen order (for deterministic enumeration)
	children    []*Scope
}

// NewScope creates an empty scope with the given parent and owning node.
func NewScope(parent *Scope, node ast.Node) *Scope {
	return &Scope{
		parent:  parent,
		node:    node,
		members: make(map[string][]*Symbol),
	}
}

// Parent returns the enclosing scope, or nil for the document root.
func (s *Scope) Parent() *Scope { return s.parent }

// Node returns the AST node that owns this scope, or nil for the document root.
func (s *Scope) Node() ast.Node { return s.node }

// Children returns the child scopes in definition order.
func (s *Scope) Children() []*Scope { return s.children }

// AddChild appends a child scope.
func (s *Scope) AddChild(c *Scope) { s.children = append(s.children, c) }

// Define registers sym under the given name key. Multiple symbols may share a
// key (duplicate declarations); all are retained in definition order.
func (s *Scope) Define(name string, sym *Symbol) {
	if name == "" {
		return
	}
	if _, ok := s.members[name]; !ok {
		s.memberOrder = append(s.memberOrder, name)
	}
	s.members[name] = append(s.members[name], sym)
}

// LookupLocal returns the first symbol defined under name in this scope only.
func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	syms := s.members[name]
	if len(syms) == 0 {
		return nil, false
	}
	return syms[0], true
}

// LookupLocalAll returns every symbol defined under name in this scope only.
func (s *Scope) LookupLocalAll(name string) []*Symbol {
	return s.members[name]
}
