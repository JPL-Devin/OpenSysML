package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// libraryDiags analyzes src against the bundled standard library and returns
// the name-resolution and type diagnostics, which is what a call reports.
func libraryDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	var out []Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == "type" || d.Source == "name-resolution" {
			out = append(out, d)
		}
	}
	return out
}

func wantLibraryClean(t *testing.T, src string) {
	t.Helper()
	if diags := libraryDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func wantLibraryDiag(t *testing.T, src, code, want string) {
	t.Helper()
	diags := libraryDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one diagnostic, got %v", diags)
	}
	if diags[0].Code != code {
		t.Fatalf("expected code %q, got %q (%s)", code, diags[0].Code, diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, want) {
		t.Fatalf("expected message containing %q, got %q", want, diags[0].Message)
	}
}

const numericImports = `
	private import ScalarValues::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import RationalFunctions::*;
	private import ComplexFunctions::*;
`

// A local name several imported function packages declare binds to the declaration
// whose parameter types the arguments conform to, so every acceptance probe checks clean.
func TestInvocationOverloadSelectsByArgumentType(t *testing.T) {
	wantLibraryClean(t, `package P {`+numericImports+`
		attribute s = ToInteger("7");
		attribute r = ToInteger(7.9);
		attribute i = abs(-2);
		attribute q = abs(-2.5);
		attribute c = abs(rect(3.0, 4.0));
		attribute z = isZero(rect(0.0, 0.0));
		attribute m = max(3, 2.5);
	}`)
}

// Only imported packages contribute candidates: without StringFunctions a String fits
// no visible ToString, and the diagnostic names the candidates considered.
func TestInvocationOverloadOnlyImportedPackagesContribute(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		private import IntegerFunctions::*;
		private import RealFunctions::*;
		attribute s = ToString("x");
	}`, "type.expr", "argument 1 of ToString expects Integer, found String (candidates: IntegerFunctions::ToString, RealFunctions::ToString)")
}

// Among the applicable candidates, the most specific parameter types win: the
// result of the call is typed by that candidate, which a binding then checks.
func TestInvocationOverloadResultTypeFollowsSelection(t *testing.T) {
	wantLibraryClean(t, `package P {`+numericImports+`
		attribute i : Natural = abs(-2);
		attribute b : Boolean = isZero(rect(0.0, 0.0));
	}`)
	wantLibraryDiag(t, `package P {`+numericImports+`
		attribute s : String = abs(-2);
	}`, "type.expr", "cannot bind Natural value to a feature typed by String")
}

// Strings and booleans take part: ToString has a String overload beside the
// numeric ones, and a Boolean argument selects the Boolean one.
func TestInvocationOverloadStringAndBoolean(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import StringFunctions::*;
		private import BooleanFunctions::*;
		private import IntegerFunctions::*;
		attribute a = ToString("x");
		attribute b = ToString(true);
		attribute c = ToString(7);
	}`)
}

// A collection argument binds to a collection parameter, which the sequence
// functions declare.
func TestInvocationOverloadCollections(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		attribute xs : Integer[0..*] = (1, 2, 3);
		attribute n = size(xs);
		attribute h = head(xs);
	}`)
}

// Quantity arguments select by declared type: a mass binds to the candidate
// taking a mass, not to the one taking a length, though both are Real-valued.
func TestInvocationOverloadQuantities(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		package A { calc def weigh { in x : MassValue; return : Boolean = true; } }
		package B { calc def weigh { in x : LengthValue; return : String = "long"; } }
		package C {
			private import A::*;
			private import B::*;
			attribute m : MassValue = 2 [kg];
			attribute l : LengthValue = 3 [m];
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `w : Boolean = weigh(m);`))
	wantLibraryClean(t, fmt.Sprintf(model, `w : String = weigh(l);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `w : String = weigh(m);`),
		"type.expr", "cannot bind Boolean value to a feature typed by String")
}

