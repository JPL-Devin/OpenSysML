package lexer

import "strings"

// NameText writes a name as the notation spells it: a name that is not a basic
// name — one holding a space or punctuation, one spelling a keyword, an empty
// one — gets the quotes of an unrestricted name back (KerML §8.2.2). The text
// is quoted as it stands, since a name parsed from the notation still carries
// the escapes it was written with, so quoting is the exact inverse of parsing.
func NameText(name string) string {
	if IsIdentifier(name) && !IsKeyword(name) {
		return name
	}
	return "'" + name + "'"
}

// QualifiedNameText writes a qualified name segment by segment, since each
// segment is a name of its own and is quoted on its own. An empty name is
// written as it stands.
func QualifiedNameText(fqn string) string {
	if fqn == "" {
		return fqn
	}
	segments := strings.Split(fqn, "::")
	for i, segment := range segments {
		segments[i] = NameText(segment)
	}
	return strings.Join(segments, "::")
}
