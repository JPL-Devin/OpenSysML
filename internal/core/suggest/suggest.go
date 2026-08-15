// Package suggest offers the spellings a name that did not resolve may have
// meant: the qualified names the index declares it under, or the nearest
// spellings by edit distance. Shared by every surface reporting an unresolved name.
package suggest

import (
	"sort"
	"strings"
	"unicode"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Limit bounds how many candidates a "did you mean" offers.
const Limit = 3

// scanLimit bounds how many same-named registrations are ranked; the library
// re-exports popular names many times.
const scanLimit = 200

// Qualified returns the qualified names, shortest first, under which the index
// declares the simple name name: `Integer` is declared as `ScalarValues::Integer`.
func Qualified(idx *symbols.Index, name string) []string {
	if idx == nil || name == "" {
		return nil
	}
	var out []string
	// A top-level name qualifies nothing, so it is not a suffix of itself.
	if idx.Declaring(name) != nil {
		out = append(out, name)
	}
	for _, fqn := range idx.FQNsEndingIn(name, scanLimit) {
		if typable(fqn) && idx.Declaring(fqn) != nil {
			out = append(out, fqn)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) < len(out[j])
		}
		return out[i] < out[j]
	})
	if len(out) > Limit {
		out = out[:Limit]
	}
	return out
}

// typable reports whether every segment of fqn is an identifier, so the
// candidate can be written as it reads: an operator member such as
// `BaseFunctions::#::index` is no use as a suggestion.
func typable(fqn string) bool {
	for _, seg := range strings.Split(fqn, "::") {
		if seg == "" {
			return false
		}
		for i, r := range seg {
			if unicode.IsLetter(r) || r == '_' || i > 0 && unicode.IsDigit(r) {
				continue
			}
			return false
		}
	}
	return true
}

// SimpleNames returns every simple name the index registers, sorted.
func SimpleNames(idx *symbols.Index) []string {
	if idx == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, fqn := range idx.FQNs() {
		if last := LastSegment(fqn); !seen[last] {
			seen[last] = true
			out = append(out, last)
		}
	}
	sort.Strings(out)
	return out
}

// LastSegment returns the simple name a qualified name ends in.
func LastSegment(fqn string) string {
	if cut := strings.LastIndex(fqn, "::"); cut >= 0 {
		return fqn[cut+2:]
	}
	return fqn
}

// Nearest returns the candidates closest to word by edit distance, within the
// tolerance a typo of that length justifies, in distance then name order.
func Nearest(word string, candidates []string) []string {
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
		if d := EditDistance(strings.ToLower(word), strings.ToLower(c)); d <= tolerance {
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
		if len(out) == Limit {
			break
		}
	}
	return out
}

// EditDistance is the Levenshtein distance between a and b, counting a
// transposition as one edit.
func EditDistance(a, b string) int {
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

// OrList joins candidates as "a, b or c", for a message that offers a choice.
func OrList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// With appends "— did you mean …?" to a message about word, ignoring a
// candidate equal to word.
func With(msg, word string, candidates []string) string {
	offer := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c != word {
			offer = append(offer, c)
		}
	}
	if len(offer) == 0 {
		return msg
	}
	return msg + " — did you mean " + OrList(offer) + "?"
}
