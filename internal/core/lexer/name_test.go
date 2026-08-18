package lexer

import "testing"

func TestNameTextQuotesWhatIsNoBasicName(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{"vehicle", "vehicle"},
		{"Vehicle_2", "Vehicle_2"},
		{"My Vehicle", "'My Vehicle'"},
		{"2wheels", "'2wheels'"},
		{"", "''"},
		// A name spelling a keyword is no basic name, so it keeps its quotes.
		{"frame", "'frame'"},
		{"state", "'state'"},
		{"render", "'render'"},
		// An escape the notation wrote stays as it is, rather than being
		// escaped a second time.
		{`it\'s`, `'it\'s'`},
	} {
		if got := NameText(tc.name); got != tc.want {
			t.Errorf("NameText(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestQualifiedNameTextQuotesEachSegmentOnItsOwn(t *testing.T) {
	for _, tc := range []struct {
		fqn  string
		want string
	}{
		{"", ""},
		{"Demo::Vehicle", "Demo::Vehicle"},
		{"Demo::My Vehicle", "Demo::'My Vehicle'"},
		{"state::frame::ok", "'state'::'frame'::ok"},
	} {
		if got := QualifiedNameText(tc.fqn); got != tc.want {
			t.Errorf("QualifiedNameText(%q) = %q, want %q", tc.fqn, got, tc.want)
		}
	}
}
