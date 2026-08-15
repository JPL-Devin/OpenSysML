package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
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
func (s *Session) lookupSymbol(name string) (*symbols.Symbol, string, error) {
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
// longest instantiated prefix, then the remaining segments walked through that
// instance's slots, since a nested part is an object of its own. The second
// result is the found object's FQN, for reporting.
func (s *Session) owningInstance(fqn string) (*runtime.Instance, string) {
	segments := strings.Split(fqn, "::")
	if len(segments) < 2 {
		return nil, ""
	}
	owner := segments[:len(segments)-1]
	for i := len(owner); i > 0; i-- {
		key := strings.Join(owner[:i], "::")
		inst, ok := s.instances[key]
		if !ok {
			continue
		}
		return s.walkSlots(inst, key, owner[i:])
	}
	return nil, ""
}

// carrierInstances names the session's objects of the type declaring sym, sorted:
// an object of `part hot : Sensor` carries `Sensor::inRange`.
func (s *Session) carrierInstances(sym *symbols.Symbol) []string {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	declaring := sym.OwnerScope.Owner()
	if declaring == nil || len(s.instances) == 0 {
		return nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil
	}
	model := ctx.Model()
	var names []string
	for name, inst := range s.instances {
		if inst == nil || !model.Conforms(inst.Type, declaring) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
		return s.instances[carriers[0]], carriers[0], nil
	default:
		return nil, "", &AmbiguousSubjectError{Name: name, Carriers: carriers}
	}
}

// walkSlots follows a chain of part slots from inst. An unwalkable segment
// yields no object, since binding to an ancestor would answer about the wrong one.
func (s *Session) walkSlots(inst *runtime.Instance, name string, segments []string) (*runtime.Instance, string) {
	if len(segments) == 0 {
		return inst, name
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, ""
	}
	for _, seg := range segments {
		slot, serr := inst.GetSlot(ctx, seg)
		if serr != nil || slot == nil {
			return nil, ""
		}
		// A variation slot holds the object of the variant it selected.
		id, isObject := slot.Value.Object()
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
		fqn := idx.GetFQN(sym)
		if !seen[fqn] {
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	sort.Strings(fqns)
	return &AmbiguousNameError{Name: name, FQNs: fqns}
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
