package passes

import (
	"fmt"
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

// A feature reference is typed by the feature's declaration, not by the value
// it happens to hold: `w` is a Real here, whatever the 3 it was given is.
func TestExprBindFeatureReferenceRespectsDeclaredType(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute w : ScalarValues::Real = 3;
		attribute x : ScalarValues::String = w;
	}`, "cannot bind Real value to a feature typed by String")
}

// A type only bounds the values an expression may yield, so a feature typed
// narrower than the expression is not refused: the Real `w` may hold a whole
// number, and whether it does is known when the value is bound.
func TestExprBindWiderTypedReferenceToNarrowerFeatureOK(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute w : ScalarValues::Real = 1.5;
		attribute x : ScalarValues::Integer = w;
	}`)
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
		`package P { calc def c { 1 + "s" } }`,
		"operator '+' is not defined for Natural and String")
}

func TestExprStringConcatenationOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { "a" + "b" } }`)
}

func TestExprIntegerRealMixOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { 1 + 2.5 } }`)
}

func TestExprDivisionOfStringsRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { "a" / "b" } }`,
		"operator '/' requires numeric operands")
}

// The reference implementation evaluates a whole-number quotient — Natural or
// Integer operands alike — as a Rational, so it binds to a Rational feature.
// Integer ** Natural stays Integer.
func TestExprWholeNumberDivisionAndPowerOK(t *testing.T) {
	wantNoDiags(t, `package P {
	attribute q : ScalarValues::Rational = 7 / 2;
	attribute p : ScalarValues::Integer = 3 ** 2;
}`)
}

// A quotient is typed Rational, but a Rational may be whole, so a whole-number
// feature does not refuse it: which values the quotient takes is known when it
// is evaluated, not from its type. A quotient of decimals is no different.
func TestExprQuotientMayBindToWholeNumberFeature(t *testing.T) {
	wantNoDiags(t, `package P {
	attribute i : ScalarValues::Integer = -7;
	attribute q : ScalarValues::Natural = 7 / 2;
	attribute r : ScalarValues::Natural = i / 2;
	attribute s : ScalarValues::Integer = 1.5 / 2;
	calc def IntDiv { return : ScalarValues::Integer = 7 / 2; }
}`)
}

// The quotient's type is still Rational, whatever the operands: it is observable
// where a Boolean is required, and where a disjoint type is.
func TestExprDivisionIsRational(t *testing.T) {
	wantOneDiag(t,
		`package P { constraint def c { 7 / 2 } }`,
		"constraint expression must be Boolean, found Rational")
	wantOneDiag(t,
		`package P {
	attribute i : ScalarValues::Integer = -7;
	constraint def c { i / 2 }
}`,
		"constraint expression must be Boolean, found Rational")
	wantOneDiag(t,
		`package P { constraint def c { 1.5 / 2 } }`,
		"constraint expression must be Boolean, found Rational")
	wantOneDiag(t,
		`package P { attribute s : ScalarValues::String = 7 / 2; }`,
		"cannot bind Rational value to a feature typed by String")
}

// A literal's type is exact, so a decimal that reads as no whole number is
// refused by a whole-number feature.
func TestExprBindDecimalLiteralToNaturalRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute n : ScalarValues::Natural = 2.5; }`,
		"cannot bind Rational value to a feature typed by Natural")
}

func TestExprNotOnIntegerRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { not 3 } }`,
		"operator 'not' requires a Boolean operand")
}

func TestExprAndOnIntegerRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { 1 and true } }`,
		"requires Boolean operands")
}

func TestExprComparisonOfBooleanRejected(t *testing.T) {
	wantOneDiag(t,
		`package P { calc def c { true < false } }`,
		"operator '<' is not defined for Boolean and Boolean")
}

func TestExprEqualityAcrossDisjointTypesWarns(t *testing.T) {
	diags := exprDiags(t, `package P { calc def c { 1 == "a" } }`)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityWarning {
		t.Fatalf("expected a warning, got %v", diags[0].Severity)
	}
}

