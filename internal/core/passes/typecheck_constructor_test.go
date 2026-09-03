package passes

import (
	"strings"
	"testing"
)

// constructorDiags is every type-tier diagnostic the full registry, library
// loaded, reports for src.
func constructorDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, d := range analyzeAll(t, "ctor.sysml", src) {
		if d.Source == "type" {
			out = append(out, d)
		}
	}
	return out
}

// constructorPrelude declares a payload type with an inherited feature and a
// receiver to send it to.
const constructorPrelude = `package P {
	private import ScalarValues::*;
	item def Base { attribute a : Integer; }
	item def Telemetry :> Base { attribute frames : Integer; attribute label : String; }
	part def Station;
	action def Downlink {
		part ground : Station;
		attribute frames : Integer;
		%s
	}
}`

func constructorModel(send string) string {
	return strings.Replace(constructorPrelude, "%s", send, 1)
}

// assertOneConstructorDiag checks that got is one error spanning at whose
// message reads want.
func assertOneConstructorDiag(t *testing.T, src string, got []Diagnostic, at, want string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("expected one diagnostic, got %+v", got)
	}
	d := got[0]
	if d.Severity != SeverityError {
		t.Errorf("severity %v, want an error", d.Severity)
	}
	if spanned := strings.TrimSpace(src[d.Span.Offset:d.Span.End()]); spanned != at {
		t.Errorf("reported at %q, want %q: %q", spanned, at, d.Message)
	}
	if !strings.Contains(d.Message, want) {
		t.Errorf("message %q does not read %q", d.Message, want)
	}
}

// A constructor's arguments bind the constructed type's features: its own in
// declaration order, then the inherited ones, by position or by label.
func TestConstructorWellFormedArgumentsAreSilent(t *testing.T) {
	cases := map[string]string{
		"no arguments":                              `send new Telemetry() to ground;`,
		"positional own features":                   `send new Telemetry(3, "hi") to ground;`,
		"positional through parent":                 `send new Telemetry(3, "hi", 7) to ground;`,
		"labelled own and inherited":                `send new Telemetry(label = "hi", a = 7) to ground;`,
		"labelled by the sender's same-named value": `send new Telemetry(frames = frames) to ground;`,
		"labelled with a calc":                      `send new Telemetry(frames = frames + 1) to ground;`,
		"untyped feature":                           `item def Note { attribute text; } send new Note("hi") to ground;`,
		"bound to an attribute":                     `item t : Telemetry = new Telemetry(frames = 3);`,
	}
	for name, send := range cases {
		t.Run(name, func(t *testing.T) {
			if got := constructorDiags(t, constructorModel(send)); len(got) != 0 {
				t.Errorf("unexpected diagnostics: %+v", got)
			}
		})
	}
}

// A positional argument beyond the constructed type's features binds nothing,
// which the first such argument reports.
func TestConstructorTooManyPositionalArgumentsIsReported(t *testing.T) {
	src := constructorModel(`send new Telemetry(3, "hi", 7, 9) to ground;`)
	assertOneConstructorDiag(t, src, constructorDiags(t, src), "9", "new Telemetry takes 3 argument(s), found 4")
}

// A feature is bound once: a second label for it is reported at that label,
// also when the two labels spell the feature differently.
func TestConstructorDuplicateBindingIsReported(t *testing.T) {
	cases := map[string]struct{ send, at string }{
		"repeated label":  {send: `send new Telemetry(frames = 3, frames = 4) to ground;`, at: "frames = 4"},
		"qualified label": {send: `send new Telemetry(frames = 3, Telemetry::frames = 4) to ground;`, at: "Telemetry::frames = 4"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			src := constructorModel(c.send)
			got := constructorDiags(t, src)
			if len(got) != 1 {
				t.Fatalf("expected one diagnostic, got %+v", got)
			}
			if !strings.HasPrefix(src[got[0].Span.Offset:], c.at) {
				t.Errorf("reported at %q, want the label of %q", src[got[0].Span.Offset:got[0].Span.End()], c.at)
			}
			if want := "frames of Telemetry is already bound"; !strings.Contains(got[0].Message, want) {
				t.Errorf("message %q does not read %q", got[0].Message, want)
			}
		})
	}
}

// An argument whose scalar type cannot bind its feature is reported at the
// argument, whether it binds by position or by label, and the feature's type is
// read where the feature is declared.
func TestConstructorArgumentTypeMismatchIsReported(t *testing.T) {
	cases := map[string]struct{ send, at, want string }{
		"positional":      {send: `send new Telemetry("hi") to ground;`, at: `"hi"`, want: "frames of Telemetry expects Integer, found String"},
		"labelled":        {send: `send new Telemetry(label = 3) to ground;`, at: "3", want: "label of Telemetry expects String, found"},
		"inherited":       {send: `send new Telemetry(a = "hi") to ground;`, at: `"hi"`, want: "a of Telemetry expects Integer, found String"},
		"outside a send":  {send: `item t : Telemetry = new Telemetry(frames = "hi");`, at: `"hi"`, want: "frames of Telemetry expects Integer, found String"},
		"in a transition": {send: `state def S { part g : Station; state s; transition first s do send new Telemetry("hi") to g then s; }`, at: `"hi"`, want: "frames of Telemetry expects Integer"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			src := constructorModel(c.send)
			assertOneConstructorDiag(t, src, constructorDiags(t, src), c.at, c.want)
		})
	}
}

// A qualified label is resolved in scope and must still name a feature of the
// constructed type.
func TestConstructorQualifiedLabelMustBeAFeatureOfTheType(t *testing.T) {
	src := constructorModel(`send new Telemetry(Downlink::frames = 3) to ground;`)
	assertOneConstructorDiag(t, src, constructorDiags(t, src), "Downlink::frames", "Downlink::frames is not a feature of Telemetry")
	silent := constructorModel(`send new Telemetry(P::Telemetry::frames = 3, Base::a = 7) to ground;`)
	if got := analyzeAll(t, "ctor.sysml", silent); len(got) != 0 {
		t.Errorf("unexpected diagnostics: %+v", got)
	}
}

// A label naming no feature of the constructed type is the resolver's report;
// the type tier adds nothing to it.
func TestConstructorUnknownLabelIsReportedOnce(t *testing.T) {
	src := constructorModel(`send new Telemetry(count = 3) to ground;`)
	all := analyzeAll(t, "ctor.sysml", src)
	if len(all) != 1 || !strings.Contains(all[0].Message, "count") {
		t.Fatalf("expected the unresolved label alone, got %+v", all)
	}
	if got := constructorDiags(t, src); len(got) != 0 {
		t.Errorf("the type tier reported %+v as well", got)
	}
}

// A constructed library type is left to the runtime; its features are the
// library's, which the checker does not bind.
func TestConstructorOfLibraryTypeIsSilent(t *testing.T) {
	src := constructorModel(`private import Items::*; item i : Item = new Item();`)
	if got := constructorDiags(t, src); len(got) != 0 {
		t.Errorf("unexpected diagnostics: %+v", got)
	}
}
