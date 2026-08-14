package symbols

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// Scope is a node in the immutable per-document scope tree. It owns the
// symbols declared directly within a namespace-like construct and links to
// its parent and child scopes.
type Scope struct {
	parent           *Scope
	owner            *Symbol              // the symbol that owns this scope (for inheritance lookup)
	node             ast.Node             // the owning declaration node (nil for the doc root)
	members          map[string][]*Symbol // name key -> symbols defined under that key (in definition order)
	memberOrder      []string             // name keys in first-seen order (for deterministic enumeration)
	anonymousMembers []*Symbol            // anonymous symbols (no name) that aren't in members map
	children         []*Scope
	childByNode      map[ast.Node]*Scope // declaration node -> the child scope it owns
	bodyLocal        bool                // declarations live only inside the owning body
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

// Owner returns the symbol that owns this scope, or nil if not set.
func (s *Scope) Owner() *Symbol { return s.owner }

// SetOwner sets the symbol that owns this scope (for inheritance lookup).
func (s *Scope) SetOwner(sym *Symbol) { s.owner = sym }

// Node returns the AST node that owns this scope, or nil for the document root.
func (s *Scope) Node() ast.Node { return s.node }

// BodyLocal reports whether this scope's names exist only inside the body that
// declares them, so a subtree search such as a recursive import must skip it.
func (s *Scope) BodyLocal() bool { return s.bodyLocal }

// markBodyLocal records that this scope's names do not escape its body.
func (s *Scope) markBodyLocal() { s.bodyLocal = true }

// Children returns the child scopes in definition order.
func (s *Scope) Children() []*Scope { return s.children }

// AddChild appends a child scope.
func (s *Scope) AddChild(c *Scope) {
	s.children = append(s.children, c)
	if node := c.Node(); node != nil {
		if s.childByNode == nil {
			s.childByNode = make(map[ast.Node]*Scope)
		}
		if _, ok := s.childByNode[node]; !ok {
			s.childByNode[node] = c
		}
	}
}

// ChildFor returns the child scope the given declaration owns, or nil when the
// declaration owns no scope here. It is the first such child, as two children of
// one node are the same body scoped twice.
func (s *Scope) ChildFor(node ast.Node) *Scope {
	if node == nil {
		return nil
	}
	return s.childByNode[node]
}

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

// DefineAnonymous adds an anonymous symbol (without name) to this scope.
func (s *Scope) DefineAnonymous(sym *Symbol) {
	s.anonymousMembers = append(s.anonymousMembers, sym)
}

// HasAnonymousMembers reports whether any member is declared without a name.
func (s *Scope) HasAnonymousMembers() bool {
	return len(s.anonymousMembers) > 0
}

// AnonymousMembers returns the members declared without a name, in declaration
// order. One may still have an effective name, taken from a feature it
// implicitly redefines, which only the semantic model knows (KerML 7.3.4.5).
func (s *Scope) AnonymousMembers() []*Symbol {
	out := make([]*Symbol, len(s.anonymousMembers))
	copy(out, s.anonymousMembers)
	return out
}

// LookupLocal returns the first symbol defined under name in this scope only.
// A declared name wins over an effective one taken from a referenced feature.
func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	syms := s.members[name]
	if len(syms) == 0 {
		return nil, false
	}
	if len(syms) == 1 {
		return syms[0], true
	}
	return PreferDeclared(syms)[0], true
}

// PreferDeclared drops symbols whose name was borrowed from a referenced
// feature when the key also has a declared one, which is how a `perform p;`
// shorthand and a declaration named p coexist in one scope.
func PreferDeclared(syms []*Symbol) []*Symbol {
	declared := make([]*Symbol, 0, len(syms))
	for _, sym := range syms {
		if !sym.EffectiveName {
			declared = append(declared, sym)
		}
	}
	if len(declared) == 0 {
		return syms
	}
	return declared
}

// LookupLocalAll returns every symbol defined under name in this scope only.
func (s *Scope) LookupLocalAll(name string) []*Symbol {
	return s.members[name]
}

// MemberNames returns the member keys of this scope in declaration order.
func (s *Scope) MemberNames() []string {
	out := make([]string, len(s.memberOrder))
	copy(out, s.memberOrder)
	return out
}

// AllMembers returns all symbols in this scope (including anonymous members).
func (s *Scope) AllMembers() []*Symbol {
	var all []*Symbol
	for _, syms := range s.members {
		all = append(all, syms...)
	}
	all = append(all, s.anonymousMembers...)
	return all
}
