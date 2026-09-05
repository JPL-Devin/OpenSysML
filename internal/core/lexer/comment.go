package lexer

import "strings"

// CommentBody reads the prose a comment token carries: the `/* */`, `//* */` or
// `//` delimiters come off, as does each line's indentation and the `*` margin a
// block comment runs down its left edge. Lines keep their breaks.
func CommentBody(raw string) string {
	raw = strings.TrimSpace(raw)
	block := false
	for _, open := range []string{"//*", "/*"} {
		if rest, ok := strings.CutPrefix(raw, open); ok {
			raw, block = strings.TrimSuffix(rest, "*/"), true
			break
		}
	}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !block {
			line = strings.TrimPrefix(line, "//")
		}
		lines[i] = strings.TrimSpace(line)
	}
	if block && hasMargin(lines[1:]) {
		for i, line := range lines[1:] {
			lines[i+1] = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// hasMargin reports whether the continuation lines of a block comment all open
// with a margin `*`; a `*` on only some lines, or one glued to text, is the
// author's (a bullet, emphasis) and stays.
func hasMargin(lines []string) bool {
	margin := false
	for _, line := range lines {
		switch {
		case line == "":
		case line == "*", strings.HasPrefix(line, "* "), strings.HasPrefix(line, "*\t"):
			margin = true
		default:
			return false
		}
	}
	return margin
}