// Two candidates the arguments fit equally, neither more specific than the
// other, are an ambiguity the checker reports by name rather than resolving.
func TestInvocationOverloadAmbiguous(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; in y : Real; return : Integer = 1; } }
		package B { calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute tied = pick(1, 2);
		}
	}`, "invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
}

// An ambiguity names only the candidates none is more specific than: a broader
// overload the arguments also fit is beaten, so it is not among the tied ones.
func TestInvocationOverloadAmbiguityNamesOnlyTheTiedBest(t *testing.T) {
	diags := libraryDiags(t, `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		package A { calc def pick { in x : P::Mass; return : Integer = 1; } }
		package B { calc def pick { in x : P::Mass; return : Integer = 2; } }
		package C { calc def pick { in x : Real; return : Integer = 3; } }
		package D {
			private import A::*;
			private import B::*;
			private import C::*;
			attribute m : Mass;
			attribute tied = pick(m);
		}
	}`)
	want := "call of pick is ambiguous between P::A::pick, P::B::pick"
	if len(diags) != 1 || diags[0].Code != "invocation-ambiguous" || diags[0].Message != want {
		t.Fatalf("expected one invocation-ambiguous diagnostic %q, got %v", want, diags)
	}
}

// With no applicable candidate the first is checked as before, and the report
// names every candidate considered.
func TestInvocationOverloadNoneApplicable(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; in y : Real; return : Integer = 1; } }
		package B { calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute none = pick("a", 2);
		}
	}`, "type.expr", `argument 1 of pick expects Integer, found String (candidates: P::A::pick, P::B::pick)`)
}

// An argument of unknown type binds to any parameter, so the first candidate
// in lookup order is kept and nothing new is reported.
func TestInvocationOverloadUnknownArgumentType(t *testing.T) {
	wantLibraryClean(t, `package P {`+numericImports+`
		attribute untyped;
		attribute a = abs(untyped);
		attribute m = max(untyped, 2);
	}`)
}

// A parameter declared without a type is typed Anything, the least specific type: a
// candidate typing that parameter wins over it, and a parameter whose type did not
// resolve is undetermined, so lookup order keeps deciding.
func TestInvocationOverloadUntypedParameterIsLeastSpecific(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Real; in y%s; return : String = "loose"; } }
		package B { calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, ``, `typed : Integer = pick(1, 2);`))
	wantLibraryDiag(t, fmt.Sprintf(model, ``, `wrong : String = pick(1, 2);`),
		"type.expr", "cannot bind Integer value to a feature typed by String")
	wantLibraryClean(t, fmt.Sprintf(model, ``, `loose : String = pick(1, "b");`))
	diags := libraryDiags(t, fmt.Sprintf(model, ` : Missing`, `first : String = pick(1, 2);`))
	if len(diags) != 1 || diags[0].Code != "unresolved" || !strings.Contains(diags[0].Message, "Missing") {
		t.Fatalf("expected only the unresolved parameter type, got %v", diags)
	}
}

// A parameter typed `Anything` in writing is the same least specific parameter as one
// declaring no type: the two spellings tie, and either loses to a typed parameter.
func TestInvocationOverloadExplicitAnythingEqualsUntyped(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import Base::Anything;
		package A { calc def pick { in x : Real; in y : Anything; return : String = "a"; } }
		package B { calc def pick { in x : Real; in y%s; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute w : Real = 1.0;
			attribute v : Integer = 2;
			attribute %s
		}
	}`
	wantLibraryDiag(t, fmt.Sprintf(model, ``, `r = pick(w, v);`),
		"invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	wantLibraryDiag(t, fmt.Sprintf(model, ``, `r = pick(1, 2);`),
		"invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	wantLibraryClean(t, fmt.Sprintf(model, ` : Integer`, `r : Integer = pick(w, v);`))
	wantLibraryDiag(t, fmt.Sprintf(model, ` : Integer`, `r : String = pick(w, v);`),
		"type.expr", "cannot bind Integer value to a feature typed by String")
}

// Candidates each narrower on a different parameter are incomparable, so the call is
// ambiguous however the least specific parameter is spelt; typing both wins outright.
func TestInvocationOverloadCrossedSpecificityIsAmbiguous(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import Base::Anything;
		attribute def Foo;
		package A { calc def pick { in x%s; in y : Real; return : String = "a"; } }
		package B { calc def pick { in x : Foo; in y%s; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute f : Foo;
			attribute w : Real = 1.0;
			attribute r%s = pick(f, w);
		}
	}`
	for _, spelling := range [][2]string{{``, ``}, {` : Anything`, ` : Anything`}, {` : Anything`, ``}, {``, ` : Anything`}} {
		wantLibraryDiag(t, fmt.Sprintf(model, spelling[0], spelling[1], ``),
			"invocation-ambiguous", "call of pick is ambiguous between P::A::pick, P::B::pick")
	}
	wantLibraryClean(t, fmt.Sprintf(model, ` : Foo`, ` : Anything`, ` : String`))
	wantLibraryDiag(t, fmt.Sprintf(model, ` : Foo`, ` : Anything`, ` : Integer`),
		"type.expr", "cannot bind String value to a feature typed by Integer")
}

