package lexer

import "strings"

// StringValue reads the text a STRING_VALUE token spells: the quotes come off
// and the backslash escapes KerML §8.2.2 defines stand for the characters they
// name. A backslash before anything else stands for that character itself.
func StringValue(raw string) string {
	if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') && raw[len(raw)-1] == raw[0] {
		raw = raw[1 : len(raw)-1]
	}
	if !strings.ContainsRune(raw, '\\') {
		return raw
	}
	var text strings.Builder
	text.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' || i+1 == len(raw) {
			text.WriteByte(raw[i])
			continue
		}
		i++
		switch raw[i] {
		case 'b':
			text.WriteByte('\b')
		case 't':
			text.WriteByte('\t')
		case 'n':
			text.WriteByte('\n')
		case 'f':
			text.WriteByte('\f')
		case 'r':
			text.WriteByte('\r')
		default:
			text.WriteByte(raw[i])
		}
	}
	return text.String()
}
