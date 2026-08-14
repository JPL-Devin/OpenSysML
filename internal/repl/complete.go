package repl

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
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
		out = out[:completionLimit]
	}
	return Completion{Candidates: out, Prefix: word}
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
// characters a qualified SysML name is written with.
func nameWord(head string) string {
	i := len(head)
	for i > 0 {
		r := head[i-1]
		if r == '_' || r == ':' || r == '\'' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			i--
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

// pathCompletions returns the files and directories the typed path can name.
// Candidates extend what was typed, so a directory keeps its trailing slash and
// a "~" is left as written.
func pathCompletions(word string) []string {
	dir, base := path.Split(word)
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
			name += "/"
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
		if name == "" || seen[name] || !strings.HasPrefix(name, word) {
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
			// would answer with the whole library.
			if cut := strings.Index(fqn[len(word):], "::"); cut >= 0 {
				add(fqn[:len(word)+cut])
				continue
			}
			add(fqn)
		}
	}
	return out
}
