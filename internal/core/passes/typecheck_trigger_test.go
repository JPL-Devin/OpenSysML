package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// triggerFixture declares the durations, time instants, quantities and
// calculations the trigger cases below name; %s is the state def body.
const triggerFixture = `package P {
	private import ISQ::*;
	private import SI::*;
	private import SIPrefixes::milli;
	private import MeasurementReferences::*;
	private import Time::*;
	private import ScalarValues::*;

	attribute <ms> millisecond : DurationUnit {
		:>> unitConversion : ConversionByPrefix { :>> prefix = milli; :>> referenceUnit = s; }
	}
	attribute def Delay :> DurationValue;

	part def Holder {
		attribute delay : Delay;
		attribute instant : Iso8601DateTime;
		attribute ok : Boolean;
	}
	calc def Twice { in d : DurationValue; return : DurationValue = d + d; }
	calc def Later { in t : TimeInstantValue; return : TimeInstantValue = t; }
	calc def Len { return : LengthValue; }
	calc def IsOk { return : Boolean; }

	state def Base {
		attribute d : DurationValue;
		attribute t : TimeInstantValue;
	}
	state def S :> Base {
		attribute :>> d;
		attribute :>> t;
		attribute d2 : DurationValue :> d;
		attribute t2 : TimeInstantValue :> t;
		attribute x : Integer;
		attribute len : LengthValue;
		attribute flag : Boolean;
		attribute untyped;
		attribute ready = false;
		attribute lazy default false;
		attribute count = 3;
		attribute total = count + count;
		attribute label : String;
		attribute wait = 5 [s];
		attribute waitTwice = d + d;
		attribute alsoReady = ready;
		attribute loop = loop;
		in attribute given = true;
		part h : Holder;
		attribute viaDelay :> h.delay;
		attribute viaInstant :> h.instant;
		attribute viaOk :> h.ok;
		state a;
		state b;
		%s
	}
}`

func triggerDiags(t *testing.T, body string) []Diagnostic {
	t.Helper()
	return libraryTypeDiags(t, strings.Replace(triggerFixture, "%s", body, 1))
}

// wantTriggerDiag asserts exactly one type error, under code, whose message
// contains want.
func wantTriggerDiag(t *testing.T, body, code, want string) {
	t.Helper()
	diags := triggerDiags(t, body)
	if len(diags) != 1 {
		t.Fatalf("want one type diagnostic for %q, got %v", body, diags)
	}
	d := diags[0]
	if d.Code != code || d.Severity != SeverityError {
		t.Errorf("%q: got code %q severity %v, want %q error", body, d.Code, d.Severity, code)
	}
	if !strings.Contains(d.Message, want) {
		t.Errorf("%q: message %q does not contain %q", body, d.Message, want)
	}
}

func wantTriggerSilent(t *testing.T, body string) {
	t.Helper()
	if diags := triggerDiags(t, body); len(diags) != 0 {
		t.Fatalf("want no type diagnostics for %q, got %v", body, diags)
	}
}

