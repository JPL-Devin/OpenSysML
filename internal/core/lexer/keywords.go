package lexer

// keywords is the union of KerML and SysML lowercase keyword literals.
// A scanned ID matching a key is emitted as Kind==Keyword with KeywordID set.
var keywords = map[string]struct{}{}

func init() {
	for _, kw := range keywordList {
		keywords[kw] = struct{}{}
	}
}

// IsKeyword reports whether s is a KerML or SysML keyword, and so cannot be
// used where a plain identifier is expected.
func IsKeyword(s string) bool {
	_, ok := keywords[s]
	return ok
}

// IsIdentifier reports whether s can be written as a basic name (KerML §8.2.2);
// a name that cannot needs the quotes of an unrestricted name.
func IsIdentifier(s string) bool {
	if s == "" || !isIdentStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isIdentCont(s[i]) {
			return false
		}
	}
	return true
}

// Words the grammar uses in one position only are not listed here: they are
// matched contextually by the parser and are ordinary names elsewhere, as
// `point`, `chain` and `var` are (see docs/reference/grammar/README.md).
//
// Neither are words that are a literal in none of the pinned grammars, such as
// the state and action notation OpenSysML adds (`region`, `initial`, `done`):
// reserving those only stops models naming features with them. See
// docs/reference/grammar/conformance-audit.md.
var keywordList = []string{
	// KerML + SysML union (deduplicated). Contextual keywords included;
	// parser disambiguates identifier usage.
	"about", "abstract", "accept", "action", "actor", "after", "alias", "all",
	"allocate", "allocation", "analysis", "and", "as", "assert", "assign",
	"assoc", "assume", "at", "attribute", "behavior", "bind", "binding", "bool",
	"by", "calc", "case", "chains", "class", "classifier", "comment", "composite",
	"concern", "conjugate", "conjugates", "conjugation", "connect", "connection",
	"connector", "const", "constant", "constraint", "crosses", "datatype",
	"decide", "def", "default", "defined", "dependency", "derived",
	"differences",
	"disjoining", "disjoint", "do", "doc", "else", "end", "entry", "enum", "event",
	"exhibit", "exit", "expose", "expr", "false", "feature", "featured",
	"featuring", "filter", "first", "flow", "for", "fork", "frame", "from",
	"function", "hastype", "if", "implies", "import", "in", "include",
	"individual", "inout", "interaction", "interface", "intersects", "inv",
	"inverse", "inverting", "istype", "item", "join", "language", "library",
	"locale", "loop", "member", "merge", "message", "meta", "metaclass",
	"metadata", "multiplicity", "namespace", "new", "nonunique", "not", "null",
	"objective", "occurrence", "of", "or", "ordered", "out", "package", "parallel",
	"part", "perform", "port", "portion", "predicate", "private", "protected",
	"public", "redefines", "redefinition", "ref", "references", "render",
	"rendering", "rep", "require", "requirement", "return", "satisfy", "send",
	"snapshot", "specialization", "specializes", "stakeholder", "standard",
	"state", "step", "struct", "subclassifier", "subject", "subset", "subsets",
	"subtype", "succession", "terminate", "then", "timeslice", "to", "transition",
	"true", "type", "typed", "typing", "unions", "until", "use", "variant",
	"variation", "verification", "verify", "via", "view", "viewpoint", "when",
	"while", "xor",
}

// Keywords returns a copy of the KerML+SysML keyword list, for tooling
// (e.g. LSP completion). The returned slice is safe to mutate.
func Keywords() []string {
	out := make([]string, len(keywordList))
	copy(out, keywordList)
	return out
}
