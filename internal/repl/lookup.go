package repl

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// lookupSymbol resolves the name a REPL command was given against the session
// document. It is the single lookup path for every symbol-taking command, so
// `%instantiate`, `%eval`, `%calc` and the debuggers all agree on what a name
// denotes.
//
// A simple name (Vehicle) is searched through the whole scope tree, so a member
// of a package is found without qualification; a qualified name
// (Demo::Vehicle) is resolved through the symbol index, the same API the gRPC
// service resolves its symbol ids with. The second result is the
// fully-qualified name the symbol was found under, which is what instances are
// keyed by so the spelling used to create one need not be the spelling used to
// inspect it.
//
// The name is taken in the notation, so a segment quoted because it holds a
// space or is a keyword ('My Pkg'::Car) denotes what the index registers it as.
func (s *Session) lookupSymbol(name string) (*symbols.Symbol, string, error) {
	return s.lookupSymbolOfKinds(name)
}

// lookupSymbolOfKinds resolves a name as lookupSymbol does. want narrows the
// suggestions offered when nothing resolves to the kinds the command can act
// on, and decides nothing about what a resolved name denotes.
func (s *Session) lookupSymbolOfKinds(name string, want ...symbols.SymbolKind) (*symbols.Symbol, string, error) {
	// A name the notation reads is resolved by the text it names; anything else is
	// looked up as typed, so the failure reported is about what was typed.
	if plain, ok := s.plainName(name); ok {
		name = plain
	}
	docScopes := s.docScopes()
	// The library is searched even from an empty session, so a qualified
	// library name is not answered with "no declarations loaded".
	idx := s.browseIndex()
	if idx == nil {
		return nil, "", fmt.Errorf("no declarations loaded")
	}

	if strings.Contains(name, "::") {
		matches := idx.LookupQualified(name)
		switch len(matches) {
		case 0:
			// A name may reach through a declared feature into its type
			// (Outer::inner::b), which no declaration is registered under.
			if sym, fqn := s.featureChainSymbol(name); sym != nil {
				if local := scopeSymbolForAny(docScopes, sym.Decl); local != nil {
					sym = local
				}
				return sym, fqn, nil
			}
			return nil, "", s.notFoundError(name, want...)
		case 1:
			// The index owns its own scope tree; map the hit back onto the
			// document's tree so every command sees one symbol per declaration.
			sym := matches[0]
			fqn := idx.GetFQN(sym)
			if fqn == "" {
				fqn = name
			}
			if local := scopeSymbolForAny(docScopes, sym.Decl); local != nil {
				sym = local
			}
			return sym, fqn, nil
		default:
			return nil, "", ambiguousError(name, matches, idx)
		}
	}

	matches := s.nameTable().lookup(name)
	switch len(matches) {
	case 0:
		// A name the session declares nowhere may still be visible where the
		// prompt evaluates — through an import of that namespace.
		if sym, ok := resolve.New(idx).LookupName(s.promptScope(), name); ok && sym != nil {
			return sym, idx.GetFQN(sym), nil
		}
		// A top-level library name (ISQ) is qualified by nothing, so the index
		// registers it under the name itself.
		if matches := idx.LookupQualified(name); len(matches) == 1 {
			return matches[0], idx.GetFQN(matches[0]), nil
		}
		return nil, "", s.notFoundError(name, want...)
	case 1:
		return matches[0], idx.GetFQN(matches[0]), nil
	default:
		return nil, "", ambiguousError(name, matches, idx)
	}
}

// owningInstance returns the object a fully-qualified name belongs to: the
// object its qualifier names, since the last segment is the feature read from
// it. The second result is the found object's FQN, for reporting.
func (s *Session) owningInstance(fqn string) (*runtime.Instance, string) {
	segments := strings.Split(fqn, "::")
	if len(segments) < 2 {
		return nil, ""
	}
	return s.objectNamed(strings.Join(segments[:len(segments)-1], "::"))
}

// objectNamed returns the object a fully-qualified name denotes: the one
// materialized under it, or the longest instantiated prefix with the remaining
// segments walked through that instance's feature values, since a nested part is an
// object of its own. The second result is the found object's FQN, for reporting.
func (s *Session) objectNamed(fqn string) (*runtime.Instance, string) {
	inst, name, err := s.objectAt(fqn)
	if err != nil || inst == nil {
		return nil, ""
	}
	return inst, name
}

