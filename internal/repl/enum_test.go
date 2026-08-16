package repl

import "testing"

// TestSlotsEnumerationLiteral: an enum-typed default materializes, and the slot
// shows the literal as the model writes it rather than a number or an error.
func TestSlotsEnumerationLiteral(t *testing.T) {
	s := loadFixture(t, "testdata/enum_package.sysml")
	run(t, s, "%instantiate D::Car")

	got := run(t, s, "%slots D::Car")
	wants(t, got,
		"c = Color::red",
		"l = Level::high",
		"isRed = true",
		"grade = 9",
	)
	rejects(t, got, "has no value", "<unknown>")
}

// A literal evaluates on its own, outside any instance.
func TestEvalEnumerationLiteral(t *testing.T) {
	s := loadFixture(t, "testdata/enum_package.sysml")

	wants(t, run(t, s, "%eval D::Color::red"), "= Color::red")
	wants(t, run(t, s, "%eval D::Level::low.n"), "= 1")
}
