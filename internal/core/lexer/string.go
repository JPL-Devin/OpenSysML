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

// StringText writes a value as a STRING_VALUE token: quoted, with the
// characters KerML §8.2.2 gives an escape written as that escape.
func StringText(value string) string {
	var raw strings.Builder
	raw.Grow(len(value) + 2)
	raw.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case '"', '\\':
			raw.WriteByte('\\')
			raw.WriteByte(c)
		case '\b':
			raw.WriteString(`\b`)
		case '\t':
			raw.WriteString(`\t`)
		case '\n':
			raw.WriteString(`\n`)
		case '\f':
			raw.WriteString(`\f`)
		case '\r':
			raw.WriteString(`\r`)
		default:
			raw.WriteByte(c)
		}
	}
	raw.WriteByte('"')
	return raw.String()
}