// featureChainSymbol resolves a qualified name whose later segments are members
// of the type of the segment before them (Outer::inner::b::c): the index
// registers a declaration only under its own owner, so such a chain is walked
// through the model's member lookup, which follows typing and specialization.
// The second result is the fully-qualified name the symbol is declared under.
func (s *Session) featureChainSymbol(name string) (*symbols.Symbol, string) {
	segments := strings.Split(name, "::")
	if len(segments) < 3 {
		return nil, ""
	}
	idx := s.browseIndex()
	ctx, err := s.getOrCreateRuntime()
	if idx == nil || err != nil {
		return nil, ""
	}
	model := ctx.Model()
	// The longest prefix a declaration answers to is the chain's root, so a
	// nested feature is preferred over the type it happens to share a name with.
	for i := len(segments) - 1; i > 0; i-- {
		matches := idx.LookupQualified(strings.Join(segments[:i], "::"))
		if len(matches) != 1 {
			continue
		}
		sym := matches[0]
		walked := true
		for _, seg := range segments[i:] {
			member, ok := model.LookupMember(sym, seg)
			if !ok || member == nil {
				walked = false
				break
			}
			sym = member
		}
		if !walked {
			continue
		}
		fqn := idx.GetFQN(sym)
		if fqn == "" {
			fqn = name
		}
		return sym, fqn
	}
	return nil, ""
}

// carrierLimit bounds the object graph a subject is searched through, so a
// deeply nested or richly connected model cannot make a lookup unbounded.
const carrierLimit = 2000

// carriesDeclaration reports whether an object of type typ carries the feature
// declared by decl: typ or a supertype of it is that declaration. Declarations
// are compared, not symbols, because the index and the document each build a
// symbol of their own for one declaration.
func carriesDeclaration(model *semantics.Model, typ *symbols.Symbol, decl ast.Node) bool {
	if typ == nil || decl == nil {
		return false
	}
	if typ.Decl == decl {
		return true
	}
	for _, sup := range model.AllSupertypes(typ) {
		if sup != nil && sup.Decl == decl {
			return true
		}
	}
	return false
}

// carrierInstances names the session's objects of the type declaring sym,
// sorted: an object of `part hot : Sensor` carries `Sensor::inRange`. Nested
// objects carry the features of their own type too, so `Spec::c` is carried by
// the `o::inner::b` a redefinition gave a value on, not only by a top-level
// object.
func (s *Session) carrierInstances(sym *symbols.Symbol) []string {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	declaring := sym.OwnerScope.Owner()
	if declaring == nil || declaring.Decl == nil || len(s.instances) == 0 {
		return nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil
	}
	model := ctx.Model()
	var names []string
	// A feature is read from the outermost object carrying it; its own nested
	// objects are of other types and are not searched again.
	s.walkObjects(ctx, func(cur carrier) bool {
		if !carriesDeclaration(model, cur.inst.Type, declaring.Decl) {
			return true
		}
		names = append(names, cur.name)
		return false
	})
	sort.Strings(names)
	return names
}

// walkObjects visits the session's objects and, while visit reports true, the
// objects they hold, breadth-first in name order and within carrierLimit.
func (s *Session) walkObjects(ctx *runtime.Context, visit func(carrier) bool) {
	seen := make(map[int64]bool, len(s.instances))
	queue := make([]carrier, 0, len(s.instances))
	for name, inst := range s.instances {
		if inst != nil {
			queue = append(queue, carrier{name: name, inst: inst})
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i].name < queue[j].name })
	for visited := 0; len(queue) > 0 && visited < carrierLimit; visited++ {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur.inst.ID] {
			continue
		}
		seen[cur.inst.ID] = true
		if visit(cur) {
			queue = append(queue, nestedObjects(ctx, cur)...)
		}
	}
}

// carrier is an object reachable from the session's objects, under the
// qualified name it is reached by.
type carrier struct {
	name string
	inst *runtime.Instance
}

// nestedObjects returns the objects held in an object's feature values, in feature-value-name
// order, each under the name it is reached by.
func nestedObjects(ctx *runtime.Context, of carrier) []carrier {
	fvs := make([]string, 0, len(of.inst.FeatureValues))
	for name := range of.inst.FeatureValues {
		fvs = append(fvs, name)
	}
	sort.Strings(fvs)
	out := make([]carrier, 0, len(fvs))
	for _, name := range fvs {
		// A part feature value holds its object only once it is asked for.
		fv, err := of.inst.GetFeatureValue(ctx, name)
		if err != nil || fv == nil {
			continue
		}
		id, isObject := fv.Value.Object()
		if !isObject {
			continue
		}
		child, ok := ctx.Instance(id)
		if !ok || child == nil {
			continue
		}
		out = append(out, carrier{name: of.name + "::" + name, inst: child})
	}
	return out
}

// AmbiguousSubjectError reports a feature or condition several of the session's
// objects carry, so which one an answer would be about is a question.
type AmbiguousSubjectError struct {
	Name     string
	Carriers []string
}

func (e *AmbiguousSubjectError) Error() string {
	return fmt.Sprintf("%s is carried by more than one object of this session (%s): name one of them, or start a session holding the one you mean",
		e.Name, strings.Join(e.Carriers, ", "))
}

