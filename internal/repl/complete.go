package repl

import (
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// completionLimit bounds a completion answer: the library registers tens of
// thousands of names, and a terminal cannot usefully offer all of them.
const completionLimit = 200

// Completion answers a completion request: the words that can replace the one
// being typed, and the part of it already typed, which every candidate starts
// with.
type Completion struct {
	Candidates []string
	Prefix     string
}

// Complete offers the words that can continue line at byte offset pos: the meta
// commands where a command is being typed, file paths where %load and %save
// take one, and otherwise the names the session and the library declare.
func (s *Session) Complete(line string, pos int) Completion {
	// Held because completing builds the library index, and readline asks from
	// its input goroutine while the loop may be evaluating the previous line.
	s.mu.Lock()
	defer s.mu.Unlock()

	if pos < 0 || pos > len(line) {
		pos = len(line)
	}
	head := line[:pos]
	command := firstToken(head)

	// A "%" word is a command only while it is still the only word: after it,
	// the line is arguments.
	if word := lastField(head); strings.HasPrefix(word, "%") && word == command {
		return completion(word, matchingPrefix(metaCommands(), word))
	}
	if command == "%load" || command == "%save" {
		word := lastField(head)
		return completion(word, pathCompletions(word))
	}
	word := nameWord(head)
	return completion(word, s.nameCompletions(word))
}

// completion assembles an answer in name order, bounding how many candidates
// are offered.
func completion(word string, candidates []string) Completion {
	out := append([]string(nil), candidates...)
	sort.Strings(out)
	if len(out) > completionLimit {
		// The prompt inserts what the candidates share, so dropping matches
		// must not lengthen it: where it would, only what every match shares
		// is offered, and nothing where that is what is already typed.
		shared := sharedPrefix(out)
		out = out[:completionLimit]
		if len(sharedPrefix(out)) > len(shared) {
			if shared == word {
				return Completion{Prefix: word}
			}
			out = []string{shared}
		}
	}
	return Completion{Candidates: out, Prefix: word}
}

// sharedPrefix returns the longest prefix every candidate begins with.
func sharedPrefix(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	prefix := candidates[0]
	for _, c := range candidates[1:] {
		for !strings.HasPrefix(c, prefix) {
			// Trimmed a character at a time: a prefix cut mid-character is
			// inserted at the prompt as a replacement character.
			_, size := utf8.DecodeLastRuneInString(prefix)
			prefix = prefix[:len(prefix)-size]
		}
	}
	return prefix
}

// firstToken returns the first whitespace-separated token of a line.
func firstToken(line string) string {
	if fields := strings.Fields(line); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// lastField returns the text after the last whitespace, which is the word a
// path is being typed into.
func lastField(head string) string {
	if cut := strings.LastIndexAny(head, " \t"); cut >= 0 {
		return head[cut+1:]
	}
	return head
}

// nameWord returns the name being typed at the end of head: the trailing run of
// characters a qualified SysML name is written with. Scanned by rune, so a name
// holding a letter outside ASCII is not cut in the middle of it.
func nameWord(head string) string {
	i := len(head)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(head[:i])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if r == '_' || r == ':' || r == '\'' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			i -= size
			continue
		}
		break
	}
	return head[i:]
}

// matchingPrefix returns the candidates that start with prefix.
func matchingPrefix(candidates []string, prefix string) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// splitPath splits a typed path after its last separator, accepting either
// separator so a path is split the way the platform reads it.
func splitPath(word string) (dir, base string) {
	for i := len(word) - 1; i >= 0; i-- {
		if os.IsPathSeparator(word[i]) {
			return word[:i+1], word[i+1:]
		}
	}
	return "", word
}

// pathCompletions returns the files and directories the typed path can name.
// Candidates extend what was typed, so a directory keeps its trailing separator
// and a "~" is left as written.
func pathCompletions(word string) []string {
	dir, base := splitPath(word)
	read := dir
	if read == "" {
		read = "."
	}
	entries, err := os.ReadDir(expandHome(read))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if e.IsDir() {
			// The separator already typed, so a candidate reads back the way
			// the path was written.
			sep := byte(os.PathSeparator)
			if dir != "" {
				sep = dir[len(dir)-1]
			}
			name += string(sep)
		}
		out = append(out, dir+name)
	}
	return out
}

// nameCompletions returns the names that can continue word: the session's own
// declarations, the next segment of a qualified library name, and the library
// functions callable by their unqualified name.
func (s *Session) nameCompletions(word string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		// A candidate equal to the typed word inserts nothing and empties the
		// prefix readline shares between the real candidates.
		if name == "" || name == word || seen[name] || !strings.HasPrefix(name, word) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	for _, name := range s.declaredSymbolNames() {
		add(name)
	}
	for _, b := range runtime.Builtins() {
		add(b.Name)
	}
	if idx := s.browseIndex(); idx != nil {
		for _, fqn := range idx.FQNs() {
			if !strings.HasPrefix(fqn, word) {
				continue
			}
			// One segment at a time: completing "ISQ" to every name under it
			// would answer with the whole library. A word already spelling a
			// namespace is answered with that namespace's members.
			rest := fqn[len(word):]
			skip := 0
			if strings.HasPrefix(rest, "::") {
				skip = len("::")
			}
			if cut := strings.Index(rest[skip:], "::"); cut >= 0 {
				add(fqn[:len(word)+skip+cut])
				continue
			}
			add(fqn)
		}
	}
	return out
}
