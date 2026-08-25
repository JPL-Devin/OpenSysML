package rdf

import (
	"strings"
	"unicode/utf8"
)

const lowerHex = "0123456789abcdef"

// idByte reports whether a byte stands for itself in an encoded element id.
func idByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}

// EncodeElementID encodes a qualified name as an element id in [A-Za-z0-9_-]+:
// `::` becomes `__`, a byte in [A-Za-z0-9-] stands for itself, and every other
// byte — a literal '_' included — becomes '_' plus two lowercase hex digits.
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

// owningMembershipSuffix ends a membership's id. EncodeElementID cannot produce
// `_o` — a '_' there starts `__` or a hex escape — so the two id spaces are disjoint.
const owningMembershipSuffix = "_om"

// OwningMembershipID returns the id of the OwningMembership that owns the
// member with the given qualified name. The membership sits between exactly one
// namespace and one member, so the member's own identity determines it.
func OwningMembershipID(memberQualifiedName string) string {
	return EncodeElementID(memberQualifiedName) + owningMembershipSuffix
}

// DecodeOwningMembershipID reverses OwningMembershipID, returning the qualified
// name of the member the membership owns.
func DecodeOwningMembershipID(id string) (string, bool) {
	member, found := strings.CutSuffix(id, owningMembershipSuffix)
	if !found {
		return "", false
	}
	return DecodeElementID(member)
}

// expressionPositionSeparator joins an expression node's id to the position it
// holds. EncodeElementID cannot produce `_p` either, so an expression id
// collides with neither an element id nor a membership id.
const expressionPositionSeparator = "_p"

// ExpressionNodeID returns the id of the expression node at position under the
// element or outer node with id owner. Positions nest, so the ids compose, and
// the result stays in [A-Za-z0-9_-]+ so a consumer restricting ids to that can
// address the node.
func ExpressionNodeID(owner, position string) string {
	return owner + expressionPositionSeparator + EncodeElementID(position)
}

// DecodeExpressionNodeID reverses ExpressionNodeID, returning the qualified name
// of the element the node belongs to and the positions leading down to it.
func DecodeExpressionNodeID(id string) (string, []string, bool) {
	parts := strings.Split(id, expressionPositionSeparator)
	if len(parts) < 2 {
		return "", nil, false
	}
	owner, ok := DecodeElementID(parts[0])
	if !ok {
		return "", nil, false
	}
	positions := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		position, ok := DecodeElementID(part)
		if !ok {
			return "", nil, false
		}
		positions = append(positions, position)
	}
	return owner, positions, true
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
