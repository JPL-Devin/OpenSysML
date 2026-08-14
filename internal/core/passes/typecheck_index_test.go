package passes

import "testing"

// An index is a position, so a value that is not a whole number is reported
// where it is written rather than only when the expression is evaluated.
func TestIndexNonIntegerIndexReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#(1.5); }`,
		"sequence index must be an Integer")
}

func TestIndexStringIndexReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#("a"); }`,
		"sequence index must be an Integer")
}

// The library declares `in index: Positive[1]`, so 0 is not a position.
func TestIndexZeroReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#(0); }`,
		"sequence index counts from 1")
}

// The length is known only where the sequence itself is written out; there the
// index is checked against it.
func TestIndexPastWrittenSequenceReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#(4); }`,
		"sequence index 4 is outside 1..3")
}

func TestIndexInRangeOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute x = (1, 2, 3)#(3); }`)
}

// A model counting positions holds them in an `Integer`, which the library's
// `Positive` parameter accepts a value of: whether that value is a position the
// operand has is known from the value, so it is checked at evaluation and not
// reported here.
func TestIndexTypedIntegerNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute xs = (1, 2, 3);
		attribute i : ScalarValues::Integer = 2;
		attribute x = xs#(i);
	}`)
}

// An element of the indexed sequence is typed once, so a mistake inside it is
// reported once rather than for each pass over the elements.
func TestIndexReportsAnErrorInAnElementOnce(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1 + true, 2)#(1); }`,
		"operator '+' is not defined for Natural and Boolean")
}

// Indexing per iteration is the idiomatic spelling, so the loop variable of a
// `for` over a sequence of numbers is an index and is not reported.
func TestIndexByLoopVariableNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		private import ScalarValues::*;
		calc total {
			attribute xs : Integer[*] = (1, 2, 3);
			attribute sum : Integer = 0;
			for i in (1, 2, 3) {
				sum = sum + xs#(i);
			}
			return : Integer = sum;
		}
	}`)
}

// An index of a value whose length the checker does not know is not reported:
// the runtime checks the position it turns out to be.
func TestIndexOfReferenceNotReported(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs#(4); }`)
}

// The element type of a written sequence is the type of the indexing, so
// binding it to a feature of another type is reported.
func TestIndexElementTypeIsCheckedAgainstTheBinding(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x : ScalarValues::Boolean = (1, 2, 3)#(1); }`,
		"cannot bind Natural value to a feature typed by Boolean")
}

// A sequence of no one scalar type has no element type, so indexing it is
// checked no further here; the runtime answers the element it turns out to be.
func TestIndexOfMixedSequenceHasNoElementType(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Boolean = (1, "a")#(1); }`)
}

// An element whose type is not known leaves the element type of the sequence
// unknowable, so nothing is reported about what an index of it holds: `flag` is
// a Boolean here, and typing the sequence from the elements that happen to be
// known would reject the model for it.
func TestIndexOfSequenceWithAnUntypedElementIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute flag = true;
		attribute y : ScalarValues::Boolean = (1, flag)#(2);
	}`)
}

// The bracket form is a quantity, not an index, so the unit name in it is not
// checked as a position.
func TestQuantityBracketFormIsNotIndexed(t *testing.T) {
	wantNoDiags(t, `package P { attribute def m; attribute x = 5 [m]; }`)
}

// A selector states a condition, so a body whose result is of a known type that
// is not Boolean is reported where it is written.
func TestSelectNonBooleanBodyReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute xs = (1, 2, 3); attribute x = xs.?{in e; 1}; }`,
		"select predicate must be Boolean")
}

func TestSelectBooleanBodyOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs.?{in e; e > 1}; }`)
}

// A body parameter's type is the element type of whatever the operand turns out
// to hold, which the checker does not track, so an expression over the parameter
// has no static type and is left to the runtime rather than guessed at.
func TestBodyParameterHasNoStaticType(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs.?{in e; e + 1}; }`)
}

func TestCollectBodyOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs.{in e; e * 2}; }`)
}

// A body parameter is visible in the body and nowhere else.
func TestBodyParameterIsNotVisibleOutsideTheBody(t *testing.T) {
	diags := exprDiags(t,
		`package P { attribute xs = (1, 2, 3); attribute x = xs.{in e; e * 2}; attribute y = e; }`)
	if len(diags) != 0 {
		t.Fatalf("expected the type tier to leave the outside reference to the name tier, got %v", diags)
	}
}
