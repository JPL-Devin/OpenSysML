package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
	// A name the notation reads is resolved by the text it names; anything else is
	// looked up as typed, so the failure reported is about what was typed.
	if plain, ok := plainName(name); ok {
		name = plain
	}
	doc := s.ws.Document(docName)
	var docScope *symbols.Scope
	if doc != nil {
		docScope = doc.Scope
	}
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
				if local := scopeSymbolFor(docScope, sym.Decl); local != nil {
					sym = local
				}
				return sym, fqn, nil
			}
			return nil, "", s.notFoundError(name)
		case 1:
			// The index owns its own scope tree; map the hit back onto the
			// document's tree so every command sees one symbol per declaration.
			sym := matches[0]
			fqn := idx.GetFQN(sym)
			if fqn == "" {
				fqn = name
			}
			if local := scopeSymbolFor(docScope, sym.Decl); local != nil {
				sym = local
			}
			return sym, fqn, nil
		default:
			return nil, "", ambiguousError(name, matches, idx)
		}
	}

	matches := collectInScopeTree(docScope, name)
	switch len(matches) {
	case 0:
		// A name the session declares nowhere may still be visible where the
		// prompt evaluates — through an import of that namespace.
		if sym, ok := resolve.New(idx).LookupName(s.promptScope(doc), name); ok && sym != nil {
			return sym, idx.GetFQN(sym), nil
		}
		// A top-level library name (ISQ) is qualified by nothing, so the index
		// registers it under the name itself.
		if matches := idx.LookupQualified(name); len(matches) == 1 {
			return matches[0], idx.GetFQN(matches[0]), nil
		}
		return nil, "", s.notFoundError(name)
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
	if fqn == "" {
		return nil, ""
	}
	segments := strings.Split(fqn, "::")
	for i := len(segments); i > 0; i-- {
		key := strings.Join(segments[:i], "::")
		inst, ok := s.instances[key]
		if !ok {
			continue
		}
		return s.walkFeatureValues(inst, key, segments[i:])
	}
	return nil, ""
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
		if carriesDeclaration(model, cur.inst.Type, declaring.Decl) {
			names = append(names, cur.name)
			// A feature is read from the outermost object carrying it; its own
			// nested objects are of other types and are not searched again.
			continue
		}
		queue = append(queue, nestedObjects(ctx, cur)...)
	}
	sort.Strings(names)
	return names
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

// walkFeatureValues follows a chain of part feature values from inst. An unwalkable segment
// yields no object, since binding to an ancestor would answer about the wrong one.
func (s *Session) walkFeatureValues(inst *runtime.Instance, name string, segments []string) (*runtime.Instance, string) {
	if len(segments) == 0 {
		return inst, name
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, ""
	}
	for _, seg := range segments {
		fv, serr := inst.GetFeatureValue(ctx, seg)
		if serr != nil || fv == nil {
			return nil, ""
		}
		// A variation feature value holds the object of the variant it selected.
		id, isObject := fv.Value.Object()
		if !isObject {
			return nil, ""
		}
		child, ok := ctx.Instance(id)
		if !ok {
			return nil, ""
		}
		inst, name = child, name+"::"+seg
	}
	return inst, name
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

// collectInScopeTree returns every symbol named name in scope or a nested
// scope. A body-local name is only visible inside its own body and is skipped.
func collectInScopeTree(scope *symbols.Scope, name string) []*symbols.Symbol {
	if scope == nil || scope.BodyLocal() {
		return nil
	}
	var out []*symbols.Symbol
	if syms := scope.LookupLocalAll(name); len(syms) > 0 {
		out = append(out, symbols.PreferDeclared(syms)...)
	}
	for _, child := range scope.Children() {
		out = append(out, collectInScopeTree(child, name)...)
	}
	return out
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
