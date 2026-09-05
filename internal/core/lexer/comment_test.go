package lexer

import "testing"

func TestCommentBody(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"block", "/* The mission shall return the crew. */", "The mission shall return the crew."},
		{"note", "//* a note */", "a note"},
		{"line", "// a line", "a line"},
		{"empty", "/* */", ""},
		{"starred_lines", "/*\n\t * first line\n\t * second line\n\t */", "first line\nsecond line"},
		{"indented_lines", "/* first\n\t\t   second\n\t*/", "first\nsecond"},
		{"emphasis_kept", "/* **bold** start */", "**bold** start"},
		{"line_comment_lines", "// one\n// two", "one\ntwo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CommentBody(tc.raw); got != tc.want {
				t.Errorf("CommentBody(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
