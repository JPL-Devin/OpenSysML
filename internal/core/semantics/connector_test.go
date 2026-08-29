package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// connector returns the single unnamed connector usage declared in scope: a
// `connection : X connect ...` names nothing of its own.
func connector(t *testing.T, scope *symbols.Scope) *symbols.Symbol {
	t.Helper()
	var found *symbols.Symbol
	for _, s := range scope.AllMembers() {
		if s.Name != "" || !connectorLike(s) {
			continue
		}
		if found != nil {
			t.Fatalf("more than one unnamed connector in scope")
		}
		found = s
	}
	if found == nil {
		t.Fatalf("no unnamed connector in scope")
	}
	return found
}

// TestImplicitEndRedefinitionOfConnectionUsage covers SysML v2 7.13.2: an end
// a connection usage names in its connect clause redefines the end at the same
// position of the connection definition that types it, and so takes its type.
func TestImplicitEndRedefinitionOfConnectionUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def TireBead;
		part def Rim;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : Rim;
		}
		part w {
			part t { part bead : TireBead; }
			part wheel { part rim : Rim; }
			connection : PressureSeat connect
				bead references t.bead to
				rim references wheel.rim;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "PressureSeat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	for _, name := range []string{"bead", "rim"} {
		end := nested(t, conn.Scope, name)
		supers := m.DirectSupertypes(end)
		if len(supers) != 1 || supers[0] != nested(t, seat.Scope, name) {
			t.Fatalf("DirectSupertypes(%s) = %v, want [PressureSeat::%s]", name, supers, name)
		}
	}
	// The redefined end supplies the type.
	if !m.Conforms(nested(t, conn.Scope, "bead"), nested(t, p.Scope, "TireBead")) {
		t.Fatalf("the connection's bead end does not conform to TireBead")
	}
}

// TestImplicitEndRedefinitionIsPositional covers that the match is by position,
// not by name: a renamed end redefines the end at its own position.
func TestImplicitEndRedefinitionIsPositional(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def TireBead;
		part def Rim;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : Rim;
		}
		part w {
			part t { part b; }
			part wheel { part r; }
			connection : PressureSeat connect
				outer references t.b to
				inner references wheel.r;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "PressureSeat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	if supers := m.DirectSupertypes(nested(t, conn.Scope, "outer")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "bead") {
		t.Fatalf("DirectSupertypes(outer) = %v, want [PressureSeat::bead]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, conn.Scope, "inner")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "rim") {
		t.Fatalf("DirectSupertypes(inner) = %v, want [PressureSeat::rim]", supers)
	}
}

// TestImplicitEndRedefinitionCountsUnnamedEnds covers that positions are
// counted over the whole connect clause: an end that only names what it
// attaches to still occupies its position, so a following end that declares a
// name of its own redefines the end at its own position, not the first one.
func TestImplicitEndRedefinitionCountsUnnamedEnds(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def TireBead;
		part def Rim;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : Rim;
		}
		part w {
			part wheel { part r : Rim; }
			part b : TireBead;
			connection : PressureSeat connect b to seatRim references wheel.r;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "PressureSeat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	if supers := m.DirectSupertypes(nested(t, conn.Scope, "seatRim")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "rim") {
		t.Fatalf("DirectSupertypes(seatRim) = %v, want [PressureSeat::rim]", supers)
	}
}

