package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// The requirement named by `require Q::r` is a reference like any other, so the
// editor must jump to its declaration.
func TestDefinitionQualifiedRequireReference(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_require.sysml").Filename()
	src := `package P {
	requirement def FuelMass {
		requirement fuelMassRequirement;
	}
	analysis def A {
		objective o {
			require FuelMass::fuelMassRequirement { }
		}
	}
}`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "fuelMassRequirement")
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	// The declaration is on line 2 (0-based).
	if locs[0].Range.Start.Line != 2 {
		t.Errorf("decl line = %d, want 2", locs[0].Range.Start.Line)
	}
}
