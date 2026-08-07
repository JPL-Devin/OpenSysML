package semantics

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
