package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// A declared subject's value part binds the subject for the check, whether it is
// written `= expr` (a feature value binding) or `default expr` (a fallback);
// either way an externally supplied subject wins.
func TestRequirementSubjectDeclarationValue(t *testing.T) {
	src := `package test {
		requirement Bound {
			subject speed : Real = 50;
			require speed < 100;
		}
		requirement Defaulted {
			subject speed : Real default 150;
			require speed < 100;
		}
	}`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)
	testPkg := idx.DocumentRoot("test.sysml").Children()[0]

	for _, tt := range []struct {
		name    string
		wantErr bool
	}{{"Bound", false}, {"Defaulted", true}} {
		sym, ok := testPkg.LookupLocal(tt.name)
		if !ok {
			t.Fatalf("%s not found", tt.name)
		}
		satisfied, err := ctx.EvaluateRequirement(sym, testPkg)
		switch {
		case tt.wantErr && err == nil:
			t.Errorf("%s: satisfied = %t, want the declared value to violate the condition", tt.name, satisfied)
		case !tt.wantErr && err != nil:
			t.Errorf("%s evaluation failed: %v", tt.name, err)
		case !tt.wantErr && !satisfied:
			t.Errorf("%s should be satisfied", tt.name)
		}
	}
}
