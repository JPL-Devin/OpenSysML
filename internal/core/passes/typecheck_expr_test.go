package passes

import (
	"strings"
	"testing"
)

// scalarPrelude declares the stdlib scalar types the expression checker keys
// off, so these tests do not need the real library loaded.
const scalarPrelude = `package ScalarValues {
	attribute def ScalarValue;
	attribute def Boolean specializes ScalarValue;
	attribute def String specializes ScalarValue;
	attribute def Number specializes ScalarValue;
	attribute def Complex specializes Number;
	attribute def Real specializes Complex;
	attribute def Rational specializes Real;
	attribute def Integer specializes Rational;
	attribute def Natural specializes Integer;
}
`

// exprDiags runs the default registry over the scalar prelude plus src and
// returns the type-tier diagnostics.
func exprDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	return typeDiags(t, scalarPrelude+src)
}

// wantOneDiag asserts exactly one type diagnostic whose message contains want.
func wantOneDiag(t *testing.T, src, want string) {
	t.Helper()
	diags := exprDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, want) {
		t.Fatalf("expected message containing %q, got %q", want, diags[0].Message)
	}
}

func wantNoDiags(t *testing.T, src string) {
	t.Helper()
	if diags := exprDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestExprBindStringToIntegerAttribute(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x : ScalarValues::Integer = "hello"; }`,
		"cannot bind String value to a feature typed by Integer")
}

func TestExprBindRealToIntegerAttribute(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x : ScalarValues::Integer = 5.5; }`,
		"cannot bind Rational value to a feature typed by Integer")
}

func TestExprBindIntegerToRealAttributeOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Real = 5; }`)
}

func TestExprBindFeatureReferenceRespectsDeclaredType(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute w : ScalarValues::Real = 1.5;
		attribute x : ScalarValues::Integer = w;
	}`, "cannot bind Real value to a feature typed by Integer")
}

func TestExprBindNestedUsageValue(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Car {
			part engine {
				attribute power : ScalarValues::Integer = "x";
			}
		}
	}`, "cannot bind String value to a feature typed by Integer")
}

func TestExprUntypedFeatureBindingSkipped(t *testing.T) {
	wantNoDiags(t, `package P { attribute x = "hello"; }`)
}

func TestExprAddIntegerAndStringRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { return 1 + "s"; } }`,
		"operator '+' is not defined for Natural and String")
}

func TestExprStringConcatenationOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { return "a" + "b"; } }`)
}

func TestExprIntegerRealMixOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { return 1 + 2.5; } }`)
}

func TestExprDivisionOfStringsRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { return "a" / "b"; } }`,
		"operator '/' requires numeric operands")
}

func TestExprNotOnIntegerRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { return not 3; } }`,
		"operator 'not' requires a Boolean operand")
}

func TestExprAndOnIntegerRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { return 1 and true; } }`,
		"requires Boolean operands")
}

func TestExprComparisonOfBooleanRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { return true < false; } }`,
		"operator '<' is not defined for Boolean and Boolean")
}

func TestExprEqualityAcrossDisjointTypesWarns(t *testing.T) {
	diags := exprDiags(t, `package P { calc def c { return 1 == "a"; } }`)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityWarning {
		t.Fatalf("expected a warning, got %v", diags[0].Severity)
	}
}

func TestExprInequalityAcrossDisjointTypesWarnsAlwaysTrue(t *testing.T) {
	diags := exprDiags(t, `package P { calc def c { return 1 != "a"; } }`)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "always true") {
		t.Fatalf("expected an always-true warning, got %q", diags[0].Message)
	}
}

func TestExprNumericEqualityOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { return 1 == 2.0; } }`)
}

func TestExprConstraintMustBeBoolean(t *testing.T) {
	wantOneDiag(t,
		`package P { constraint def c { 1 + 2 } }`,
		"constraint expression must be Boolean, found Natural")
}

func TestExprBooleanConstraintOK(t *testing.T) {
	wantNoDiags(t, `package P { constraint def c { 1 < 2 } }`)
}

func TestExprTransitionGuardMustBeBoolean(t *testing.T) {
	wantOneDiag(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition a to b if temp;
			}
		}
	}`, "transition guard must be Boolean, found Integer")
}

func TestExprTransitionGuardComparisonOK(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition a to b if temp > 5;
			}
		}
	}`)
}

const calcAdd = `calc def add {
	in a : ScalarValues::Integer;
	in b : ScalarValues::Integer;
	return a + b;
}
`

func TestExprInvocationTooFewArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { return add(1); } }`,
		"add requires 2 argument(s), found 1")
}

func TestExprInvocationTooManyArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { return add(1, 2, 3); } }`,
		"add takes 2 argument(s), found 3")
}

func TestExprInvocationCorrectArityOK(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { return add(1, 2); } }`)
}

func TestExprInvocationArgumentTypeMismatch(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { return add(1, "two"); } }`,
		`argument 2 of add expects Integer, found String`)
}

func TestExprInvocationDefaultedParameterOptional(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def scale {
			in a : ScalarValues::Integer;
			in factor : ScalarValues::Integer = 2;
			return a * factor;
		}
		calc c { return scale(3); }
	}`)
}

func TestExprInvocationUnknownNamedArgument(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { return add(a = 1, c = 2); } }`,
		`add has no parameter named "c"`)
}

func TestExprInvocationNamedArgumentsOK(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { return add(a = 1, b = 2); } }`)
}

func TestExprLiteralConformsToNatural(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute n : ScalarValues::Natural = 3;
		attribute r : ScalarValues::Rational = 1.5;
	}`)
}

func TestExprNegatedLiteralIsNotNatural(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute n : ScalarValues::Natural = -3; }`,
		"cannot bind Integer value to a feature typed by Natural")
}

func TestExprArrowFormReceiverCountsAsFirstArgument(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { return 1->add(2); } }`)
}

func TestExprArrowFormArityStillChecked(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { return 1->add(2, 3); } }`,
		"add takes 2 argument(s), found 3")
}

func TestExprInheritedParametersCounted(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc def Add2 :> add;
		calc c { return Add2(1, 2); }
	}`)
}

func TestExprTypedCalcUsageInheritsParameters(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc myAdd : add;
		calc c { return myAdd(1, 2); }
	}`)
}

func TestExprParameterlessCalcRejectsArguments(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def zero { return 0; }
		calc c { return zero(1); }
	}`, "zero takes 0 argument(s), found 1")
}

func TestExprUnresolvedNameProducesNoTypeDiagnostic(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Integer = missing; }`)
}

func TestExprNonScalarTypeSkipped(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute def Mass;
		part def Car { attribute m : Mass = 5; }
	}`)
}
