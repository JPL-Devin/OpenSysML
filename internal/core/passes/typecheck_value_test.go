package passes

import "testing"

// enumPrelude declares types outside the scalar lattice: an enumeration, a
// structural hierarchy, and an unrelated definition.
const valuePrelude = `package M {
	enum def Color { red; green; }
	enum def Size { small; large; }
	part def Vehicle;
	part def Truck specializes Vehicle;
	part def Boat;
}
`

func valueDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	return typeDiags(t, scalarPrelude+valuePrelude+src)
}

func wantOneValueDiag(t *testing.T, src, want string) {
	t.Helper()
	diags := valueDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if got := diags[0].Message; got != want {
		t.Fatalf("expected message %q, got %q", want, got)
	}
}

func wantNoValueDiags(t *testing.T, src string) {
	t.Helper()
	if diags := valueDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestValueEnumerationLiteralOfSameEnumOK(t *testing.T) {
	wantNoValueDiags(t, `package P { attribute c : M::Color = M::Color::red; }`)
}

func TestValueEnumerationLiteralOfOtherEnum(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute c : M::Color = M::Size::small; }`,
		"cannot bind a value of type Size to a feature typed by Color")
}

func TestValueScalarLiteralToEnumeration(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute c : M::Color = 5; }`,
		"cannot bind Natural value to a feature typed by Color")
}

func TestValueSubtypeInstanceConforms(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part t : M::Truck;
		part v : M::Vehicle = t;
	}`)
}

func TestValueUnrelatedInstanceDoesNot(t *testing.T) {
	wantOneValueDiag(t, `package P {
		part b : M::Boat;
		part v : M::Vehicle = b;
	}`, "cannot bind a value of type Boat to a feature typed by Vehicle")
}

// A supertype instance is not an instance of the subtype, so the direction of
// conformance matters.
func TestValueSupertypeInstanceDoesNot(t *testing.T) {
	wantOneValueDiag(t, `package P {
		part v : M::Vehicle;
		part t : M::Truck = v;
	}`, "cannot bind a value of type Vehicle to a feature typed by Truck")
}

func TestValueTooManyValuesForUpperBound(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[2] = (1, 2, 3); }`,
		"3 value(s) bound to a feature with multiplicity upper bound 2")
}

func TestValueTooFewValuesForLowerBound(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[2] = 1; }`,
		"1 value(s) bound to a feature with multiplicity lower bound 2")
}

// An empty collection is a count of zero, which a nonzero lower bound rejects.
func TestValueEmptyCollectionForLowerBound(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[1..3] = (); }`,
		"0 value(s) bound to a feature with multiplicity lower bound 1")
}

// A redefining feature that states no multiplicity of its own is bound by the
// one it inherits (KerML 1.0 §7.3.4.5), so a default it adds is checked there.
func TestValueCountAgainstRedefinedMultiplicity(t *testing.T) {
	wantOneValueDiag(t, `package P {
		part def Base { attribute xs : ScalarValues::Integer[3]; }
		part def Derived :> Base { attribute :>> xs = (1, 2); }
	}`, "2 value(s) bound to a feature with multiplicity lower bound 3")
}

func TestValueSingleValueForRangeOK(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute opt : ScalarValues::Integer[0..1] = 7;
		attribute many : ScalarValues::Integer[1..*] = (1, 2, 3);
		attribute exact : ScalarValues::Integer[3] = (1, 2, 3);
	}`)
}

// A reference may itself be multi-valued, so its count is not known statically
// and the multiplicity check must stay silent rather than assume one value.
func TestValueReferenceCountUnknown(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute xs : ScalarValues::Integer[2] = (1, 2);
		attribute ys : ScalarValues::Integer[2] = xs;
	}`)
}

// Every element of a collection is checked against the feature's type, not just
// the first.
func TestValueCollectionElementTypes(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute names : ScalarValues::String[*] = ("a", 2); }`,
		"cannot bind Natural value to a feature typed by String")
}

func TestValueCollectionOfEnumerationLiterals(t *testing.T) {
	wantOneValueDiag(t, `package P {
		attribute cs : M::Color[*] = (M::Color::red, M::Size::large);
	}`, "cannot bind a value of type Size to a feature typed by Color")
}

// An unresolved or untyped feature must not produce a diagnostic: the checker
// only reports when both sides are known.
func TestValueUntypedFeatureNotReported(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute unknownType : Missing = 5;
		part p = 5;
	}`)
}

// A value that is an expression over the type, rather than a literal or a name,
// is not judged: the checker cannot type it.
func TestValueExpressionNotReported(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part def Wheel;
		part w : Wheel = Wheel();
	}`)
}

// A referenced value's type name must resolve where the value was declared: the
// scope the declaration owns also exposes its own members, which would shadow
// the type it names and make a well-formed model report a mismatch.
func TestValueTypeNameNotShadowedByOwnMembers(t *testing.T) {
	wantNoValueDiags(t, `package P {
		private import M::*;
		part t : Truck {
			attribute Truck : ScalarValues::Integer = 1;
		}
		part v : Vehicle = t;
	}`)
}
