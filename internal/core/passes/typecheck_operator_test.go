package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// operatorDiags is the type-tier diagnostics under code of src, analysed as a
// document named name against the bundled library; the name's extension
// decides the language.
func operatorDiags(t *testing.T, name, code, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	var out []Diagnostic
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Severity == SeverityError && d.Source != "type" {
			t.Fatalf("%s does not analyse cleanly: %v", name, d)
		}
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// wantOperatorDiags asserts the diagnostics under code, in source order, carry
// exactly the given "line:col: fragment" verdicts; every one is a warning.
func wantOperatorDiags(t *testing.T, name, code, src string, want ...string) {
	t.Helper()
	diags := operatorDiags(t, name, code, src)
	var got []string
	for _, d := range diags {
		if d.Severity != SeverityWarning {
			t.Errorf("%v is not a warning", d)
		}
		pos := source.New(name, []byte(src)).Lines().PosAt(d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, d.Message))
	}
	if len(got) != len(want) {
		t.Fatalf("want %d %s diagnostics, got %d:\n%s", len(want), code, len(got), strings.Join(got, "\n"))
	}
	for i, w := range want {
		at, fragment, _ := strings.Cut(w, " ")
		if !strings.HasPrefix(got[i], at+": ") || !strings.Contains(got[i], fragment) {
			t.Errorf("diagnostic %d = %q, want %q at %s", i, got[i], fragment, at)
		}
	}
}

// KerML `x[i]` names a function the kernel library leaves abstract: each
// bracket warns, wherever it sits in the tree; `#(...)` is the index.
func TestBracketOperatorInKerML(t *testing.T) {
	wantOperatorDiags(t, "a.kerml", codeBracketOperator, `package P {
	feature xs : ScalarValues::Integer[0..*] ordered;
	feature firstElem = xs[1];
	feature second = xs#(2);
	feature two = xs[1, 2];
	feature twice = xs[1][2];
	feature cast = (xs as ScalarValues::Real)[1];
	function F { in s : ScalarValues::Integer[0..*]; return : ScalarValues::Integer = s[1]; }
	feature units = 10 [SI::m];
}`,
		"3:22 `x[i]` is not an index in KerML",
		"5:16 `x[i]` is not an index in KerML",
		"6:18 `x[i]` is not an index in KerML",
		"6:18 `x[i]` is not an index in KerML",
		"7:17 `x[i]` is not an index in KerML",
		"8:84 `x[i]` is not an index in KerML",
		"9:18 `x[i]` is not an index in KerML",
	)
}

// SysML gives `[` the quantity functions, so a SysML document never gets the
// KerML warning, and a KerML document never gets the SysML one.
func TestBracketOperatorIsKerMLOnly(t *testing.T) {
	sysml := `package P {
	private import SI::*;
	attribute xs : ScalarValues::Integer[0..*] ordered;
	attribute firstElem = xs[1];
	attribute length = 10 [m];
}`
	wantOperatorDiags(t, "a.sysml", codeBracketOperator, sysml)
	wantOperatorDiags(t, "a.sysml", codeQuantityUnit, sysml,
		"4:27 the unit of a quantity must be a measurement reference, found Natural")
	wantOperatorDiags(t, "a.kerml", codeQuantityUnit, `package P {
	feature xs : ScalarValues::Integer[0..*] ordered;
	feature firstElem = xs[1];
}`)
}

const castFixture = `package P {
	private import ScalarValues::*;
	datatype A; datatype B :> A; datatype C; datatype D :> A, C;
	feature a : A;
	feature ab : A, C;
	feature s : String;
	classifier Box { feature base : A; feature str : String; }
	feature box : Box;
	classifier Box2 :> Box { feature :>> base; %s }
	function F { return r : A; }
	classifier Q; classifier R :> Q; classifier CQ ~ Q; feature cq : CQ;
	feature xs : A[*];
	feature d : D;
	feature untyped;
	feature valued = 3;
	%s
}`

// castDiags analyses a KerML cast fixture with member written in Box2 and body
// at package level.
func castDiags(t *testing.T, member, body string, want ...string) {
	t.Helper()
	src := strings.Replace(castFixture, "%s", member, 1)
	src = strings.Replace(src, "%s", body, 1)
	wantOperatorDiags(t, "a.kerml", codeCastConformance, src, want...)
}

