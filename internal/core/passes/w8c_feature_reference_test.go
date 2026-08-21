package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w8cLibraryDiagnostics analyzes src as the named document against the standard
// library.
func w8cLibraryDiagnostics(t *testing.T, name, src string) []Diagnostic {
	t.Helper()

	idx := symbols.NewIndex()
	libSrc := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		cache = nil
	}
	loader := libs.NewLoader(libSrc, cache)
	for _, lib := range libSrc.List() {
		if err := loader.Load(lib, idx); err != nil {
			t.Fatalf("load library %s: %v", lib, err)
		}
	}
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	return Analyze(name, root, nil, idx)
}

func w8cLibraryMessagesIn(t *testing.T, name, src string) []string {
	t.Helper()
	var out []string
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		out = append(out, d.Message)
	}
	return out
}

func TestW8CFeatureReferenceAccessibleAndValid(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V {
		feature n : Integer;
		struct S;
	}
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1::n;
	feature v1_S : Integer = v1.S;
	feature v1_V : Integer = V;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if got := w8cCount(msgs, msgSubsettingFeaturingTypes); got != 1 {
		t.Errorf("want one %q, got %v", msgSubsettingFeaturingTypes, msgs)
	}
	if got := w8cCount(msgs, msgReferentIsFeature); got != 2 {
		t.Errorf("want two %q, got %v", msgReferentIsFeature, msgs)
	}
}

func TestW8CFeatureReferenceLocation(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V { feature n : Integer; }
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1::n;
}`
	var spans []string
	for _, d := range w8cLibraryDiagnostics(t, "<t>.kerml", src) {
		if d.Message == msgSubsettingFeaturingTypes {
			spans = append(spans, strings.TrimSpace(src[d.Span.Offset:d.Span.End()]))
		}
	}
	if len(spans) != 1 || spans[0] != "v1::n" {
		t.Errorf("want the diagnostic at \"v1::n\", got %v", spans)
	}
}

func TestW8CFeatureReferenceEnumerationLiteralLegal(t *testing.T) {
	src := `package P {
	enum def Color { enum red; enum green; }
	attribute c : Color = Color::red;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.sysml", src)
	if got := w8cCount(msgs, msgSubsettingFeaturingTypes); got != 0 {
		t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
	}
}

func TestW8CFeatureReferenceLegal(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct V { feature n : Integer; }
	feature v1 : V { feature redefines n; }
	feature v1_n : Integer = v1.n;
	feature m : Integer;
	feature m2 : Integer = m;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if got := w8cCount(msgs, msgSubsettingFeaturingTypes); got != 0 {
		t.Errorf("unexpected %q: %v", msgSubsettingFeaturingTypes, msgs)
	}
	if got := w8cCount(msgs, msgReferentIsFeature); got != 0 {
		t.Errorf("unexpected %q: %v", msgReferentIsFeature, msgs)
	}
}