// TestImplicitEndRedefinitionOfInterfaceUsage covers SysML v2 7.14.2, where the
// ends are ports bound with `::>` rather than `references`.
func TestImplicitEndRedefinitionOfInterfaceUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def FuelOutPort;
		port def FuelInPort;
		interface def FuelInterface {
			end supplierPort : FuelOutPort;
			end consumerPort : FuelInPort;
		}
		part vehicle {
			part tankAssy { port fuelTankPort : FuelOutPort; }
			part eng { port engineFuelPort : FuelInPort; }
			interface : FuelInterface connect
				supplierPort ::> tankAssy.fuelTankPort to
				consumerPort ::> eng.engineFuelPort;
		}
	}`)
	p := sym(t, root, "P")
	def := nested(t, p.Scope, "FuelInterface")
	iface := connector(t, nested(t, p.Scope, "vehicle").Scope)

	for _, name := range []string{"supplierPort", "consumerPort"} {
		supers := m.DirectSupertypes(nested(t, iface.Scope, name))
		if len(supers) != 1 || supers[0] != nested(t, def.Scope, name) {
			t.Fatalf("DirectSupertypes(%s) = %v, want [FuelInterface::%s]", name, supers, name)
		}
	}
}

// TestExplicitEndRedefinitionGoverns covers that an end declaring `:>>` keeps
// redefining what it names, whatever its position.
func TestExplicitEndRedefinitionGoverns(t *testing.T) {
	m, root := buildModel(t, `package P {
		connection def Seat { end [1] part bead; end [1] part rim; }
		part w {
			part t { part b; }
			part wheel { part r; }
			connection : Seat connect
				second :>> Seat::rim references wheel.r to
				first :>> Seat::bead references t.b;
		}
	}`)
	p := sym(t, root, "P")
	seat := nested(t, p.Scope, "Seat")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	if supers := m.DirectSupertypes(nested(t, conn.Scope, "second")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "rim") {
		t.Fatalf("DirectSupertypes(second) = %v, want [Seat::rim]", supers)
	}
	if supers := m.DirectSupertypes(nested(t, conn.Scope, "first")); len(supers) != 1 ||
		supers[0] != nested(t, seat.Scope, "bead") {
		t.Fatalf("DirectSupertypes(first) = %v, want [Seat::bead]", supers)
	}
}

// TestConnectorEndRedefinitionNegativeCases covers the boundaries of the rule:
// an end beyond the last position of the general connector redefines nothing,
// an untyped connector has nothing to redefine, and an end of a connect clause
// that names an existing feature is a reference, not a declaration.
func TestConnectorEndRedefinitionNegativeCases(t *testing.T) {
	t.Run("more ends than the general connector", func(t *testing.T) {
		m, root := buildModel(t, `package P {
			connection def Seat { end [1] part bead; }
			part w {
				part a; part b;
				connection : Seat connect e1 references a to e2 references b;
			}
		}`)
		p := sym(t, root, "P")
		conn := connector(t, nested(t, p.Scope, "w").Scope)
		if supers := m.DirectSupertypes(nested(t, conn.Scope, "e2")); len(supers) != 0 {
			t.Fatalf("DirectSupertypes(e2) = %v, want none", supers)
		}
		general, unmatched := m.UnmatchedConnectorEnds(conn)
		if general == nil || general.Name != "Seat" || len(unmatched) != 1 || unmatched[0].Name != "e2" {
			t.Fatalf("UnmatchedConnectorEnds = (%v, %v), want (Seat, [e2])", general, unmatched)
		}
	})

	t.Run("untyped connector", func(t *testing.T) {
		m, root := buildModel(t, `package P {
			part w {
				part a; part b;
				connection connect e1 references a to e2 references b;
			}
		}`)
		p := sym(t, root, "P")
		conn := connector(t, nested(t, p.Scope, "w").Scope)
		if general, unmatched := m.UnmatchedConnectorEnds(conn); general != nil || unmatched != nil {
			t.Fatalf("UnmatchedConnectorEnds = (%v, %v), want none", general, unmatched)
		}
	})

	t.Run("an end naming an existing feature declares nothing", func(t *testing.T) {
		_, root := buildModel(t, `package P {
			connection def Seat { end [1] part bead; end [1] part rim; }
			part w {
				part a; part b;
				connection : Seat connect a to b;
			}
		}`)
		p := sym(t, root, "P")
		conn := connector(t, nested(t, p.Scope, "w").Scope)
		if _, ok := conn.Scope.LookupLocal("a"); ok {
			t.Fatalf("connect a to b must not declare an end named a")
		}
	})
}

// TestImplicitEndRedefinitionOfAssociationUsage covers that the rule is stated
// over associations (SysML v2 7.13.2, KerML 7.4.5): a connection usage typed by
// an association redefines the association's ends by position too.
func TestImplicitEndRedefinitionOfAssociationUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def Owner;
		part def Owned;
		assoc Ownership {
			end owner : Owner;
			end owned : Owned;
		}
		part w {
			part o : Owner;
			part d : Owned;
			connection : Ownership connect
				theOwner references o to
				theOwned references d;
		}
	}`)
	p := sym(t, root, "P")
	assoc := nested(t, p.Scope, "Ownership")
	conn := connector(t, nested(t, p.Scope, "w").Scope)

	for _, c := range []struct{ end, redefined string }{
		{"theOwner", "owner"},
		{"theOwned", "owned"},
	} {
		supers := m.DirectSupertypes(nested(t, conn.Scope, c.end))
		if len(supers) != 1 || supers[0] != nested(t, assoc.Scope, c.redefined) {
			t.Fatalf("DirectSupertypes(%s) = %v, want [Ownership::%s]", c.end, supers, c.redefined)
		}
	}
}