// A cast whose argument's types and target specialize one another in neither
// direction selects nothing and warns, for every shape an argument takes.
func TestCastConformanceUnrelatedTypes(t *testing.T) {
	castDiags(t, "", `feature bad = a as C;`, "16:16 cast argument is typed by A, unrelated to the target C")
	castDiags(t, "", `feature bad = s as Integer;`, "16:16 cast argument is typed by String, unrelated to the target Integer")
	castDiags(t, "", `feature bad = 3 as String;`, "16:16 cast argument is typed by Integer, unrelated to the target String")
	castDiags(t, "", `feature bad = valued as String;`, "16:16 cast argument is typed by Integer, unrelated to the target String")
	castDiags(t, "", `feature bad = ab as String;`, "16:16 cast argument is typed by A and C, unrelated to the target String")
	castDiags(t, "", `feature bad = box.base as String;`, "16:16 cast argument is typed by A, unrelated to the target String")
	castDiags(t, "", `feature bad = F() as String;`, "16:16 cast argument is typed by A, unrelated to the target String")
	castDiags(t, "", `feature bad = (a as B) as C;`, "16:16 cast argument is typed by B, unrelated to the target C")
	castDiags(t, "", `feature bad = (1 < 2) as Integer;`, "16:16 cast argument is typed by Boolean, unrelated to the target Integer")
	castDiags(t, "", `feature bad = xs.?{in x; true} as C;`, "16:16 cast argument is typed by A, unrelated to the target C")
	castDiags(t, "", `feature bad = a#(1) as C;`, "16:16 cast argument is typed by A, unrelated to the target C")
	castDiags(t, "", `feature bad = a as s;`, "16:16 cast argument is typed by A, unrelated to the target s")
	castDiags(t, "", `feature bad = cq as R;`, "16:16 cast argument is typed by CQ, unrelated to the target R")
	castDiags(t, `feature bad = base as String;`, "", "9:59 cast argument is typed by A, unrelated to the target String")
	castDiags(t, "", `feature two = (a as C) as String;`,
		"16:16 cast argument is typed by C, unrelated to the target String",
		"16:17 cast argument is typed by A, unrelated to the target C")
}

// A cast up, down, or sideways through one of several types conforms; so does
// one whose argument's type is not statically known, or is Anything.
func TestCastConformanceRelatedTypes(t *testing.T) {
	castDiags(t, "", `feature up = a as Base::Anything; feature down = a as B; feature same = a as A; feature self = a as a;`)
	castDiags(t, "", `feature one = ab as B; feature other = ab as C; feature viaD = d as C;`)
	castDiags(t, "", `feature chain = box.base as B; feature call = F() as B; feature nested = (a as B) as A;`)
	castDiags(t, `feature redef = base as B;`, "")
	castDiags(t, "", `feature conjugate = cq as Q;`)
	castDiags(t, "", `feature wide = untyped as String; feature nothing = null as A; feature real = 3 as Real;`)
	castDiags(t, "", `feature data = (1 + 2) as String; feature seq = (1, 2) as Integer; feature body = xs.{in x; x} as C;`)
	castDiags(t, "", `feature cond = (if true ? a else a) as C; feature sel = xs.?{in x; true} as B;`)
}

// The rule is KerML's, but SysML declares the same operator: a usage cast to an
// unrelated definition warns there too.
func TestCastConformanceInSysML(t *testing.T) {
	wantOperatorDiags(t, "a.sysml", codeCastConformance, `package P {
	private import ScalarValues::*;
	private import ISQ::*;
	private import Quantities::*;
	part def PD { attribute a : A; }
	attribute def A; attribute def B :> A;
	part p : PD;
	attribute q : MassValue;
	attribute chainOk = p.a as B;
	attribute chainBad = p.a as String;
	attribute castOk = q as ScalarQuantityValue;
	attribute castBad = q as LengthValue;
	attribute unitOk = 10 [q as MeasurementReferences::MeasurementUnit];
	attribute unitBad = 10 [(q as Integer) as MeasurementReferences::MeasurementUnit];
}`,
		"10:23 cast argument is typed by A, unrelated to the target String",
		"12:22 cast argument is typed by MassValue, unrelated to the target LengthValue",
		"13:25 cast argument is typed by MassValue, unrelated to the target MeasurementUnit",
		"14:26 cast argument is typed by Integer, unrelated to the target MeasurementUnit",
		"14:27 cast argument is typed by MassValue, unrelated to the target Integer",
	)
}

const quantityImports = "private import ScalarValues::*;\nprivate import ISQ::*;\nprivate import SI::*;\nprivate import MeasurementReferences::*;\nprivate import Quantities::*;\n"

func quantityDiags(t *testing.T, body string, want ...string) {
	t.Helper()
	wantOperatorDiags(t, "a.sysml", codeQuantityUnit, "package P {\n"+quantityImports+body+"\n}", want...)
}

