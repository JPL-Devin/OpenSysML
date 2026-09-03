package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// metadataDiags analyzes src as KerML against the standard library and returns
// the findings about metadata annotations, with the source text each covers.
func metadataDiags(t *testing.T, src string) []metadataFinding {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx.AddDocument("<t>.kerml", root)
	idx.ExpandWildcardImports()

	var out []metadataFinding
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		out = append(out, metadataFinding{
			Code: d.Code,
			Text: src[d.Span.Offset:d.Span.End()],
			Msg:  d.Message,
		})
	}
	return out
}

type metadataFinding struct {
	Code string
	Text string
	Msg  string
}

// findingsWithCode returns the findings carrying one code.
func findingsWithCode(found []metadataFinding, code string) []metadataFinding {
	var out []metadataFinding
	for _, f := range found {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

const metadataEvaluableSrc = `package P {
	metadata def A {
		feature x;
		feature y;
	}
	feature f { in p; }
	feature a {
		@A {
			x = ~3;
			y = 1 + 2;
		}
	}
}`

// A metadata feature is a model-level element, so a value it binds must be one
// the model alone decides (KerML §7.4.9).
func TestMetadataBodyValueMustBeModelLevelEvaluable(t *testing.T) {
	found := findingsWithCode(metadataDiags(t, metadataEvaluableSrc), "metadata-value-not-evaluable")
	if len(found) != 1 {
		t.Fatalf("want one inevaluable value, got %v", found)
	}
	if found[0].Text != "= ~3" {
		t.Errorf("reported %q, want the binding and value `= ~3`", found[0].Text)
	}
	if found[0].Msg != msgFilterNotEvaluable {
		t.Errorf("message %q, want %q", found[0].Msg, msgFilterNotEvaluable)
	}
}

func TestNestedMetadataBodyValueUsesBindingSpan(t *testing.T) {
	src := `package P {
		metadata def A {
			feature x { feature y; feature z; }
		}
		feature f { in p; }
		feature a {
			@A {
				x {
					y = ~3;
					z = 1 + 2;
				}
			}
		}
	}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	if len(found) != 1 || found[0].Text != "= ~3" {
		t.Fatalf("want one nested finding on `= ~3`, got %v", found)
	}
}

// A declaration in an annotation body restates a feature of the metadata type;
// one that restates nothing is reported where it is written.
func TestMetadataBodyFeatureMustRestateAnOwningTypeFeature(t *testing.T) {
	src := `package P {
	metadata def A { feature x; }
	feature a {
		@A {
			x = 1;
			bad;
		}
	}
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-owning-type-feature")
	if len(found) != 1 || strings.TrimSpace(found[0].Text) != "bad;" {
		t.Fatalf("want one finding on `bad;`, got %v", found)
	}
}

// The metaclass of an annotated element conforms to every type of the
// annotatedElement feature of the metadata type (KerML §8.3.4.9).
func TestMetadataAnnotatedElementMustConform(t *testing.T) {
	src := `package P {
	metaclass Only {
		:>> annotatedElement : KerML::Structure;
	}
	#Only struct Ok;
	#Only class Bad;
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-annotated-element")
	if len(found) != 1 {
		t.Fatalf("want one annotated-element finding, got %v", found)
	}
	if want := msgCannotAnnotate + "Class"; found[0].Msg != want {
		t.Errorf("message %q, want %q", found[0].Msg, want)
	}
}

// A metadata value calling an overloaded name is judged by the overload its
// arguments select — the checker's and runtime's choice — not the first found.
func TestMetadataBodyValueSelectsTheOverloadItCalls(t *testing.T) {
	const body = `package P {
	private import ScalarValues::*;
	private import %s::*;
	private import %s::*;
	package Q {
		private import ScalarValues::*;
		function 'if' { in test : Boolean; in t : String; in f : String; return : String; }
	}
	metadata def A { feature x; feature y; }
	feature a {
		@A {
			x = 'if'(true, 1, 2);
			y = 'if'(true, "a", "b");
		}
	}
}`
	for _, imports := range [][2]string{{"ControlFunctions", "Q"}, {"Q", "ControlFunctions"}} {
		src := fmt.Sprintf(body, imports[0], imports[1])
		found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
		if len(found) != 1 || found[0].Text != `= 'if'(true, "a", "b")` {
			t.Errorf("imports %v: findings %v, want only the call selecting Q::'if'", imports, found)
		}
	}
}

// A body value is judged in the body's own scope, where the metadata type's
// members shadow what the annotated element sees: the call and the read below
// name A's own function and feature, not the imported one and the evaluable one.
func TestMetadataBodyValueIsJudgedInTheBodyScope(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	feature k = 2;
	metadata def A {
		feature x;
		feature y;
		feature k;
		function 'if' { in test : Boolean; in t : Integer; in f : Integer; return : Integer; }
	}
	feature a {
		@A {
			x = 'if'(true, 1, 2);
			y = k;
		}
	}
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	want := []string{"= 'if'(true, 1, 2)", "= k"}
	if len(found) != len(want) {
		t.Fatalf("findings %v, want %v", found, want)
	}
	for i, f := range found {
		if f.Text != want[i] {
			t.Errorf("finding %d is %q, want %q", i, f.Text, want[i])
		}
	}
}

// A body with no fault draws nothing: a value the model folds and a feature that
// restates one of the metadata type are both legal.
func TestMetadataBodyWithoutFaultsIsSilent(t *testing.T) {
	src := `package P {
	metadata def A { feature x; }
	feature a {
		@A { x = 1 + 2; }
	}
}`
	for _, f := range metadataDiags(t, src) {
		switch f.Code {
		case "metadata-value-not-evaluable", "metadata-owning-type-feature", "metadata-annotated-element":
			t.Errorf("unexpected finding %v", f)
		}
	}
}
