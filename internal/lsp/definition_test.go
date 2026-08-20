package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

func TestDefinitionJumpsToDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Absolute name so uri.File(name).Filename() round-trips back to name.
	name := uri.File("/tmp/def.sysml").Filename()
	src := "package P { namespace N; }\nimport P::N;"
	ws.Open(name, []byte(src), 1)

	// Cursor on the "N" inside "import P::N;".
	off := strings.LastIndex(src, "N")
	pos := offsetToPosition([]byte(src), off)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].URI != uri.File(name) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(name))
	}
	// Declaration "N" is on line 0.
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("decl line = %d, want 0", locs[0].Range.Start.Line)
	}
}

// Go-to-definition on a body-expression parameter must land on the parameter's
// own identifier, not on the body's brace or the same-named outer feature; on
// the declaration itself there is nothing to jump to.
func TestDefinitionBodyExpressionParameter(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_bodyparam.sysml").Filename()
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	attribute s : Integer = 1;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
	}
}
`
	ws.Open(name, []byte(src), 1)

	definitionAt := func(anchor string) []protocol.Location {
		t.Helper()
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), strings.Index(src, anchor)),
			},
		})
		if err != nil {
			t.Fatalf("Definition err = %v", err)
		}
		return locs
	}

	locs := definitionAt("s > 0")
	if len(locs) != 1 {
		t.Fatalf("locations from use = %d, want 1", len(locs))
	}
	if got, want := positionToOffset([]byte(src), locs[0].Range.Start), strings.Index(src, "s : Real; s > 0"); got != want {
		t.Errorf("definition offset = %d, want %d (the parameter's own name)", got, want)
	}

	if locs := definitionAt("s : Real; s > 0"); len(locs) != 0 {
		t.Errorf("definition on the declaration returned %d locations, want 0", len(locs))
	}
}

func TestDefinitionCrossFile(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Two docs under distinct absolute round-trip names.
	libName := uri.File("/tmp/lib.sysml").Filename()
	useName := uri.File("/tmp/use.sysml").Filename()
	ws.Open(libName, []byte("package P { namespace N; }"), 1)
	useSrc := "import P::N;"
	ws.Open(useName, []byte(useSrc), 1)

	// Cursor on the "N" inside "import P::N;" in the using doc.
	off := strings.LastIndex(useSrc, "N")
	pos := offsetToPosition([]byte(useSrc), off)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(useName)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	// The declaring file is libName, not the requesting useName.
	if locs[0].URI != uri.File(libName) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(libName))
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("decl line = %d, want 0", locs[0].Range.Start.Line)
	}
}

// The name in a perform statement names the action performed, not the perform
// statement — which now carries that name too (symbols.effectiveIdent).
func TestDefinitionPerformReference(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_perform.sysml").Filename()
	src := `package P {
	action providePower;
	part vehicle {
		perform providePower;
	}
}
`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "providePower")
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
	// `action providePower;` is on line 1; the perform statement is on line 3.
	if locs[0].Range.Start.Line != 1 {
		t.Errorf("decl line = %d, want 1 (the action, not the perform statement)", locs[0].Range.Start.Line)
	}
}

// The last segment of `perform providePower.generateTorque;` names a member of
// the referenced action, not of the enclosing part — where the perform statement
// binds that very name.
func TestDefinitionPerformChainMember(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_perform_chain.sysml").Filename()
	src := `package P {
	action providePower {
		action generateTorque;
	}
	part torqueGenerator {
		perform providePower.generateTorque;
	}
}
`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "generateTorque")
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
	// `action generateTorque;` is on line 2; the perform statement is on line 5.
	if locs[0].Range.Start.Line != 2 {
		t.Errorf("decl line = %d, want 2 (the action's member, not the perform statement)", locs[0].Range.Start.Line)
	}
}

// The feature a connector end attaches to is a feature of the connector's
// owner, so a name it shares with a sibling end names the owner's feature.
func TestDefinitionConnectorEndReferenceIsNotASiblingEnd(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_end_sibling.sysml").Filename()
	src := `package P {
	part def TireBead;
	connection def PressureSeat {
		end [1] part bead : TireBead;
		end [1] part rim : TireBead;
	}
	part wheelAssy {
		part outer : TireBead;
		part bead : TireBead;
		connection : PressureSeat connect
			bead references outer to
			rim references bead;
	}
}
`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "references bead") + len("references ")
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
	// `part bead : TireBead;` is on line 8; the sibling end is on line 10.
	if locs[0].Range.Start.Line != 8 {
		t.Errorf("decl line = %d, want 8 (the part, not the sibling end)", locs[0].Range.Start.Line)
	}
}

// A name in a filter condition resolves through the imports of its own
// namespace, which the namespace's filters must not restrict — the diagnostics
// pass resolves it that way, and the editor has to agree or rename skips it.
func TestDefinitionMetadataTypeInsideAFilterCondition(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_filter.sysml").Filename()
	src := `package Lib {
	metadata def Safety;
	metadata def Other;
	part seatBelt;
}
package P {
	private import Lib::*[@Other];
	filter @Safety;
}`
	ws.Open(name, []byte(src), 1)
	for _, d := range ws.Diagnostics(name) {
		t.Fatalf("the source reports %q", d.Message)
	}

	// Line 1 declares Safety, line 2 Other; each is named again in P.
	for _, tc := range []struct {
		name string
		decl uint32
	}{{"Other", 2}, {"Safety", 1}} {
		pos := offsetToPosition([]byte(src), strings.LastIndex(src, tc.name))
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     pos,
			},
		})
		if err != nil {
			t.Fatalf("Definition(%s) err = %v", tc.name, err)
		}
		if len(locs) != 1 || locs[0].Range.Start.Line != tc.decl {
			t.Errorf("Definition(%s) = %+v, want its declaration on line %d", tc.name, locs, tc.decl)
		}
	}
}
