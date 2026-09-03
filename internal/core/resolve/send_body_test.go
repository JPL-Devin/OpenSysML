package resolve

import "testing"

// A typo in the body of a send is reported wherever the send is written: on an
// action node, bare in a definition, in a branch, or in a succession's body.
func TestResolveTypoInASendBody(t *testing.T) {
	cases := map[string]string{
		"action node": `package P {
			part def R;
			action def A { part r : R; action s send to r { in x = nosuchInSend; } }
		}`,
		"bare in a definition": `package P {
			part def R;
			action def A { part r : R; send to r { in x = nosuchInSend; } }
		}`,
		"in a branch": `package P {
			part def R;
			action def A { part r : R; if true { send to r { in x = nosuchInSend; } } }
		}`,
		"in a succession body": `package P {
			part def R;
			action def A { part r : R; action a; action b; then b { send to r { in x = nosuchInSend; } } }
		}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			assertUnresolvedAt(t, resolveDoc(t, "d.sysml", src), src, "nosuchInSend")
		})
	}
}

// A send body reads the names around it and the parameters it declares.
func TestResolveSendBodyReadsEnclosingNames(t *testing.T) {
	src := `package P {
		part def R;
		item def Msg;
		action def A {
			part r : R;
			item m : Msg;
			action s send to r { in x = m; }
			send to r { in y = m; in z = y; }
		}
	}`
	if r := resolveDoc(t, "d.sysml", src); len(r.Diagnostics) != 0 {
		t.Fatalf("expected a clean resolve, got %v", r.Diagnostics)
	}
}
