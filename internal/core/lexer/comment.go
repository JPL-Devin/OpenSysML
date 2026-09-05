package lexer

import "strings"

// CommentBody reads the prose a comment token carries: the `/* */`, `//* */` or
// `//` delimiters come off, as does the `*` a block comment runs down its left
// edge and the indentation of each line. Lines keep their breaks; a `**` that
// opens Markdown emphasis is kept, as it is the author's rather than decoration.
func CommentBody(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, open := range []string{"//*", "/*"} {
		if rest, ok := strings.CutPrefix(raw, open); ok {
			raw = strings.TrimSuffix(rest, "*/")
			break
		}
	}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		if !strings.HasPrefix(line, "**") {
			line = strings.TrimPrefix(line, "*")
		}
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
