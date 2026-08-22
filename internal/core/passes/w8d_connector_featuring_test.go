package passes

import "testing"

// The reference reports each end that names a feature of a nested feature
// (Connector_Invalid.sysml.xt, BindingConnector_redefine.sysml.xt).
func TestW8DConnectorEndMustBeAccessible(t *testing.T) {
	const src = `package P {
		part def A {
			part x;
		}
		part def B {
			part y {
				part z;
			}
			connect A::x to y::z;
		}
		part b {
			part y : A {
				connect x to z;
			}
			part z;
			connect y::x to z;
		}
	}`
	w8dWantLines(t, src, "connector-type-featuring", 9, 9, 13, 16)
}

func TestW8DBindingEndMustBeAccessible(t *testing.T) {
	const src = `package P {
		port def P0 {
			in ref myDefIn;
		}
		part def B0 {
			port p0 : P0;
		}
		part v {
			part b0 : B0;
			bind B0::p0.myDefIn = b0.p0.myDefIn;
		}
	}`
	w8dWantLines(t, src, "connector-type-featuring", 10)
}

// Ends named with dot notation, and ends of a variant of a variation, are
// accessible, so a legal model stays silent.
func TestW8DAccessibleConnectorEndsStaySilent(t *testing.T) {
	const src = `package P {
		port def PingPort;
		part def A {
			part x;
		}
		part def B {
			port outPort : PingPort;
			port inPort : PingPort;
			port bypass : PingPort;
			variation interface link {
				variant interface direct connect outPort to inPort;
				variant interface indirect connect outPort to bypass;
			}
		}
		part b {
			part y : A;
			part z;
			connect y.x to z;
			bind y.x = z;
		}
	}`
	if diags := only(w8dDiags(t, src), "connector-type-featuring"); len(diags) != 0 {
		t.Fatalf("legal connector model reported: %v", diags)
	}
}