// TestConnectorEndReferenceIsNotASiblingEnd covers that the feature an end
// attaches to is a feature of the connector's owner, not of the connector, so a
// name it shares with a sibling end names the owner's feature. The model is
// queried before any document walk, since the walk's memo would otherwise
// supply the answer.
func TestConnectorEndReferenceIsNotASiblingEnd(t *testing.T) {
	const name = "t.sysml"
	src := `package P {
		part def TireBead;
		connection def PressureSeat {
			end [1] part bead : TireBead;
			end [1] part rim : TireBead;
		}
		part wheelAssy {
			part outer : TireBead;
			part bead : TireBead;
			connection : PressureSeat connect
				bead references outer to
				rim references bead;
		}
	}`
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)

	pkg := sym(t, idx.DocumentRoot(name), "P")
	wheel := nested(t, pkg.Scope, "wheelAssy")
	conn := connector(t, wheel.Scope)
	got := m.ReferencedFeature(nested(t, conn.Scope, "rim"))
	if got != nested(t, wheel.Scope, "bead") {
		t.Fatalf("ReferencedFeature(rim) = %v (kind %v), want wheelAssy::bead", got, got.Kind)
	}
}

// TestImplicitEndRedefinitionCountsUnnamedBodyEnds covers that an `end` feature
// of a connector's body occupies its position even when it declares no name, so
// the ends after it match the general's ends at the right position.
func TestImplicitEndRedefinitionCountsUnnamedBodyEnds(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def B;
		connection def Base {
			end [1] part one : A;
			end [1] part two : B;
		}
		connection def Sub specializes Base {
			end [1] : A;
			end [1] part later : B;
		}
	}`)
	p := sym(t, root, "P")
	base := nested(t, p.Scope, "Base")
	sub := nested(t, p.Scope, "Sub")

	supers := m.DirectSupertypes(nested(t, sub.Scope, "later"))
	if want := nested(t, base.Scope, "two"); len(supers) == 0 || supers[len(supers)-1] != want {
		t.Fatalf("DirectSupertypes(later) = %v, want the last to be Base::two", supers)
	}
}

// A connector usage is recognized whether it names a definition or not, and
// whether it names itself or not: what makes it one is the connect clause
// (SysML v2 §7.13.2, §8.3.13).
func TestIsConnectorUsage(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def Pt;
		part def A { port p : Pt; }
		connection def Link { end source : Pt; end target : Pt; }
		part w {
			part a : A;
			part b : A;
			attribute plain;
			part nested { port q : Pt; }
			connection typed : Link connect a.p to b.p;
			interface untyped connect a.p to b.p;
			connect a.p to b.p;
		}
	}`)
	w := nested(t, sym(t, root, "P").Scope, "w")
	for _, name := range []string{"typed", "untyped"} {
		if !m.IsConnectorUsage(nested(t, w.Scope, name)) {
			t.Errorf("IsConnectorUsage(%s) = false, want true", name)
		}
	}
	if !m.IsConnectorUsage(connector(t, w.Scope)) {
		t.Error("IsConnectorUsage(anonymous connect) = false, want true")
	}
	for _, name := range []string{"a", "plain", "nested"} {
		if m.IsConnectorUsage(nested(t, w.Scope, name)) {
			t.Errorf("IsConnectorUsage(%s) = true, want false: it connects nothing", name)
		}
	}
}

// The attachments of a connector are the features its connect clause names, in
// the order written, each under the name its end is known by — the end's own
// name when it declares one, otherwise the definition's end at that position.
func TestConnectorEndAttachments(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def Pt;
		part def A { port p : Pt; part inner { port q : Pt; } }
		connection def Link { end source : Pt; end target : Pt; }
		part w {
			part a : A;
			part b : A;
			connection typed : Link connect a.p to b.inner.q;
			connection named : Link connect
				source references a.p to
				target references b.p;
			connection tri connect (a, b, a.inner);
		}
	}`)
	w := nested(t, sym(t, root, "P").Scope, "w")

	for _, name := range []string{"typed", "named"} {
		got := m.ConnectorEndAttachments(nested(t, w.Scope, name))
		if len(got) != 2 {
			t.Fatalf("%s: %d attachments, want two", name, len(got))
		}
		for i, wantEnd := range []string{"source", "target"} {
			if got[i].Name != wantEnd {
				t.Errorf("%s: attachment %d is named %q, want %q", name, i, got[i].Name, wantEnd)
			}
			if got[i].Attachment == nil {
				t.Errorf("%s: attachment %d attaches to nothing", name, i)
			}
		}
	}

	// Every end of an n-ary connector is kept, in declaration order.
	if got := m.ConnectorEndAttachments(nested(t, w.Scope, "tri")); len(got) != 3 {
		t.Fatalf("tri: %d attachments, want three", len(got))
	}
}
