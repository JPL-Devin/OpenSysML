package passes

import (
	"strings"
	"testing"
)

// binaryEndLines returns the 1-based line of every diagnostic reporting an end
// beyond the two a binary link allows, in order.
func binaryEndLines(t *testing.T, src string, kerml bool) []int {
	t.Helper()
	var diags []Diagnostic
	if kerml {
		diags = constraintDiagsKerML(t, src)
	} else {
		diags = constraintDiags(t, src)
	}
	var lines []int
	for _, d := range only(diags, "connector-ends") {
		if !strings.Contains(d.Message, "binary link") {
			continue
		}
		lines = append(lines, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	return lines
}

// The corpus case semantic/k26-binary-connector-three-ends.kerml: a connector
// typed by BinaryLink states three positional ends. Each end past the second is
// reported at its own span; an untyped n-ary connector is not binary.
func TestBinaryConnectorPositionalEnds(t *testing.T) {
	const src = `package P {
	class T {
		feature x : T; feature y : T; feature z : T;
		connector c : Links::BinaryLink (x, y, z);
		connector d : Links::BinaryLink (x, y,
			z,
			x);
		connector ok : Links::BinaryLink (x, y);
		connector nary (x, y, z);
		connector e (x, y);
	}
}`
	if got, want := binaryEndLines(t, src, true), []int{4, 6, 7}; !sameLines(got, want) {
		t.Fatalf("binary link diagnostics on lines %v, want %v", got, want)
	}
}

// Declared `end` features count alongside positional and inherited ends: the
// corpus case semantic/k24-binary-assoc-three-ends.kerml and a connector adding
// a third end to an inherited binary pair. Owned ends redefine inherited ends by
// position or by name, so a single added end or a redefined pair adds nothing.
func TestBinaryConnectorDeclaredAndInheritedEnds(t *testing.T) {
	const src = `package P {
	class T { feature x : T; feature y : T; feature z : T; }
	assoc A specializes Links::BinaryLink { end a : T; end b : T; end c : T; }
	assoc B specializes Links::BinaryLink { end a : T; end b : T; }
	assoc C specializes B { end c : T; }
	assoc D specializes B { end redefines a : T; end redefines b : T; }
	assoc E specializes B { end a : T; end b : T; }
	interaction F specializes Links::BinaryLink { end a : T; end b : T; end c : T; }
	interaction G { end a : T; end b : T; }
	classifier K {
		feature x : T; feature y : T; feature z : T;
		connector k1 : B (x, y);
		connector k2 : B from x to y;
		connector k3 : B (x, y, z);
		connector k4 : B { end redefines a references x; end redefines b references y; }
		connector k5 : B { end feature p ::> x; end feature q ::> y; end feature r ::> z; }
	}
}`
	if got, want := binaryEndLines(t, src, true), []int{3, 8, 14, 16}; !sameLines(got, want) {
		t.Fatalf("binary link diagnostics on lines %v, want %v", got, want)
	}
}

// An interaction of any arity inherits both Link's `participant` (`source`/`target`
// when binary) and Performance's `subperformances`, so redefining them is clean.
func TestInteractionInheritsLinkAndPerformanceMembers(t *testing.T) {
	diags := analyzeAll(t, "interaction.kerml", `package P {
	class T;
	abstract interaction Zero {
		feature redefines participant : T[2..*];
		step redefines subperformances;
	}
	interaction Two {
		end redefines source : T;
		end redefines target : T;
		step redefines subperformances;
	}
	interaction Three {
		end a : T; end b : T; end c : T;
		feature redefines participant : T[3];
		step redefines subperformances;
	}
}`)
	if len(diags) != 0 {
		t.Fatalf("interaction members analysed with diagnostics: %v", diags)
	}
}

// A binary pair inherited along both paths of a diamond is still one pair; only
// a leaf adding a third end of its own is reported.
func TestBinaryConnectorDiamondInheritance(t *testing.T) {
	const src = `package P {
	class T { feature x : T; feature y : T; feature z : T; }
	classifier K {
		feature x : T; feature y : T; feature z : T;
		connector base : Links::BinaryLink from x to y;
		connector mid1 :> base;
		connector mid2 :> base;
		connector leaf :> mid1, mid2;
		connector leaf2 :> mid1, mid2 from x to y;
		connector leaf3 :> mid1, mid2 (x, y, z);
	}
}`
	if got, want := binaryEndLines(t, src, true), []int{10}; !sameLines(got, want) {
		t.Fatalf("binary link diagnostics on lines %v, want %v", got, want)
	}
}

// SysML spellings of a binary connector: a binding, a succession, a flow and a
// connection typed by a binary definition are all binary; only a genuine third
// end is reported.
func TestBinaryConnectorSysMLShapes(t *testing.T) {
	const src = `package P {
	part def T { attribute v; }
	connection def CD { end a : T; end b : T; }
	part p {
		part x : T; part y : T; part z : T;
		connection c1 : CD connect x to y;
		connection c2 : CD connect (x, y, z);
		connection c3 : Connections::BinaryConnection connect (x, y, z);
		connection c4 connect (x, y, z);
		binding bind x = y;
		succession first x then y;
		flow from x.v to y.v;
		allocation al allocate x to y;
	}
}`
	if got, want := binaryEndLines(t, src, false), []int{7, 8}; !sameLines(got, want) {
		t.Fatalf("binary link diagnostics on lines %v, want %v", got, want)
	}
}

// A declaration owning two ends that redefine two of an n-ary general's ends
// inherits the rest, so it stays n-ary and takes no binary base (KerML 1.1
// checkAssociationBinarySpecialization, checkConnectorBinarySpecialization);
// only a declared binary base makes the inherited third end excessive.
func TestBinaryConnectorInheritedThirdEndStaysNary(t *testing.T) {
	const src = `package P {
	class T { feature x : T; feature y : T; feature z : T; }
	assoc N { end a : T; end b : T; end c : T; }
	assoc M specializes N { end redefines a : T; end redefines b : T; }
	assoc O specializes N { end p : T; end q : T; }
	assoc Q specializes N, Links::BinaryLink { end redefines a : T; end redefines b : T; }
	interaction I specializes N { end redefines a : T; end redefines b : T; }
	assoc A { end source : T; end target : T; }
	assoc B { end source : T; end target : T; }
	assoc OfA specializes A, B { end redefines A::source : T; end redefines A::target : T; }
	assoc A3 { end source : T; end target : T; end via : T; }
	assoc B3 { end source : T; end target : T; end via : T; }
	assoc OfA3 specializes A3, B3 { end redefines A3::source : T; end redefines A3::target : T; }
	classifier K {
		feature x : T; feature y : T; feature z : T;
		connector ofA : A, B { end redefines A::source references x; end redefines A::target references y; }
		connector ofA3 : A3, B3 { end redefines A3::source references x; end redefines A3::target references y; }
		connector n : N (x, y, z);
		connector m : N { end redefines a references x; end redefines b references y; }
		connector o :> n { end redefines a references x; end redefines b references y; }
		connector q :> n, Links::binaryLinks { end redefines a references x; end redefines b references y; }
	}
}`
	// OfA/ofA conform to BinaryLink through binary A and still inherit B's ends.
	if got, want := binaryEndLines(t, src, true), []int{6, 10, 16, 21}; !sameLines(got, want) {
		t.Fatalf("binary link diagnostics on lines %v, want %v", got, want)
	}
}