// A calc the model declares under a library function's name shadows every
// library declaration, imported or not, however its arguments are typed.
func TestInvocationOverloadModelShadowsLibrary(t *testing.T) {
	wantLibraryDiag(t, `package P {`+numericImports+`
		calc def abs { in x : String; return : String = x; }
		attribute a = abs(-2.5);
	}`, "type.expr", "argument 1 of abs expects String, found Rational")
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		calc def sqrt { in x : String; return : String = x; }
		attribute a = sqrt("x");
	}`)
}

// Function libraries are not implicitly imported: a bare call to any library function
// is unresolved until its package is imported, and the diagnostic offers those imports.
func TestInvocationUnimportedLibraryFunction(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute r = sqrt(4.0);
	}`, "unresolved", "unresolved reference: sqrt — did you mean RealFunctions::sqrt")
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute e = exp(1.0);
	}`, "unresolved", "unresolved reference: exp")
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute e = nosuchfunction(1.0);
	}`, "unresolved", "unresolved reference: nosuchfunction")
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import RealFunctions::*;
		attribute r = sqrt(4.0);
		attribute q = RealFunctions::sqrt(4.0);
	}`)
}

// A qualified library name resolves whatever the model imports, and its
// arguments are type-checked against the declaration it denotes.
func TestInvocationQualifiedLibraryFunctionArgumentsChecked(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute r = RealFunctions::sqrt("four");
	}`, "type.expr", "argument 1 of sqrt expects Real, found String")
}

// A named argument binds by the parameter's effective name, which is what
// tells two same-arity candidates apart.
func TestInvocationOverloadNamedArguments(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in width : Integer; return : Integer = 1; } }
		package B { calc def pick { in height : Integer; return : String = "h"; } }
		package C {
			private import A::*;
			private import B::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `w : Integer = pick(width = 2);`))
	wantLibraryClean(t, fmt.Sprintf(model, `h : String = pick(height = 2);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `d = pick(depth = 2);`),
		"type.expr", "candidates: P::A::pick, P::B::pick")
}

// A named argument written twice binds its last value, as the evaluator does,
// so only the last value decides the overload and is type-checked.
func TestInvocationOverloadRepeatedNamedArgumentBindsLast(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : String; return : String = "s"; } }
		package C {
			private import A::*;
			private import B::*;
			attribute %s
		}
		calc def one { in x : Integer; return : Integer = x; }
		attribute %s
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `s : String = pick(x = 1, x = "s");`, `o : Integer = one(x = "s", x = 1);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `s : String = pick(x = "s", x = 1);`, `o : Integer = one(x = 1);`),
		"type.expr", "cannot bind Integer value to a feature typed by String")
	wantLibraryDiag(t, fmt.Sprintf(model, `s : String = pick(x = 1, x = "s");`, `o : Integer = one(x = 1, x = "s");`),
		"type.expr", "expects Integer, found String")
}

