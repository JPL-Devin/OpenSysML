package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// searchLimit bounds a %search listing: the library declares thousands of
// names, and a listing the user must scroll past discovers nothing.
const searchLimit = 40

// browseIndex is the session's symbol index, or one holding the standard
// library alone, so library symbols are reachable from an empty prompt.
func (s *Session) browseIndex() *symbols.Index {
	if idx := s.symbolIndex(); idx != nil {
		return idx
	}
	if s.idx == nil {
		s.idx = symbols.NewIndex()
		model.LoadStdlibInto(s.idx)
	}
	return s.idx
}

// doSearch lists the indexed symbols whose fully-qualified name contains
// substr, case-insensitively, with the kind of each.
func (s *Session) doSearch(substr string) ([]string, bool, error) {
	idx := s.browseIndex()
	if idx == nil {
		return []string{"no symbols to search"}, false, nil
	}
	want := strings.ToLower(substr)
	type match struct {
		fqn    string
		kind   string
		onName bool // the substring matched the symbol's own name, not an ancestor's
	}
	var matches []match
	for _, fqn := range idx.FQNs() {
		lower := strings.ToLower(fqn)
		if !strings.Contains(lower, want) {
			continue
		}
		// Only where a name is declared: the library re-exports most names
		// through wildcard imports, which would bury the declaration.
		sym := idx.Declaring(fqn)
		if sym == nil {
			continue
		}
		matches = append(matches, match{
			fqn:    fqn,
			kind:   sym.Kind.String(),
			onName: strings.Contains(suggest.LastSegment(lower), want),
		})
	}
	if len(matches) == 0 {
		return []string{fmt.Sprintf("no symbol matches %q", substr)}, false, nil
	}
	// A name that matched itself comes before one that matched through an
	// ancestor, and a shallower name before a deeper one.
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.onName != b.onName {
			return a.onName
		}
		if da, db := strings.Count(a.fqn, "::"), strings.Count(b.fqn, "::"); da != db {
			return da < db
		}
		if len(a.fqn) != len(b.fqn) {
			return len(a.fqn) < len(b.fqn)
		}
		return a.fqn < b.fqn
	})
	shown := matches
	if len(shown) > searchLimit {
		shown = shown[:searchLimit]
	}
	out := make([]string, 0, len(shown)+1)
	for _, m := range shown {
		// Spelled as the notation writes it, so a hit can be typed back into a
		// command that takes a name.
		out = append(out, fmt.Sprintf("%s  %s", notationName(m.fqn), m.kind))
	}
	if len(matches) > len(shown) {
		out = append(out, fmt.Sprintf("(%d more; narrow the search)", len(matches)-len(shown)))
	}
	return out, false, nil
}

// doBuiltins lists the library functions this build implements directly, which
// are callable whether or not the model imports the function libraries.
func (s *Session) doBuiltins() ([]string, bool, error) {
	all := runtime.Builtins()
	scalar := make([]string, 0, len(all))
	collection := make([]string, 0, len(all))
	for _, b := range all {
		if b.Collection {
			collection = append(collection, fmt.Sprintf("x->%s()  %s", b.Name, b.FQN))
			continue
		}
		scalar = append(scalar, fmt.Sprintf("%s(%s)  %s", b.Name, strings.Join(b.Params, ", "), b.FQN))
	}
	out := []string{"Scalar functions:"}
	out = append(out, scalar...)
	out = append(out, "", "Collection and control functions (also callable as name(x, ...)):")
	out = append(out, collection...)
	return append(out, "", "Every one is also callable by its qualified name, e.g. RealFunctions::sqrt(2.0)."), false, nil
}

// suggestCommand offers the meta commands closest to an unknown one.
func suggestCommand(cmd string) []string {
	return suggest.Nearest(cmd, metaCommands())
}

// suggestSymbol offers the names closest to one the session could not find,
// its own declarations first as the likelier target of a typo.
func (s *Session) suggestSymbol(name string) []string {
	idx := s.browseIndex()
	if idx == nil {
		return nil
	}
	simple := suggest.LastSegment(name)
	if hits := suggest.Nearest(simple, s.declaredSymbolNames()); len(hits) > 0 {
		return hits
	}
	var out []string
	for _, last := range suggest.Nearest(simple, suggest.SimpleNames(idx)) {
		if cands := suggest.Qualified(idx, last); len(cands) > 0 {
			out = append(out, cands[0])
		}
	}
	return out
}

// declaredSymbolNames returns the names the session document declares, at every
// nesting level, sorted.
func (s *Session) declaredSymbolNames() []string {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil
	}
	seen := map[string]bool{}
	var collect func(scope *symbols.Scope)
	collect = func(scope *symbols.Scope) {
		if scope == nil || scope.BodyLocal() {
			return
		}
		for _, n := range scope.MemberNames() {
			seen[n] = true
		}
		for _, child := range scope.Children() {
			collect(child)
		}
	}
	collect(doc.Scope)
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// notFoundError reports a name no declaration answers to, offering the
// qualified name the index does know it under, or the nearest spellings.
func (s *Session) notFoundError(name string) error {
	// Spelled as the notation writes it, so the name in the failure is the name
	// that was typed.
	err := unresolvedError(notationName(name))
	msg := err.Error()
	if !strings.Contains(name, "::") {
		if idx := s.browseIndex(); idx != nil {
			if qualified := suggest.With(msg, name, suggest.Qualified(idx, name)); qualified != msg {
				return suggestionError(err, msg, qualified)
			}
		}
	}
	return suggestionError(err, msg, suggest.With(msg, name, s.suggestSymbol(name)))
}

// suggestionError offers what suggested added to msg while keeping err's
// sentinel, which callers match on to tell a missing name from other failures.
func suggestionError(err error, msg, suggested string) error {
	if suggested == msg {
		return err
	}
	return fmt.Errorf("%w%s", err, strings.TrimPrefix(suggested, msg))
}

// unknownCommandLine reports an unrecognised meta command, naming the closest
// spellings when the input is near one.
func unknownCommandLine(cmd string) string {
	if cands := suggestCommand(cmd); len(cands) > 0 {
		return fmt.Sprintf("unknown command %q — did you mean %s?", cmd, suggest.OrList(cands))
	}
	return fmt.Sprintf("unknown command %q (try %%help)", cmd)
}