func TestExprInequalityAcrossDisjointTypesWarnsAlwaysTrue(t *testing.T) {
	diags := exprDiags(t, `package P { calc def c { 1 != "a" } }`)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "always true") {
		t.Fatalf("expected an always-true warning, got %q", diags[0].Message)
	}
}

func TestExprNumericEqualityOK(t *testing.T) {
	wantNoDiags(t, `package P { calc def c { 1 == 2.0 } }`)
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
				transition first a if temp then b;
			}
		}
	}`, "transition guard must be Boolean, found Integer")
}

// A change-event condition is a bare expression in the parsed tree, so it must
// be checked there rather than in the lowered ast.ChangeEvent. A bare name is
// left alone: lowering reads it as a signal trigger, not a condition.
func TestExprChangeEventConditionMustBeBoolean(t *testing.T) {
	wantOneDiag(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a when temp + 1 then b;
			}
		}
	}`, "change event condition must be Boolean, found Integer")
}

func TestExprChangeEventConditionOK(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a when temp > 5 then b;
			}
		}
	}`)
}

func TestExprTimedTransitionDelayIsNotACondition(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute period : ScalarValues::Integer = 10;
			state def S {
				state a {
					accept after 10 then b;
				}
				state b {
					accept at period then a;
				}
			}
		}
	}`)
}

func TestExprAcceptWhenConditionMustBeBoolean(t *testing.T) {
	wantOneDiag(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a {
					accept when temp then b;
				}
				state b;
			}
		}
	}`, "change event condition must be Boolean, found Integer")
}

func TestExprTransitionGuardComparisonOK(t *testing.T) {
	wantNoDiags(t, `package P {
		part def M {
			attribute temp : ScalarValues::Integer = 3;
			state def S {
				state a;
				state b;
				transition first a if temp > 5 then b;
			}
		}
	}`)
}

const calcAdd = `calc def add {
	in a : ScalarValues::Integer;
	in b : ScalarValues::Integer;
	a + b
}
`

func TestExprInvocationTooFewArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1) } }`,
		"add requires 2 argument(s), found 1")
}

func TestExprInvocationTooManyArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1, 2, 3) } }`,
		"add takes 2 argument(s), found 3")
}

// An argument expression is typed once, so an error inside it is reported once.
func TestExprInvocationArgumentErrorReportedOnce(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1 + "s", 2) } }`,
		`operator '+' is not defined for Natural and String`)
}

func TestExprInvocationCorrectArityOK(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { add(1, 2) } }`)
}

func TestExprInvocationThroughAliasChecksArguments(t *testing.T) {
	const model = `package P {
		` + calcAdd + `
		alias addAlias for add;
		calc c { addAlias(%s) }
	}`
	wantOneDiag(t, fmt.Sprintf(model, `1`), "add requires 2 argument(s), found 1")
	wantOneDiag(t, fmt.Sprintf(model, `1, "two"`),
		"argument 2 of add expects Integer, found String")
	wantNoDiags(t, fmt.Sprintf(model, `1, 2`))
}

func TestExprInvocationArgumentTypeMismatch(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1, "two") } }`,
		`argument 2 of add expects Integer, found String`)
}

// An argument binds to its parameter as a value binds to a feature: a decimal
// literal is no Integer, a quotient or a Real feature's value may be one.
func TestExprInvocationArgumentNarrowerThanExpression(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(1, 2.5) } }`,
		`argument 2 of add expects Integer, found Rational`)
	wantNoDiags(t, `package P {
		`+calcAdd+`
		attribute w : ScalarValues::Real = 1.5;
		calc c { add(7 / 2, w) }
	}`)
}

func TestExprInvocationDefaultedParameterOptional(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def scale {
			in a : ScalarValues::Integer;
			in factor : ScalarValues::Integer = 2;
			a * factor
		}
		calc c { scale(3) }
	}`)
}

func TestExprInvocationUnknownNamedArgument(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { add(a = 1, c = 2) } }`,
		`add has no parameter named "c"`)
}

// A receiver binds by position, so a call whose arguments bind by name states
// no parameter for it (runtime/eval.go reports the same call).
func TestExprInvocationReceiverWithNamedArguments(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { 1->add(a = 1, b = 2) } }`,
		"add cannot be called with a receiver and named arguments")
}