// Two same-named calcs one wildcard import surfaces are both candidates.
func TestInvocationOverloadSiblingsThroughOneWildcardImport(t *testing.T) {
	const model = `package Lib {
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = x; }
		calc def pick { in x : String; return : String = x; }
	}
	package P {
		private import ScalarValues::*;
		private import Lib::*;
		attribute i : Integer = pick(2);
		attribute s : String = pick("s");
	}`
	for _, d := range libraryDiags(t, model) {
		if d.Severity != SeverityWarning || d.Code != "name-conflict" {
			t.Fatalf("expected only the owned name-conflict warnings, got %v", d)
		}
	}
}

// Parameters typed by sibling specializations of one scalar type are told apart
// by the argument's declared type; a bare scalar is not an ambiguity.
func TestInvocationOverloadSiblingScalarTypes(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		attribute def Volume :> Real;
		package A { calc def label { in m : Mass; return : String = "mass"; } }
		package B { calc def label { in v : Volume; return : Integer = 1; } }
		package C {
			private import A::*;
			private import B::*;
			attribute kg : Mass = 3.0;
			attribute litre : Volume = 2.0;
			attribute r : Real = 1.0;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `m : String = label(kg);`))
	wantLibraryClean(t, fmt.Sprintf(model, `v : Integer = label(litre);`))
	wantLibraryClean(t, fmt.Sprintf(model, `first : String = label(r);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `wrong : Integer = label(kg);`),
		"type.expr", "cannot bind String value to a feature typed by Integer")
}

// A named argument is checked against the parameter it names as a positional
// one is against its position, and a parameter without a default must be named.
func TestInvocationNamedArgumentsAreTypeChecked(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		attribute def Volume :> Real;
		calc def density { in m : Mass; in v : Volume; in scale : Real = 1.0; return : Real = m / v * scale; }
		attribute kg : Mass = 3.0;
		attribute litre : Volume = 2.0;
		attribute %s
	}`
	wantLibraryClean(t, fmt.Sprintf(model, `ok : Real = density(m = kg, v = litre);`))
	wantLibraryClean(t, fmt.Sprintf(model, `scaled : Real = density(v = litre, m = kg, scale = 2);`))
	wantLibraryDiag(t, fmt.Sprintf(model, `s : Real = density(m = "3", v = litre);`),
		"type.expr", `argument m of density expects Real, found String`)
	wantLibraryDiag(t, fmt.Sprintf(model, `b : Real = density(m = kg, v = litre, scale = true);`),
		"type.expr", `argument scale of density expects Real, found Boolean`)
	wantLibraryDiag(t, fmt.Sprintf(model, `partial : Real = density(m = kg);`),
		"type.expr", `density requires an argument for v`)
}

// Candidates come through re-exporting public imports and from general types; an
// inherited member hides a same-named import, as for any other name.
func TestInvocationOverloadReexportedAndInheritedCandidates(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = 1; } }
		package B { calc def pick { in x : String; return : String = x; } }
		package Both {
			public import A::*;
			public import B::*;
		}
		package C {
			private import Both::*;
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
	const inherited = `package P {
		private import ScalarValues::*;
		package B { calc def pick { in x : String; return : String = x; } }
		part def Base {
			calc def pick { in x : Integer; return : Integer = 1; }
		}
		part def Derived :> Base {
			private import B::*;
			attribute %s
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(inherited, `i : Integer = pick(2);`))
	wantLibraryDiag(t, fmt.Sprintf(inherited, `s = pick("s");`),
		"type.expr", "argument 1 of pick expects Integer, found String")
}

// A general type's protected and public imports each contribute their overloads to
// the bodies specializing it, not only the first import's.
func TestInvocationOverloadCandidatesThroughInheritedImports(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		package A { calc def pick { in x : Integer; return : Integer = x; } }
		package B { calc def pick { in x : String; return : String = x; } }
		part def Base {
			protected import A::*;
			%s import B::*;
		}
		part def Derived :> Base {
			attribute i : Integer = pick(2);
			attribute s = pick("s");
		}
		part p : Base {
			attribute i : Integer = pick(2);
			attribute s = pick("s");
		}
	}`
	for _, visibility := range []string{"protected", "public"} {
		for _, d := range libraryDiags(t, fmt.Sprintf(src, visibility)) {
			if d.Code != "name-conflict" || d.Severity != SeverityWarning {
				t.Fatalf("%s import: expected only name-conflict warnings, got %v", visibility, d)
			}
		}
	}
	// A private import is not inherited, so only A's overload is visible.
	diags := libraryDiags(t, fmt.Sprintf(src, "private"))
	if len(diags) != 2 {
		t.Fatalf("expected one rejected call per body, got %v", diags)
	}
	for _, d := range diags {
		if d.Code != "type.expr" || d.Message != "argument 1 of pick expects Integer, found String" {
			t.Fatalf("unexpected diagnostic %v", d)
		}
	}
}

