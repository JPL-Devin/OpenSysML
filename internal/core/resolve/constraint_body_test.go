package resolve

import (
	"strings"
	"testing"
)

// A typo nested two levels inside a require body is reported, at its own span.
func TestResolveTypoAtDepthTwoInARequireBody(t *testing.T) {
	src := `package P {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		part def Payload { attribute mass; }
		analysis def C {
			objective o {
				require A::r {
					part p : Payload {
						attribute x = nosuchAtTwo;
					}
				}
			}
		}
	}`
	r := resolveDoc(t, "d.sysml", src)
	assertUnresolvedAt(t, r, src, "nosuchAtTwo")
}

// The same at depth three: the walk is recursive, not one level of special case.
func TestResolveTypoAtDepthThreeInAnAssumeBody(t *testing.T) {
	src := `package P {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		part def Payload { attribute mass; part inner : Payload; }
		analysis def C {
			objective o {
				assume A::r {
					part p : Payload {
						part q : Payload {
							attribute y = nosuchAtThree;
						}
					}
				}
			}
		}
	}`
	r := resolveDoc(t, "d.sysml", src)
	assertUnresolvedAt(t, r, src, "nosuchAtThree")
}

// A legal deep body stays clean, reading its own names and those it inherits.
func TestResolveDeepRequireBodyResolves(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package P {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		part def Payload { attribute mass; part inner : Payload; }
		analysis def C {
			attribute budget;
			objective o {
				require A::r {
					:>> mass = budget;
					part p : Payload {
						:>> mass = budget;
						part q :>> inner {
							attribute local;
							attribute reads = local;
							attribute reachesOut = budget;
						}
					}
				}
			}
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

// The body is a scope of its own: what it declares is unresolved outside it.
func TestResolveRequireBodyNamesDoNotLeakOutward(t *testing.T) {
	src := `package P {
		requirement def R { attribute mass; }
		package A { requirement r : R; }
		part def Payload { attribute mass; }
		analysis def C {
			objective o {
				require A::r {
					part bodyLocal : Payload;
				}
			}
			attribute reads = bodyLocal;
		}
	}`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the leaked name, got %v", r.Diagnostics)
	}
	d := r.Diagnostics[0]
	if !strings.Contains(d.Message, "bodyLocal") {
		t.Errorf("diagnostic %q does not name bodyLocal", d.Message)
	}
	// The read outside the body is reported, not the declaration inside it.
	if want := strings.LastIndex(src, "bodyLocal"); d.Span.Offset != want {
		t.Errorf("bodyLocal reported at offset %d, want %d", d.Span.Offset, want)
	}
}

// A nested constraint body declares parameters and features of its own, read by
// its statements and conditions and by the nested constraints it owns.
func TestResolveNestedConstraintBodyDeclarations(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package P {
		attribute limit;
		requirement def R {
			require constraint {
				in attribute x;
				attribute y = x + 1.0;
				if y > limit { action a; action b; fork f; first a then f; first f then b; }
				y < limit
			}
			assume constraint c {
				in attribute z;
				assert constraint inner { in attribute w; w > z }
				z > 0.0
			}
		}
		constraint def C {
			assert constraint { in attribute v; v > 0.0 }
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

// What a nested `assert constraint { … }` body declares is its own: unresolved
// from the constraint around it.
func TestResolveNestedAssertBodyNamesDoNotLeakOutward(t *testing.T) {
	src := `package P {
		constraint def C {
			assert constraint { in attribute bodyOnly; bodyOnly > 0.0 }
			bodyOnly > 0.0
		}
	}`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the leaked name, got %v", r.Diagnostics)
	}
	d := r.Diagnostics[0]
	if want := strings.LastIndex(src, "bodyOnly"); d.Span.Offset != want {
		t.Errorf("bodyOnly reported at offset %d, want %d", d.Span.Offset, want)
	}
}

// assertUnresolvedAt asserts one unresolved diagnostic spanning name in src.
func assertUnresolvedAt(t *testing.T, r *Resolver, src, name string) {
	t.Helper()
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for %s, got %v", name, r.Diagnostics)
	}
	d := r.Diagnostics[0]
	name = strings.SplitN(name, ";", 2)[0]
	if !strings.Contains(d.Message, name) {
		t.Errorf("diagnostic %q does not name %s", d.Message, name)
	}
	want := strings.Index(src, name)
	if got := d.Span.Offset; got != want {
		t.Errorf("%s reported at offset %d, want %d", name, got, want)
	}
	if got := d.Span.Len; got != len(name) {
		t.Errorf("%s reported with length %d, want %d", name, got, len(name))
	}
}
