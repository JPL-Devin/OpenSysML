package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

// bodyRefSource uses `speed` from every expression position a behavioral body
// offers. The declaration plus 13 uses means rename must produce 14 edits and
// references must report 13.
const bodyRefSource = `package P {
	import ScalarValues::*;
	attribute speed : Integer = 0;
	attribute bare : Integer = speed;
	attribute paren : Integer = (speed);
	attribute operand : Integer = speed + 1;
	calc plain { return r = speed; }
	calc def Plain { return r = speed; }
	constraint inRange { speed > 0 }
	requirement Req {
		assume speed > 0;
		require speed < 100;
	}
	action drive {
		attribute local : Integer = 0;
		assign local := speed;
	}
	state Machine {
		attribute seen : Integer = 0;
		initial i;
		state running {
			entry { assign seen := speed; }
			do { assign seen := speed; }
			exit { assign seen := speed; }
		}
		state stopped;
		i then running;
		transition running to stopped if speed > 90;
	}
}
`

const wantBodyRefUses = 13

func TestRenameEditsBareReferencesInBehaviorBodies(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/walk_rename.sysml", bodyRefSource)

	edits := renameEdits(t, ws, name, "speed :", "velocity")
	if len(edits) != wantBodyRefUses+1 {
		t.Fatalf("Rename produced %d edit(s), want %d (declaration + %d uses)",
			len(edits), wantBodyRefUses+1, wantBodyRefUses)
	}

	got, err := applyRename(t, ws, name, "speed :", "velocity")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if strings.Contains(got[name], "speed") {
		t.Errorf("speed still referenced after rename:\n%s", got[name])
	}
}

func TestReferencesFindsBareReferencesInBehaviorBodies(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/walk_refs.sysml", bodyRefSource)

	s := NewServer(ws)
	doc := ws.Document(name)
	off := strings.Index(string(doc.Content), "speed :")
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(doc.Content, off),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != wantBodyRefUses {
		t.Fatalf("References found %d use(s), want %d:\n%v", len(locs), wantBodyRefUses, locs)
	}
}

// A bare trigger name is a signal injected by the event source, not a reference
// to the like-named declaration, so renaming that declaration must leave it be.
func TestRenameLeavesSignalTriggerNames(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	import ScalarValues::*;
	attribute go : Integer = 0;
	state Machine {
		initial i;
		state a;
		state b;
		i then a;
		transition a to b when go;
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_trigger.sysml", src)

	got, err := applyRename(t, ws, name, "go :", "start")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "attribute start : Integer = 0;") {
		t.Errorf("declaration not renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "when go;") {
		t.Errorf("signal trigger name was rewritten:\n%s", got[name])
	}
}

// A body expression's parameter is its own declaration, so renaming a
// same-named outer feature must not rewrite the parameter's uses inside the
// body, while a name the body only reads from outside is still rewritten.
func TestRenameLeavesBodyExpressionParameters(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	import ScalarValues::*;
	import ControlFunctions::*;
	attribute s : Integer = 1;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
		assert constraint { samples->forAll { in x : Real; x > s } }
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_bodyexpr.sysml", src)

	got, err := applyRename(t, ws, name, "s : Integer", "threshold")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "attribute threshold : Integer = 1;") {
		t.Errorf("declaration not renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "in s : Real; s > 0") {
		t.Errorf("body-expression parameter was rewritten:\n%s", got[name])
	}
	if !strings.Contains(got[name], "x > threshold") {
		t.Errorf("reference to the outer feature was not renamed:\n%s", got[name])
	}
}

// Renaming from a use of a body-expression parameter must edit the parameter's
// own declaration, not the body's opening brace, and must leave the same-named
// outer feature alone.
func TestRenameBodyExpressionParameterFromUse(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	import ScalarValues::*;
	import ControlFunctions::*;
	attribute s : Integer = 1;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_bodyparam.sysml", src)

	got, err := applyRename(t, ws, name, "s > 0", "sample")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "in sample : Real; sample > 0") {
		t.Errorf("parameter declaration and use not both renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "attribute s : Integer = 1;") {
		t.Errorf("outer feature was rewritten:\n%s", got[name])
	}
}