func TestExprInvocationNamedArgumentsOK(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { add(a = 1, b = 2) } }`)
}

func TestExprLiteralConformsToNatural(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute n : ScalarValues::Natural = 3;
		attribute r : ScalarValues::Rational = 1.5;
	}`)
}

// A signed literal spells its value out, so `-3` is refused by a Natural feature
// as a decimal literal is; a negated Integer feature may be a Natural.
func TestExprNegatedLiteralIsNotNatural(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute n : ScalarValues::Natural = -3; }`,
		"cannot bind Integer value to a feature typed by Natural")
	wantNoDiags(t, `package P {
		attribute i : ScalarValues::Integer = -3;
		attribute n : ScalarValues::Natural = -i;
	}`)
}

func TestExprArrowFormReceiverCountsAsFirstArgument(t *testing.T) {
	wantNoDiags(t, `package P { `+calcAdd+` calc c { 1->add(2) } }`)
}

func TestExprArrowFormArityStillChecked(t *testing.T) {
	wantOneDiag(t,
		`package P { `+calcAdd+` calc c { 1->add(2, 3) } }`,
		"add takes 2 argument(s), found 3")
}

func TestExprInheritedParametersCounted(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc def Add2 :> add;
		calc c { Add2(1, 2) }
	}`)
}

// A specialization may redefine a subset of the inherited parameters; the
// signature is still the full inherited one.
func TestExprPartiallyRedefinedParametersKeepInheritedSignature(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc def AddPositive :> add {
			in a :>> a : ScalarValues::Real;
		}
		calc c { AddPositive(1, 2) }
	}`)
}

// Parameters inherited from several supertypes keep the order the supertypes
// were declared in, so `first` precedes `second` here.
func TestExprMultipleSupertypesKeepDeclarationOrder(t *testing.T) {
	const model = `package P {
		calc def First { in first : ScalarValues::String; }
		calc def Second { in second : ScalarValues::Integer; }
		calc def Both :> First, Second;
		calc c { Both(%s) }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `"s", 1`))
	// Were the supertypes folded in reverse, `second` would come first and the
	// String would be reported against it as argument 1 instead.
	wantOneDiag(t, fmt.Sprintf(model, `1, "s"`),
		"argument 2 of Both expects Integer, found String")
}

