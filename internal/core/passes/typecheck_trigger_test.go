package passes

import (
	"strings"
	"testing"
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
		attribute wait = 5 [s];
		attribute waitTwice = d + d;
		attribute alsoReady = ready;
		attribute loop = loop;
		in attribute given = true;
		part h : Holder;
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
		{"after (if flag ? 5 else 6)", "Natural"},
		{"after (if flag ? 5 else 5.5)", "Natural or Rational"},
		{"after (if flag ? x else len)", "Integer or LengthValue"},
		{"after x ?? 5", "Integer or Natural"},
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
		"after d + 5 [s]",
		"after 2 * d",
		"after -d",
		"after t2 - t",
		"after Twice(d)",
		"after 10 [m] / 2 [m/s]",
		"after (if flag ? 5 [s] else 6 [s])",
		"after (if flag ? d else d2)",
		"after d ?? 5 [s]",
	} {
		wantTriggerSilent(t, "transition first a accept "+trigger+" then b;")
	}
}

// Arithmetic over durations and time instants is judged by its dimension, as
// the pilot's isDuration/isTime admit it, so a sum of instants or of an instant
// and a duration passes either trigger; a conditional whose branches disagree
// is left to evaluation.
func TestTriggerTimeArithmeticIsJudgedByDimension(t *testing.T) {
	for _, trigger := range []string{
		"after d + d",
		"after t + d",
		"at d + d",
		"at t - t",
		"after (if flag ? d else 5)",
		"at (if flag ? t else 5)",
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
		{"at x", "Integer"},
		{"at flag", "Boolean"},
		{"at Twice(d)", "DurationValue"},
		{"at d * d", "a value of dimension T^2"},
		{"at x % 2", "NumericalValue"},
		{"at (if flag ? 5 else 6)", "Natural"},
		{"at (if flag ? d else 5 [s])", "DurationValue or a quantity in second (a DurationUnit)"},
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
		"at Later(t)",
		"at TimeOf(a)",
		"at t + d",
		"at t2 - d",
		"at (if flag ? t else t2)",
		"at (if flag ? t else h.instant)",
		"at t ?? TimeOf(a)",
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
		{"when untyped", "an untyped feature"},
		{"when lazy", "an untyped feature"},
		{"when given", "an untyped feature"},
		{"when count", "Natural"},
		{"when wait", "ScalarQuantityValue"},
		{"when x + 1", "Integer"},
		{"when 5 [s]", "a quantity in second (a DurationUnit)"},
		{"when d * 2", "a value of dimension T"},
		{"when Twice(d)", "DurationValue"},
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
