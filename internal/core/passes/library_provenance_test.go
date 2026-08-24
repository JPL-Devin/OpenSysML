package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// constraintDiagsOverLibrary analyses src in an index where libSrc is marked as
// library content, which is how a bundled definition reaches a model.
func constraintDiagsOverLibrary(t *testing.T, libSrc, src string) []Diagnostic {
	t.Helper()
	idx := newTestIndex()
	idx.AddDocument("frame.sysml", parser.New(source.New("frame.sysml", []byte(libSrc))).ParseFile())
	idx.MarkLibrary("frame.sysml")
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()

	var out []Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == "constraint" {
			out = append(out, d)
		}
	}
	return out
}

// A library objective is the frame a case objective redefines, so a case owning
// one objective is silent even though it inherits the library's.
func TestW7GLibraryObjectiveIsNotACompetingObjective(t *testing.T) {
	const frame = `package Frame {
		case def FramedCase {
			objective frameObj;
		}
	}`
	diags := only(constraintDiagsOverLibrary(t, frame, `package C {
		case def Analysis :> Frame::FramedCase {
			objective own;
		}
	}`), "only-one-objective")
	if len(diags) != 0 {
		t.Fatalf("a library objective competed with the model's own, got %v", diags)
	}
}

// A model's own inherited objective still competes: provenance decides, not
// inheritance.
func TestW7GModelObjectiveStillCompetesWithAnInheritedOne(t *testing.T) {
	const frame = `package Frame {
		case def FramedCase {
			objective frameObj;
		}
	}`
	diags := only(constraintDiagsOverLibrary(t, frame, `package C {
		case def Base {
			objective inheritedObj;
		}
		case def Analysis :> Base {
			objective own;
		}
	}`), "only-one-objective")
	if len(diags) != 1 {
		t.Fatalf("expected the model's own objective to be diagnosed once, got %v", diags)
	}
}