// A redefined parameter's own type is what its argument is checked against.
func TestExprRedefinedParameterTypeChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		`+calcAdd+`
		calc def AddText :> add {
			in a :>> a : ScalarValues::String;
		}
		calc c { AddText(1, 2) }
	}`, "argument 1 of AddText expects String, found Natural")
}

// A specialization may refine an inherited parameter under a new name, which
// redefines it by position instead of extending the signature.
func TestExprRenamedParameterRedefinesByPosition(t *testing.T) {
	const model = `package P {
		` + calcAdd + `
		calc def Convert :> add {
			in x : ScalarValues::String;
		}
		calc c { Convert(%s) }
	}`
	// `x` redefines `a`, so the signature stays (x, b), not (a, b, x).
	wantOneDiag(t, fmt.Sprintf(model, `1, 2`),
		"argument 1 of Convert expects String, found Natural")
	wantNoDiags(t, fmt.Sprintf(model, `"s", 2`))
}

// A redeclaration matches the inherited parameter at its own position, even
// when its name is that of a different inherited parameter — the same rule
// semantics/redefinition.go applies, so both tiers see one signature.
func TestExprRedeclaredParametersMatchByPositionNotName(t *testing.T) {
	const model = `package P {
		calc def Swap { in a : ScalarValues::String; in b : ScalarValues::Integer; }
		calc def Swapped :> Swap { in b : ScalarValues::String; in a : ScalarValues::Integer; }
		calc c { Swapped(%s) }
	}`
	wantNoDiags(t, fmt.Sprintf(model, `"s", 1`))
	// Matched by name instead, the signature would be (a : Integer, b : String)
	// and this would report against argument 2.
	wantOneDiag(t, fmt.Sprintf(model, `1, 1`),
		"argument 1 of Swapped expects String, found Natural")
}

// An `out` parameter occupies a position, so an input declared after one
// redefines the inherited parameter at that position, not the first. The input
// at the position the output took keeps its place in the list, since an output
// does not redefine an input — the list semantics.Model.parametersOf derives.
func TestExprOutParameterOccupiesAPosition(t *testing.T) {
	const model = `package P {
		calc def C { in a : ScalarValues::String; in b : ScalarValues::Integer; }
		calc def D :> C { out y; in x : ScalarValues::Integer; }
		calc c { D(%s) }
	}`
	// D's parameters are (y, x, a): `x` is at position 1, so it redefines `b`,
	// and `a` is inherited because `out y` does not redefine it.
	wantNoDiags(t, fmt.Sprintf(model, `1, "s"`))
	wantOneDiag(t, fmt.Sprintf(model, `"s", "s"`),
		"argument 1 of D expects Integer, found String")

	// Adding only an output leaves the inherited signature untouched.
	wantNoDiags(t, `package P {
		calc def C { in a : ScalarValues::Integer; in b : ScalarValues::Integer; }
		calc def D :> C { out y; }
		calc c { D(1, 2) }
	}`)
}

// An explicit `:>>` naming a parameter at another position claims that one, so
// the parameter left to inherit is the one no declaration redefines — again the
// list semantics.Model.parametersOf derives.
func TestExprExplicitRedefinitionClaimsItsTarget(t *testing.T) {
	const model = `package P {
		calc def C { in a : ScalarValues::String; in b : ScalarValues::Integer; }
		calc def D :> C { in bb :>> b; }
		calc c { D(%s) }
	}`
	// D's parameters are (bb, a), not (bb, b).
	wantNoDiags(t, fmt.Sprintf(model, `1, "s"`))
	wantOneDiag(t, fmt.Sprintf(model, `1, 1`),
		"argument 2 of D expects String, found Natural")
}

// A declaration claims the inherited parameter at its own position, not the
// next unclaimed one, so an explicit `:>>` further along does not shift the
// declarations after it.
func TestExprPositionalClaimIsByDeclarationIndex(t *testing.T) {
	const model = `package P {
		calc def C { in a : ScalarValues::String; in b : ScalarValues::Integer; in c : ScalarValues::Boolean; }
		calc def D :> C { in z :>> c; in w; }
		calc c { D(%s) }
	}`
	// D's parameters are (z, w, a): `z` claims `c`, `w` claims `b` by position.
	wantOneDiag(t, fmt.Sprintf(model, `true, 1, "s", 1`),
		"D takes 3 argument(s), found 4")
	wantNoDiags(t, fmt.Sprintf(model, `true, 1, "s"`))
}

func TestExprTypedCalcUsageInheritsParameters(t *testing.T) {
	wantNoDiags(t, `package P {
		`+calcAdd+`
		calc myAdd : add;
		calc c { myAdd(1, 2) }
	}`)
}

func TestExprParameterlessCalcRejectsArguments(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def zero { 0 }
		calc c { zero(1) }
	}`, "zero takes 0 argument(s), found 1")
}

func TestExprUnresolvedNameProducesNoTypeDiagnostic(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Integer = missing; }`)
}

// A type with no scalar ancestor is outside the lattice, so the scalar rules
// say nothing about it — but no literal is an instance of it either, which the
// value rules report (see typecheck_value_test.go).
func TestExprLiteralBoundToNonScalarType(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute def Mass;
		part def Car { attribute m : Mass = 5; }
	}`, "cannot bind Natural value to a feature typed by Mass")
}

// Reaching a scalar through a user-declared type keeps the lattice rules, and
// their more precise message, in force.
func TestExprScalarSpecializationUsesLattice(t *testing.T) {
	wantOneDiag(t, `package P {
		attribute def Mass specializes ScalarValues::Integer;
		part def Car { attribute m : Mass = 5.5; }
	}`, "cannot bind Rational value to a feature typed by Integer")
}