// A collection literal is typed by the type its elements share, so overloads
// differing by element type select; a mixed one selects as an unknown type does.
func TestInvocationOverloadSelectsByCollectionLiteralElementType(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		package A { calc def count { in xs : String[*]; return : Integer = 1; } }
		package B { calc def count { in xs : Integer[*]; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute n : Integer = count(%s);
		}
	}`
	wantLibraryClean(t, fmt.Sprintf(src, `("a", "b")`))
	wantLibraryClean(t, fmt.Sprintf(src, `(1, 2, 3)`))
	wantLibraryClean(t, fmt.Sprintf(src, `(1, "b")`))
	wantLibraryClean(t, fmt.Sprintf(src, `()`))
	wantLibraryDiag(t, fmt.Sprintf(src, `(true, false)`),
		"type.expr", "argument 1 of count expects String, found Boolean (candidates: P::A::count, P::B::count)")

	// Features typed by sibling types select by the declared type their elements share.
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		attribute def Mass :> Real;
		attribute def Volume :> Real;
		package A { calc def total { in ms : P::Mass[*]; return : Integer = 1; } }
		package B { calc def total { in vs : P::Volume[*]; return : Integer = 2; } }
		package C {
			private import A::*;
			private import B::*;
			attribute m1 : Mass; attribute m2 : Mass; attribute v : Volume;
			attribute n : Integer = total((m1, m2));
			attribute k : Integer = total((v));
		}
	}`)
}

// Every general type, owned member and recursive-import descendant contributes its
// declaration; two sharing a name is a warning, not a lost candidate.
func TestInvocationOverloadCandidatesFromEveryGeneralAndRecursiveImport(t *testing.T) {
	diags := libraryDiags(t, `package P {
		private import ScalarValues::*;
		part def ByNumber { calc def pick { in x : Integer; return : Integer = x; } }
		part def ByText { calc def pick { in x : String; return : String = x; } }
		part def Both :> ByNumber, ByText {
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
	if len(diags) != 1 || diags[0].Code != "name-conflict" || diags[0].Severity != SeverityWarning {
		t.Fatalf("expected only the inherited name-conflict warning, got %v", diags)
	}
	diags = libraryDiags(t, `package P {
		private import ScalarValues::*;
		part def Base {
			calc def pick { in x : Integer; return : Integer = x; }
			calc def pick { in x : String; return : String = x; }
		}
		part def Derived :> Base {
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
	for _, d := range diags {
		if d.Code != "name-conflict" || d.Severity != SeverityWarning {
			t.Fatalf("expected only name-conflict warnings for Base's overloads, got %v", diags)
		}
	}
	diags = libraryDiags(t, `package P {
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = x; }
		calc def pick { in x : String; return : String = x; }
		attribute i : Integer = pick(2);
		attribute s : String = pick("s");
		attribute qi : Integer = P::pick(2);
		attribute qs : String = P::pick("s");
	}`)
	if len(diags) != 2 {
		t.Fatalf("expected only the two owned name-conflict warnings, got %v", diags)
	}
	for _, d := range diags {
		if d.Code != "name-conflict" || d.Severity != SeverityWarning {
			t.Fatalf("expected only owned name-conflict warnings, got %v", diags)
		}
	}
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		package Lib {
			package Numbers { calc def pick { in x : Integer; return : Integer = x; } }
			package Text { calc def pick { in x : String; return : String = x; } }
		}
		package C {
			private import Lib::**;
			attribute i : Integer = pick(2);
			attribute s : String = pick("s");
		}
	}`)
}
