package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// metadataDiags analyzes src as KerML against the standard library and returns
// the findings about metadata annotations, with the source text each covers.
func metadataDiags(t *testing.T, src string) []metadataFinding {
	t.Helper()

	idx := symbols.NewIndex()
	libSrc := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		cache = nil
	}
	if err := libs.NewLoader(libSrc, cache).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
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
	if found[0].Text != "~3" {
		t.Errorf("reported %q, want the value expression `~3`", found[0].Text)
	}
	if found[0].Msg != msgFilterNotEvaluable {
		t.Errorf("message %q, want %q", found[0].Msg, msgFilterNotEvaluable)
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
