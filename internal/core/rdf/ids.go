package rdf

import (
	"strings"
	"unicode/utf8"
)

// Element ids are the last segment of an element IRI, and the id a consumer
// such as Flexo MMS derives from it, so they are restricted to [A-Za-z0-9_-].
// EncodeElementID maps a qualified name into that alphabet reversibly, with
// '_' as the escape character: `::` becomes `__`, a byte in [A-Za-z0-9-]
// stands for itself, and every other byte — a literal '_' included — becomes
// '_' plus two lowercase hex digits. Distinct names yield distinct ids.

const lowerHex = "0123456789abcdef"

// idByte reports whether a byte stands for itself in an encoded element id.
func idByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}

// EncodeElementID encodes a qualified name as an element id in [A-Za-z0-9_-]+.
func EncodeElementID(qualifiedName string) string {
	var b strings.Builder
	for i := 0; i < len(qualifiedName); i++ {
		c := qualifiedName[i]
		switch {
		case c == ':' && i+1 < len(qualifiedName) && qualifiedName[i+1] == ':':
			b.WriteString("__")
			i++
		case idByte(c):
			b.WriteByte(c)
		default:
			b.WriteByte('_')
			b.WriteByte(lowerHex[c>>4])
			b.WriteByte(lowerHex[c&0x0f])
		}
	}
	return b.String()
}

// DecodeElementID reverses EncodeElementID, reporting whether id is a
// well-formed encoding. A malformed escape, a byte outside [A-Za-z0-9_-] or
// an invalid UTF-8 result is rejected rather than guessed at.
func DecodeElementID(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c != '_' {
			if !idByte(c) {
				return "", false
			}
			b.WriteByte(c)
			continue
		}
		if i+2 < len(id) {
			hi := strings.IndexByte(lowerHex, id[i+1])
			lo := strings.IndexByte(lowerHex, id[i+2])
			if hi >= 0 && lo >= 0 {
				b.WriteByte(byte(hi<<4 | lo))
				i += 2
				continue
			}
		}
		if i+1 < len(id) && id[i+1] == '_' {
			b.WriteString("::")
			i++
			continue
		}
		return "", false
	}
	name := b.String()
	if !utf8.ValidString(name) {
		return "", false
	}
	return name, true
}
