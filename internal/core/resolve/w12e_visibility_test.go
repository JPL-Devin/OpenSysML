package resolve

import "testing"

// A connector end's participant is a feature of the type featuring the
// connector, so the connector's own features are not participants
// (KerML 8.3.4.5).
func TestConnectorEndDoesNotReferenceAFeatureOfTheConnector(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		feature x;
		connector c {
			feature f;
			end a ::> x;
			end b ::> f;
		}
	}`)
	wantUnresolved(t, r, "f")
	wantNoUnresolved(t, r, "x")
}

// An end outside a connector relates nothing, so its reference subsetting sees
// the enclosing declaration's features as any other reference does.
func TestEndOutsideAConnectorReferencesItsOwnersFeature(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		feature Pattern {
			feature source;
			end s ::> source;
		}
	}`)
	wantNoUnresolved(t, r, "source")
}

// A feature with no declared name takes the name of the feature it redefines,
// so it binds no name when that redefinition resolves to nothing, and a later
// redefinition of the same name does not find it (KerML 7.3.4.5).
func TestUnnamedRedefinitionOfAnInvisibleFeatureBindsNoName(t *testing.T) {
	r := resolveVisibilityDoc(t, `package test {
		feature x {
			feature a;
			private feature c;
		}
		feature y subsets x {
			feature redefines a;
			feature redefines c;
		}
		feature redefines x.c;
	}`)
	wantUnresolved(t, r, "c")
	wantNoUnresolved(t, r, "a")
}