// subjectFor is the object an answer about sym is about: the one
// instantiated under the name it was reached by, else the single carrier. Several
// carriers are an AmbiguousSubjectError; none leaves the answer about declared
// defaults. name is the spelling to report, fqn the resolved one.
func (s *Session) subjectFor(name, fqn string, sym *symbols.Symbol) (*runtime.Instance, string, error) {
	if inst, owner := s.owningInstance(fqn); inst != nil {
		return inst, owner, nil
	}
	carriers := s.carrierInstances(sym)
	switch len(carriers) {
	case 0:
		return nil, "", nil
	case 1:
		// A carrier may be nested, so it is reached the way any object name is.
		inst, owner := s.objectNamed(carriers[0])
		if inst == nil {
			return nil, "", nil
		}
		return inst, owner, nil
	default:
		return nil, "", &AmbiguousSubjectError{Name: name, Carriers: carriers}
	}
}

// unresolvedError reports a name nothing declares, in the wording every surface
// uses for it: the parser's diagnostic and the runtime's sentinel.
func unresolvedError(name string) error {
	return fmt.Errorf("%w: %s", runtime.ErrUnresolvedReference, name)
}

// AmbiguousNameError reports a name that matched more than one declaration. It
// is distinct from a name found nowhere: a command may look elsewhere for the
// latter, but must never answer about one of several candidates.
type AmbiguousNameError struct {
	Name string
	FQNs []string
}

func (e *AmbiguousNameError) Error() string {
	return fmt.Sprintf("symbol %q is ambiguous: %s (use a qualified name)", e.Name, strings.Join(e.FQNs, ", "))
}

// ambiguousError reports a name that matched more than one declaration, listing
// the candidates' fully-qualified names rather than picking one of them.
func ambiguousError(name string, matches []*symbols.Symbol, idx *symbols.Index) error {
	fqns := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, sym := range matches {
		fqn := notationName(idx.GetFQN(sym))
		if !seen[fqn] {
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	sort.Strings(fqns)
	return &AmbiguousNameError{Name: notationName(name), FQNs: fqns}
}

// nameTable maps every simple name the session documents declare outside a body
// to its declarations in scope-tree order, so a lookup costs nothing of the model's size.
type nameTable struct {
	scopes []*symbols.Scope // the trees the table was built from
	byName map[string][]*symbols.Symbol
}

// nameTable returns the table over the session's current documents, rebuilt
// when a submission or reset has replaced a scope tree.
func (s *Session) nameTable() *nameTable {
	scopes := s.docScopes()
	if s.names == nil || !slices.Equal(s.names.scopes, scopes) {
		s.names = buildNameTable(scopes)
	}
	return s.names
}

// buildNameTable tabulates every scope tree in turn, a scope's own members before
// its children's; a body-local scope is skipped with everything nested in it.
func buildNameTable(scopes []*symbols.Scope) *nameTable {
	t := &nameTable{scopes: scopes, byName: make(map[string][]*symbols.Symbol)}
	for _, scope := range scopes {
		t.collect(scope)
	}
	return t
}

func (t *nameTable) collect(scope *symbols.Scope) {
	if scope == nil || scope.BodyLocal() {
		return
	}
	for _, name := range scope.MemberNames() {
		t.byName[name] = append(t.byName[name], symbols.PreferDeclared(scope.LookupLocalAll(name))...)
	}
	for _, child := range scope.Children() {
		t.collect(child)
	}
}

// lookup returns every declaration of name in scope-tree order. The slice is
// the table's own and must not be modified.
func (t *nameTable) lookup(name string) []*symbols.Symbol {
	syms := t.byName[name]
	return syms[:len(syms):len(syms)]
}

// sorted returns every declared name, sorted.
func (t *nameTable) sorted() []string {
	out := make([]string, 0, len(t.byName))
	for name := range t.byName {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// docScopes returns the scope trees of the session's open documents, in
// buffer order, so a lookup reads both languages.
func (s *Session) docScopes() []*symbols.Scope {
	var out []*symbols.Scope
	for _, doc := range s.sessionDocs() {
		if doc.Scope != nil {
			out = append(out, doc.Scope)
		}
	}
	return out
}

// scopeSymbolForAny is scopeSymbolFor over several scope trees, first hit wins.
func scopeSymbolForAny(scopes []*symbols.Scope, decl ast.Node) *symbols.Symbol {
	for _, scope := range scopes {
		if sym := scopeSymbolFor(scope, decl); sym != nil {
			return sym
		}
	}
	return nil
}

// scopeSymbolFor returns the symbol in scope's tree declared by decl, or nil.
func scopeSymbolFor(scope *symbols.Scope, decl ast.Node) *symbols.Symbol {
	if scope == nil || decl == nil {
		return nil
	}
	for _, sym := range scope.Members() {
		if sym.Decl == decl {
			return sym
		}
	}
	for _, child := range scope.Children() {
		if sym := scopeSymbolFor(child, decl); sym != nil {
			return sym
		}
	}
	return nil
}
