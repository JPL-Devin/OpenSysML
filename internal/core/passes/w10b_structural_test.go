package passes

import "testing"

// A second entry/do/exit action, a second return parameter and a composite
// usage owned by a port each draw the reference's diagnostic, on the extra
// member rather than on the declaration.
func TestW10BStructuralReportsTheExtraMember(t *testing.T) {
	const src = `package P {
		action def Act;
		part def Fuel;
		state def S {
			entry action e1 : Act;
			entry action e2 : Act;
			do action d1 : Act;
			do action d2 : Act;
			exit action x1 : Act;
			exit action x2 : Act;
		}
		calc def C {
			return a;
			return b;
		}
		port def BadPort {
			part fuel : Fuel;
			ref part other : Fuel;
		}
		part holder {
			port bad : BadPort {
				part nested : Fuel;
			}
		}
	}`
	byMessage := map[string]int{}
	for _, d := range typeDiags(t, src) {
		switch d.Message {
		case msgOnlyOneEntryAction, msgOnlyOneDoAction, msgOnlyOneExitAction,
			msgOnlyOneReturn, msgPortDefComposite, msgPortUsageComposite:
			byMessage[d.Message]++
			if d.Severity != SeverityError {
				t.Errorf("%q severity = %v, want an error", d.Message, d.Severity)
			}
		}
	}
	for msg, want := range map[string]int{
		msgOnlyOneEntryAction: 1,
		msgOnlyOneDoAction:    1,
		msgOnlyOneExitAction:  1,
		msgOnlyOneReturn:      1,
		msgPortDefComposite:   1,
		msgPortUsageComposite: 1,
	} {
		if byMessage[msg] != want {
			t.Errorf("got %d %q diagnostics, want %d", byMessage[msg], msg, want)
		}
	}
}

// One of each subaction, one return, and referential port members — including
// the reference-subsetting form the training corpus uses — stay silent.
func TestW10BStructuralClean(t *testing.T) {
	const src = `package P {
		action def Act;
		part def Fuel;
		attribute def Temp;
		state def S {
			entry action e1 : Act;
			do action d1 : Act;
			exit action x1 : Act;
		}
		calc def C {
			return a;
		}
		port def GoodPort {
			attribute t : Temp;
			ref part fuel : Fuel;
			in item i : Fuel;
		}
		part holder {
			part fuel : Fuel;
			port good : GoodPort {
				ref part f2 : Fuel;
				port sub : GoodPort;
			}
		}
	}`
	for _, d := range typeDiags(t, src) {
		switch d.Message {
		case msgOnlyOneEntryAction, msgOnlyOneDoAction, msgOnlyOneExitAction,
			msgOnlyOneReturn, msgPortDefComposite, msgPortUsageComposite:
			t.Errorf("unexpected %q at offset %d", d.Message, d.Span.Offset)
		}
	}
}

// Malformed input must not panic the pass.
func TestW10BStructuralMalformed(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { entry ; entry ; } }",
		"package P { port def X { part",
		"package P { calc def C { return ; return ; } ",
	} {
		typeDiags(t, src)
	}
}