// The unit of a quantity is a measurement reference: a unit, an aliased or
// custom one, a feature typed by one, a frame, and arithmetic or selection
// over them. An operator is judged by the operands it is passed by value, as
// the pilot judges it: `(m, 3)` and `m ?? 3` pass, a conditional does not.
func TestQuantityUnitMeasurementReferences(t *testing.T) {
	quantityDiags(t, `attribute a = 10 [m];
	attribute pair = 10 [m, s];
	alias metre for m;
	attribute aliased = 10 [metre];
	attribute typed : LengthUnit = m;
	attribute viaFeature = 10 [typed];
	attribute frame : CoordinateFrame;
	attribute vector = (1, 2, 3) [frame];
	attribute product = 10 [m * s];
	attribute quotient = 10 [m / s];
	attribute power = 10 [m ** 2];
	attribute scaled = 10 [m * 2];
	attribute prescaled = 10 [2 * m];
	attribute negated = 10 [-m];
	attribute derived = 10 [km / h];
	attribute fallback = 10 [m ?? 3];
	attribute mixed = 10 [(m, 3)];
	attribute cast = 10 [(m * s) as LengthUnit];
	attribute called = 10 [MeasurementRefCalculations::'*'(m, s)];
	attribute def Custom :> MeasurementUnit;
	attribute custom : Custom;
	attribute viaCustom = 10 [custom];
	calc def Convert { in mag : Real; in unit : MeasurementUnit; return : ScalarQuantityValue = mag [unit]; }
	attribute sequence = (1, 2) [m];
	attribute units : MeasurementUnit[0..*] ordered;
	attribute selected = 10 [units#(1)];
	attribute frame3d : CartesianSpatial3dCoordinateFrame;
	attribute component = 10 [frame3d.mRefs#(1)];
	attribute picked = 10 [(m, s)#(2)];`)
}

// Anything else in the unit position warns at the unit: a number, a string, a
// quantity, a dimensionless computation, an untyped feature, a conditional (its
// branches are expression bodies, so its result is Anything); a unit nested in
// the unit is judged on its own.
func TestQuantityUnitNotAMeasurementReference(t *testing.T) {
	quantityDiags(t, `attribute n : Integer;
	attribute q : MassValue;
	attribute product = m * s;
	attribute a = 10 [3];
	attribute b = 10 ["m"];
	attribute c = 10 [n];
	attribute d = 10 [q];
	attribute e = 10 [2 * 3];
	attribute f = 10 [-3];
	attribute g = 10 [2 ** 2];
	attribute h = 10 [if true ? 1 else 2];
	attribute i = 10 [product as Integer];
	attribute j = 10 [ScalarFunctions::'+'(1, 2)];
	calc def Convert { in mag : Real; in unit; return : ScalarQuantityValue = mag [unit]; }
	attribute xs : Integer[0..*] ordered;
	attribute k = xs[1];
	attribute l = 10 [1] [2];
	attribute o = 10 [xs#(1)];
	attribute p = 10 [if true ? m else s];
	attribute r = 10 [3 ?? m];
	attribute t = 10 [2 [m]];
	attribute u = 10 [m * 3 ["s"]];
	attribute v = 10 [2 [3]];`,
		"10:20 found Natural",
		"11:20 found String",
		"12:20 found Integer",
		"13:20 found MassValue",
		"14:20 found the result of `*`, typed by DataValue",
		"15:20 found the result of `-`, typed by DataValue",
		"16:20 found the result of `**`, typed by DataValue",
		"17:20 found the result of `if`, typed by Anything",
		"18:20 found the result of `as`, typed by Integer",
		"19:20 found ScalarValue",
		"20:81 found an untyped feature",
		"22:19 found Natural",
		"23:20 found Natural",
		"23:24 found Natural",
		"24:20 found the result of `#`, typed by Integer",
		"25:20 found the result of `if`, typed by Anything",
		"26:20 found the result of `??`, typed by Anything",
		"27:20 found a quantity in metre",
		"28:27 found String",
		"29:20 found ScalarQuantityValue",
		"29:23 found Natural",
	)
}

// The unit is judged wherever an expression is typed: guards, trigger times,
// assignments, payloads, and the bodies of constraints, calcs and requirements.
func TestQuantityUnitInEveryExpressionPosition(t *testing.T) {
	quantityDiags(t, `constraint def Chk { in v : Real; 3 [m] > v [1] }
	calc def Calc { in v : Real; (v + 1) [2] }
	action def Act {
		attribute z : Real;
		action a1 { accept after 5 [1]; }
		then action a2 { assign z := 5 [3]; }
		then if 3 [1] > 2 [m] { action a3 { send 4 [1] to a2; } }
	}
	state def SM {
		entry; then s1;
		state s1;
		transition first s1 if 3 [1] > 2 [m] then s2;
		state s2;
	}
	requirement def Req { require constraint { 3 [1] > 2 [m] } }`,
		"7:46 found Natural",
		"8:40 found Natural",
		"11:31 found Natural",
		"12:35 found Natural",
		"13:14 found Natural",
		"13:47 found Natural",
		"18:29 found Natural",
		"21:48 found Natural",
	)
}
