package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// searchLimit bounds a %search listing: the library declares thousands of
// names, and a listing the user must scroll past discovers nothing.
const searchLimit = 40

// suggestionLimit bounds how many candidates a "did you mean" offers.
const suggestionLimit = 3

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
		sym := declaringSymbol(idx, fqn)
		if sym == nil {
			continue
		}
		matches = append(matches, match{
			fqn:    fqn,
			kind:   sym.Kind.String(),
			onName: strings.Contains(lastSegment(lower), want),
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
		out = append(out, fmt.Sprintf("%s  %s", m.fqn, m.kind))
	}
	if len(matches) > len(shown) {
		out = append(out, fmt.Sprintf("(%d more; narrow the search)", len(matches)-len(shown)))
	}
	return out, false, nil
}

// lastSegment returns the simple name a qualified name ends in.
func lastSegment(fqn string) string {
	if cut := strings.LastIndex(fqn, "::"); cut >= 0 {
		return fqn[cut+2:]
	}
	return fqn
}

// declaringSymbol returns the symbol fqn declares, and nil when fqn is only an
// import re-export of a declaration made elsewhere. The lookup is made from
// fqn itself so a private member is visible where it is declared.
func declaringSymbol(idx *symbols.Index, fqn string) *symbols.Symbol {
	for _, sym := range idx.LookupQualifiedFrom(fqn, fqn) {
		if sym != nil && idx.GetFQN(sym) == fqn {
			return sym
		}
	}
	return nil
}

// candidateScanLimit bounds how many same-named registrations are examined
// before ranking them; the library re-exports popular names dozens of times.
const candidateScanLimit = 200

// qualifiedCandidates returns the qualified names, shortest first, under which
// the index declares the simple name name.
func qualifiedCandidates(idx *symbols.Index, name string) []string {
	if name == "" {
		return nil
	}
	var out []string
	// A top-level name qualifies nothing, so it is not a suffix of itself.
	if declaringSymbol(idx, name) != nil {
		out = append(out, name)
	}
	for _, fqn := range idx.FQNsEndingIn(name, candidateScanLimit) {
		if declaringSymbol(idx, fqn) != nil {
			out = append(out, fqn)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) < len(out[j])
		}
		return out[i] < out[j]
	})
	if len(out) > suggestionLimit {
		out = out[:suggestionLimit]
	}
	return out
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

// annotateDiagnostics points an unresolved unqualified reference at the
// qualified name the index knows it under, copying the workspace's diagnostics.
func (s *Session) annotateDiagnostics(diags []passes.Diagnostic) []passes.Diagnostic {
	// Nothing to annotate leaves the index unbuilt: a submission that resolves
	// cleanly should not pay for indexing the library.
	if len(diags) == 0 {
		return diags
	}
	idx := s.symbolIndex()
	if idx == nil {
		return diags
	}
	out := make([]passes.Diagnostic, len(diags))
	copy(out, diags)
	for i, d := range out {
		name, ok := unresolvedName(d.Message)
		if !ok {
			continue
		}
		out[i].Message = withSuggestion(d.Message, name, qualifiedCandidates(idx, name))
	}
	return out
}

// unresolvedReferencePrefix is what the name-resolution pass reports an
// unresolvable reference as.
const unresolvedReferencePrefix = "unresolved reference: "

// unresolvedName returns the unqualified name an unresolved-reference message
// reports; a qualified one is left to the resolver, which explains it already.
func unresolvedName(msg string) (string, bool) {
	if !strings.HasPrefix(msg, unresolvedReferencePrefix) {
		return "", false
	}
	name := strings.TrimPrefix(msg, unresolvedReferencePrefix)
	if name == "" || strings.Contains(name, "::") || strings.Contains(name, " ") {
		return "", false
	}
	return name, true
}

// orList joins candidates as "a, b or c", for a message that offers a choice.
func orList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// nearest returns the candidates closest to word by edit distance, within the
// tolerance a typo of that length justifies, in distance then name order.
func nearest(word string, candidates []string) []string {
	tolerance := 1
	switch n := len([]rune(word)); {
	case n >= 9:
		tolerance = 3
	case n >= 6:
		tolerance = 2
	}
	type scored struct {
		name string
		dist int
	}
	var hits []scored
	for _, c := range candidates {
		if c == word {
			continue
		}
		if d := editDistance(strings.ToLower(word), strings.ToLower(c)); d <= tolerance {
			hits = append(hits, scored{name: c, dist: d})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].dist != hits[j].dist {
			return hits[i].dist < hits[j].dist
		}
		return hits[i].name < hits[j].name
	})
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.name)
		if len(out) == suggestionLimit {
			break
		}
	}
	return out
}

// editDistance is the Levenshtein distance between a and b, counting a
// transposition as one edit so a swapped pair reads as the single typo it is.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev2 := make([]int, len(br)+1)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				if t := prev2[j-2] + 1; t < cur[j] {
					cur[j] = t
				}
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// suggestCommand offers the meta commands closest to an unknown one.
func suggestCommand(cmd string) []string {
	return nearest(cmd, metaCommands())
}

// suggestSymbol offers the names closest to one the session could not find,
// its own declarations first as the likelier target of a typo.
func (s *Session) suggestSymbol(name string) []string {
	idx := s.browseIndex()
	if idx == nil {
		return nil
	}
	simple := lastSegment(name)
	if hits := nearest(simple, s.declaredSymbolNames()); len(hits) > 0 {
		return hits
	}
	seen := map[string]bool{}
	var lasts []string
	for _, fqn := range idx.FQNs() {
		if last := lastSegment(fqn); !seen[last] {
			seen[last] = true
			lasts = append(lasts, last)
		}
	}
	var out []string
	for _, last := range nearest(simple, lasts) {
		if cands := qualifiedCandidates(idx, last); len(cands) > 0 {
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
	err := unresolvedError(name)
	msg := err.Error()
	if !strings.Contains(name, "::") {
		if idx := s.browseIndex(); idx != nil {
			if qualified := withSuggestion(msg, name, qualifiedCandidates(idx, name)); qualified != msg {
				return suggestionError(err, msg, qualified)
			}
		}
	}
	return suggestionError(err, msg, withSuggestion(msg, name, s.suggestSymbol(name)))
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
		return fmt.Sprintf("unknown command %q — did you mean %s?", cmd, orList(cands))
	}
	return fmt.Sprintf("unknown command %q (try %%help)", cmd)
}

// withSuggestion appends "— did you mean …?" to a message about word when there
// are candidates to offer other than word itself.
func withSuggestion(msg, word string, candidates []string) string {
	offer := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c != word {
			offer = append(offer, c)
		}
	}
	if len(offer) == 0 {
		return msg
	}
	return msg + " — did you mean " + orList(offer) + "?"
}
