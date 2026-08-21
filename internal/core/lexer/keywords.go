package lexer

import "github.com/Open-MBEE/OpenSysML/internal/core/source"

// keywords is the union of KerML and SysML lowercase keyword literals.
// A scanned ID matching a key is emitted as Kind==Keyword with KeywordID set.
var keywords = map[string]struct{}{}

// keywordsKerML and keywordsSysML are the per-language sets. Xtext reserves a
// literal only within the grammar that declares it, and the two grammars share
// only KerMLExpressions.xtext, so the sets differ (see keywordListKerML).
var (
	keywordsKerML = map[string]struct{}{}
	keywordsSysML = map[string]struct{}{}
)

func init() {
	for _, kw := range keywordList {
		keywords[kw] = struct{}{}
	}
	for _, kw := range keywordListKerML {
		keywordsKerML[kw] = struct{}{}
	}
	for _, kw := range keywordListSysML {
		keywordsSysML[kw] = struct{}{}
	}
}

// keywordSet returns the reserved set of a language. A source of unknown kind
// gets the union, so a name-less surface still lexes both notations.
func keywordSet(kind source.Kind) map[string]struct{} {
	switch kind {
	case source.KindKerML:
		return keywordsKerML
	case source.KindSysML:
		return keywordsSysML
	default:
		return keywords
	}
}

// IsKeyword reports whether s is a KerML or SysML keyword, and so cannot be
// used where a plain identifier is expected in either language.
func IsKeyword(s string) bool {
	_, ok := keywords[s]
	return ok
}

// IsKeywordIn reports whether s is reserved by one language's grammar.
func IsKeywordIn(s string, kind source.Kind) bool {
	_, ok := keywordSet(kind)[s]
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

// keywordListKerML is every lowercase literal of KerML.xtext plus the shared
// KerMLExpressions.xtext, generated from build/pilot-grammars (TestKeywordSets).
var keywordListKerML = []string{
	"about", "abstract", "alias", "all", "and", "as", "assoc", "behavior",
	"binding", "bool", "by", "chains", "class", "classifier", "comment",
	"composite", "conjugate", "conjugates", "conjugation", "connector", "const",
	"crosses", "datatype", "default", "dependency", "derived", "differences",
	"disjoining", "disjoint", "doc", "else", "end", "expr", "false", "feature",
	"featured", "featuring", "filter", "first", "flow", "for", "from",
	"function", "hastype", "if", "implies", "import", "in", "inout",
	"interaction", "intersects", "inv", "inverse", "inverting", "istype",
	"language", "library", "locale", "member", "meta", "metaclass", "metadata",
	"multiplicity", "namespace", "new", "nonunique", "not", "null", "of", "or",
	"ordered", "out", "package", "portion", "predicate", "private", "protected",
	"public", "redefines", "redefinition", "references", "rep", "return",
	"specialization", "specializes", "standard", "step", "struct",
	"subclassifier", "subset", "subsets", "subtype", "succession", "then", "to",
	"true", "type", "typed", "typing", "unions", "var", "xor",
}

// keywordListSysML is every lowercase literal of SysML.xtext plus the shared
// KerMLExpressions.xtext. SysML.xtext extends KerMLExpressions, not KerML, so
// `chains`, `type` and `namespace` are ordinary names in a `.sysml` file.
var keywordListSysML = []string{
	"about", "abstract", "accept", "action", "actor", "after", "alias", "all",
	"allocate", "allocation", "analysis", "and", "as", "assert", "assign",
	"assume", "at", "attribute", "bind", "binding", "by", "calc", "case",
	"comment", "concern", "connect", "connection", "constant", "constraint",
	"crosses", "decide", "def", "default", "defined", "dependency", "derived",
	"do", "doc", "else", "end", "entry", "enum", "event", "exhibit", "exit",
	"expose", "false", "filter", "first", "flow", "for", "fork", "frame",
	"from", "hastype", "if", "implies", "import", "in", "include", "individual",
	"inout", "interface", "istype", "item", "join", "language", "library",
	"locale", "loop", "merge", "message", "meta", "metadata", "new",
	"nonunique", "not", "null", "objective", "occurrence", "of", "or",
	"ordered", "out", "package", "parallel", "part", "perform", "port",
	"private", "protected", "public", "redefines", "ref", "references",
	"render", "rendering", "rep", "require", "requirement", "return", "satisfy",
	"send", "snapshot", "specializes", "stakeholder", "standard", "state",
	"subject", "subsets", "succession", "terminate", "then", "timeslice", "to",
	"transition", "true", "until", "use", "variant", "variation",
	"verification", "verify", "via", "view", "viewpoint", "when", "while",
	"xor",
}

// Keywords returns a copy of the KerML+SysML keyword list, for tooling
// (e.g. LSP completion). The returned slice is safe to mutate.
func Keywords() []string {
	out := make([]string, len(keywordList))
	copy(out, keywordList)
	return out
}