// The argument of `after` must be a DurationValue (SysML v2 §8.3.17,
// TriggerInvocationExpression; pilot validateTriggerInvocationActionAfterArgument).
func TestTriggerAfterRejectsNonDuration(t *testing.T) {
	for _, tc := range []struct{ trigger, found string }{
		{"after 5", "Natural"},
		{"after 5.5", "Rational"},
		{"after \"soon\"", "String"},
		{"after flag", "Boolean"},
		{"after x", "Integer"},
		{"after len", "LengthValue"},
		{"after t", "TimeInstantValue"},
		{"after h.instant", "Iso8601DateTime"},
		{"after untyped", "an untyped feature"},
		{"after count", "Natural"},
		{"after wait", "ScalarQuantityValue"},
		{"after waitTwice", "ScalarQuantityValue"},
		{"after 5 [m]", "a quantity in metre (a LengthUnit)"},
		{"after 5 [kg]", "a quantity in kilogram (a MassUnit)"},
		{"after d * d", "a value of dimension T^2"},
		{"after 5 [m] / 2 [s]", "a value of dimension L·T^-1"},
		{"after Len()", "LengthValue"},
		{"after IsOk()", "Boolean"},
		{"after 5 % 2", "NumericalValue"},
		{"after x % 2", "NumericalValue"},
		{"after 5 + 5", "NumericalValue"},
		{"after -5", "NumericalValue"},
		{"after x + 1", "NumericalValue"},
		{"after 2 * d", "NumericalValue"},
		{"after d / 2", "NumericalValue"},
		{"after total", "NumericalValue"},
		{"after label + label", "String"},
		{"after (if flag ? 5 else 6)", "the result of `if`, typed Anything"},
		{"after (if flag ? d else d2)", "the result of `if`, typed Anything"},
		{"after (if flag ? 5 [s] else 6 [s])", "the result of `if`, typed Anything"},
		{"after x ?? 5", "the result of `??`, typed Anything"},
		{"after d ?? 5 [s]", "the result of `??`, typed Anything"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		wantTriggerDiag(t, body, "trigger-after-duration",
			"an 'after' trigger's delay must be a DurationValue, found "+tc.found+": write it with a duration unit (`after 5 [s]`)")
	}
}

func TestTriggerAfterAcceptsDurations(t *testing.T) {
	for _, trigger := range []string{
		"after 5 [s]",
		"after 5 [ms]",
		"after 5.5 [min]",
		"after 2 [h]",
		"after d",
		"after d2",
		"after h.delay",
		"after viaDelay",
		"after d + 5 [s]",
		"after 2 [one] * d",
		"after -d",
		"after t2 - t",
		"after Twice(d)",
		"after 10 [m] / 2 [m/s]",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// Time arithmetic is judged by dimension, as the pilot's isDuration/isTime
// admit it: a sum of instants or of an instant and a duration passes either.
func TestTriggerTimeArithmeticIsJudgedByDimension(t *testing.T) {
	for _, trigger := range []string{
		"after d + d",
		"after t + d",
		"at d + d",
		"at t - t",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// The argument of `at` must be a TimeInstantValue (pilot
// validateTriggerInvocationActionAtArgument).
func TestTriggerAtRejectsNonTimeInstant(t *testing.T) {
	for _, tc := range []struct{ trigger, found string }{
		{"at 5", "Natural"},
		{"at 5 [s]", "a quantity in second (a DurationUnit)"},
		{"at d", "DurationValue"},
		{"at h.delay", "Delay"},
		{"at viaDelay", "Delay"},
		{"at viaOk", "Boolean"},
		{"at x", "Integer"},
		{"at flag", "Boolean"},
		{"at Twice(d)", "DurationValue"},
		{"at d * d", "a value of dimension T^2"},
		{"at x % 2", "NumericalValue"},
		{"at (if flag ? 5 else 6)", "the result of `if`, typed Anything"},
		{"at (if flag ? t else t2)", "the result of `if`, typed Anything"},
		{"at t ?? TimeOf(a)", "the result of `??`, typed Anything"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		wantTriggerDiag(t, body, "trigger-at-time-instant",
			"an 'at' trigger's time must be a TimeInstantValue, found "+tc.found+": name a feature typed by TimeInstantValue")
	}
}

func TestTriggerAtAcceptsTimeInstants(t *testing.T) {
	for _, trigger := range []string{
		"at t",
		"at t2",
		"at h.instant",
		"at viaInstant",
		"at Later(t)",
		"at TimeOf(a)",
		"at t + d",
		"at t2 - d",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// The argument of `when` must be Boolean (pilot
// validateTriggerInvocationActionWhenArgument).
func TestTriggerWhenRejectsNonBoolean(t *testing.T) {
	for _, tc := range []struct{ trigger, found string }{
		{"when x", "Integer"},
		{"when 5", "Natural"},
		{"when \"yes\"", "String"},
		{"when d", "DurationValue"},
		{"when h.delay", "Delay"},
		{"when viaDelay", "Delay"},
		{"when viaInstant", "Iso8601DateTime"},
		{"when untyped", "an untyped feature"},
		{"when lazy", "an untyped feature"},
		{"when given", "an untyped feature"},
		{"when count", "Natural"},
		{"when total", "NumericalValue"},
		{"when label + label", "String"},
		{"when wait", "ScalarQuantityValue"},
		{"when x + 1", "Integer"},
		{"when 5 [s]", "a quantity in second (a DurationUnit)"},
		{"when d * 2", "NumericalValue"},
		{"when d * d", "a value of dimension T^2"},
		{"when Twice(d)", "DurationValue"},
		{"when flag ?? ready", "the result of `??`, typed Anything"},
	} {
		body := "transition first a accept " + tc.trigger + " then b;"
		wantTriggerDiag(t, body, "trigger-when-boolean",
			"a 'when' trigger's condition must be Boolean, found "+tc.found+": compare the value (`when x > 3`)")
	}
}

func TestTriggerWhenAcceptsBooleans(t *testing.T) {
	for _, trigger := range []string{
		"when x > 3",
		"when flag",
		"when h.ok",
		"when viaOk",
		"when x > 3 and flag",
		"when not flag",
		"when x == 1 or h.ok",
		"when IsOk()",
		"when ready",
		"when alsoReady",
		"when loop",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// A conditional's result is typed Anything: a trigger judges it strictly, an
// ordinary condition leaves it to evaluation, as the pilot does.
func TestConditionalResultIsLeftToEvaluationOutsideTriggers(t *testing.T) {
	wantTriggerSilent(t, "assert constraint { flag ?? ready }")
	wantTriggerSilent(t, "entry action { if flag ?? ready { assign x := 1; } }")
}

// `transition ... when <expr>` without `accept` is a change trigger too; a bare
// name there names a signal and is not a condition.
func TestTriggerBareWhenTransition(t *testing.T) {
	wantTriggerSilent(t, "transition first a when x > 3 then b;")
	wantTriggerSilent(t, "transition first a when x then b;")
	wantTriggerDiag(t, "transition first a when x + 1 then b;", "trigger-when-boolean",
		"a 'when' trigger's condition must be Boolean, found Integer")
}

// A trigger written on an accept action — in a state body, as an action node,
// or with a named payload parameter — is checked like a transition's.
func TestTriggerOnAcceptActions(t *testing.T) {
	wantTriggerSilent(t, `state c {
		accept after 5 [s] then b;
		accept at t then b;
		accept when x > 3 then b;
	}
	do action body {
		action w1 accept after 5 [s];
		action w2 accept at t;
		action w3 accept when flag;
		action w4 accept p after Twice(d);
	}`)
	wantTriggerDiag(t, "state c { accept after 5 then b; }", "trigger-after-duration", "found Natural")
	wantTriggerDiag(t, "state c { accept at d then b; }", "trigger-at-time-instant", "found DurationValue")
	wantTriggerDiag(t, "state c { accept when x then b; }", "trigger-when-boolean", "found Integer")
	wantTriggerDiag(t, "do action body { action w accept p after 5 [m]; }", "trigger-after-duration",
		"found a quantity in metre (a LengthUnit)")
}

// The body an action target succession carries (`then starting { ... }`) is a
// behavior body like any other: its triggers and assignments see the enclosing
// scope and the declarations the body itself makes.
func TestTriggerInSuccessionBody(t *testing.T) {
	const succession = `do action body {
		action prep;
		action starting;
		first prep;
		then starting { %s }
	}`
	body := func(stmt string) string { return strings.Replace(succession, "%s", stmt, 1) }
	wantTriggerSilent(t, body("accept after 5 [s]; assign x := 1;"))
	wantTriggerDiag(t, body("accept after 5;"), "trigger-after-duration", "found Natural")
	wantTriggerDiag(t, body("accept at d;"), "trigger-at-time-instant", "found DurationValue")
	wantTriggerDiag(t, body("accept when x;"), "trigger-when-boolean", "found Integer")
	diags := triggerDiags(t, body(`assign x := "s";`))
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "cannot bind String value to a feature typed by Integer") {
		t.Fatalf("want one assignment diagnostic, got %v", diags)
	}

	const local = "attribute wait : DurationValue; attribute instant : TimeInstantValue; attribute ok : Boolean; attribute n : Integer; "
	wantTriggerSilent(t, body(local+"accept after wait; accept at instant; accept when ok; accept when n > 3; assign n := 1;"))
	wantTriggerDiag(t, body(local+"accept after n;"), "trigger-after-duration", "found Integer")
	wantTriggerDiag(t, body(local+"accept at wait;"), "trigger-at-time-instant", "found DurationValue")
	wantTriggerDiag(t, body(local+"accept when n;"), "trigger-when-boolean", "found Integer")
	diags = triggerDiags(t, body(local+`assign n := "s";`))
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "cannot bind String value to a feature typed by Integer") {
		t.Fatalf("want one assignment diagnostic, got %v", diags)
	}
}

// A name a succession body declares resolves inside that body and nowhere
// else: the body is a namespace of its own, not part of the enclosing action.
func TestSuccessionBodyDeclarationScope(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def A {
			action prep;
			action starting;
			first prep;
			then starting { attribute n : Integer; attribute inner : Integer = n; }
			attribute outer : Integer = n;
		}
	}`
	idx := newTestIndex()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	diags := Analyze("<t>", root, nil, idx)
	if len(diags) != 1 || diags[0].Code != "unresolved" || diags[0].Span.Offset != strings.LastIndex(src, "n;") {
		t.Fatalf("want one unresolved reference, the outer `= n`, got %v", diags)
	}
}

// An argument the type tier cannot type — an unresolved name, which the name
// tier reports — is not reported again here.
func TestTriggerUnresolvedArgumentIsSilent(t *testing.T) {
	for _, trigger := range []string{"after missing", "at missing", "when missing", "after 5 [missing]"} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// A requirement or constraint named as a Boolean stands for its result, whose
// type reaches back through the library's nameless `return : Boolean`; that
// holds on every load path, the bundled snapshot included.
func TestRequireNamesRequirementResult(t *testing.T) {
	diags := libraryTypeDiags(t, `package P {
		requirement def R;
		requirement r1 : R;
		requirement r2;
		constraint c { true }
		requirement group {
			require r1;
			require r2;
			require c;
		}
	}`)
	if len(diags) != 0 {
		t.Fatalf("want no type diagnostics, got %v", diags)
	}
	diags = libraryTypeDiags(t, `package P {
		attribute n : ScalarValues::Integer;
		requirement group { require n; }
	}`)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "require expression must be Boolean, found Integer") {
		t.Fatalf("want one Integer diagnostic, got %v", diags)
	}
}

// An untyped feature holds nothing statically: a trigger's `when` requires a
// Boolean and reports it, while an `if`, `while` or `require` condition is
// left to evaluation, as before.
func TestUntypedConditionIsLeftToEvaluationOutsideTriggers(t *testing.T) {
	diags := libraryTypeDiags(t, `package P {
		calc def C { in x; if x { return : ScalarValues::Real = 1.0; } return : ScalarValues::Real = 0.0; }
		attribute u;
		action def A { while u { } }
		requirement group { require u; }
	}`)
	if len(diags) != 0 {
		t.Fatalf("want no type diagnostics, got %v", diags)
	}
	wantTriggerDiag(t, "transition first a accept when untyped then b;", "trigger-when-boolean", "found an untyped feature")
}
