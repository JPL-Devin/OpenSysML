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

// A local name several imported function packages declare is bound to the
// declaration whose parameter types the arguments conform to, so every
// acceptance probe checks clean where the first import alone would not.
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

// Importing part of the library does not hide the rest of it: with only the
// numeric packages imported, a Complex argument still selects ComplexFunctions::abs.
func TestInvocationOverloadImportedLibraryJoinsInForceLibrary(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		private import IntegerFunctions::*;
		private import RealFunctions::*;
		private import RationalFunctions::*;
		attribute c : Real = abs(rect(3.0, 4.0));
		attribute z : Boolean = isZero(rect(0.0, 0.0));
	}`)
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

// A Kernel Function Library function called by its bare name resolves without
// an import of its package, as the runtime evaluates it; an OpenSysML extension
// function and a name no library declares stay unresolved.
func TestInvocationUnimportedLibraryFunction(t *testing.T) {
	wantLibraryClean(t, `package P {
		private import ScalarValues::*;
		attribute r = sqrt(4.0);
		attribute a = abs(-2);
		attribute n = size(r);
	}`)
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute e = exp(1.0);
	}`, "unresolved", "unresolved reference: exp")
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute e = nosuchfunction(1.0);
	}`, "unresolved", "unresolved reference: nosuchfunction")
}

// A library name reached without an import is still type-checked against the
// declaration it denotes.
func TestInvocationUnimportedLibraryFunctionArgumentsChecked(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute r = sqrt("four");
	}`, "type.expr", "argument 1 of sqrt expects Real, found String")
}

// A bare library name outside a call is not in force: only an invocation
// reaches the Kernel Function Library unimported.
func TestUnimportedLibraryNameOutsideInvocationUnresolved(t *testing.T) {
	wantLibraryDiag(t, `package P {
		private import ScalarValues::*;
		attribute f = sqrt;
	}`, "unresolved", "unresolved reference: sqrt")
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

// Candidates are gathered through a public import re-exporting them and from
// the general type a definition inherits them from; an inherited member hides
// what an import of the same name would bring in, as for any other name.
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

// Every general type, every owned member and every descendant of a recursive
// import contributes its declaration; two sharing a name is a warning, not a
// lost candidate.
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
		calc def pick { in x : Integer; return : Integer = x; }
		calc def pick { in x : String; return : String = x; }
		attribute i : Integer = pick(2);
		attribute s : String = pick("s");
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
