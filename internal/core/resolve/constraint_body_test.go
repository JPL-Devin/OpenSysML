package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
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

// A named constraint a requirement owns is a feature of it, so a specializing
// requirement or a usage may redefine it by name, and may read it by name.
func TestResolveOwnedConstraintRedefinedByName(t *testing.T) {
	src := `package P {
		constraint def C;
		constraint c0 : C;
		constraint c1 : C;
		requirement def R {
			require constraint c : C = c0;
			assume constraint a : C default = c0;
		}
		requirement def S :> R {
			require constraint :>> c = c1;
			assume constraint :>> a = c1;
		}
		requirement r : R { require constraint :>> c = c1; }
		requirement def T :> R { require constraint x :>> c = c1; }
		requirement def U :> R { require constraint :>> nosuch = c1; }
	}`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "nosuch") {
		t.Fatalf("expected one diagnostic, for nosuch, got %v", r.Diagnostics)
	}
	want := strings.Index(src, "require constraint c : C = c0;")
	for _, decl := range []string{
		"require constraint :>> c = c1;\n\t\t\tassume",
		"requirement r : R { require constraint :>> c",
		"require constraint x :>> c",
	} {
		sym, ok := r.PartSymbol(redefinitionTarget(t, r, src, decl), 0)
		if !ok || sym.Decl.Span().Offset != want {
			t.Errorf("%q: `:>> c` resolves to %v, want R's c", decl, sym)
		}
	}
}

// redefinitionTarget finds the qualified name written after the `:>>` in decl,
// among the names r resolved.
func redefinitionTarget(t *testing.T, r *Resolver, src, decl string) *ast.QualifiedName {
	t.Helper()
	at := strings.Index(src, decl)
	if at < 0 {
		t.Fatalf("%q not in source", decl)
	}
	at += strings.Index(decl, ":>> ") + len(":>> ")
	for qn := range r.parts {
		if qn.Span().Offset == at {
			return qn
		}
	}
	t.Fatalf("no resolved qualified name at offset %d", at)
	return nil
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
