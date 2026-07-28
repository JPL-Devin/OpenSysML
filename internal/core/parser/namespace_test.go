package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

var _ = ast.Node(nil)

func TestParseQualifiedNameSimple(t *testing.T) {
	p := newParser("A::B::C")
	qn := p.parseQualifiedName()
	if qn == nil {
		t.Fatal("got nil qualified name")
	}
	if qn.Global {
		t.Error("expected not global")
	}
	if len(qn.Parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(qn.Parts))
	}
	if qn.Parts[0].Text != "A" || qn.Parts[1].Text != "B" || qn.Parts[2].Text != "C" {
		t.Errorf("parts = %q/%q/%q", qn.Parts[0].Text, qn.Parts[1].Text, qn.Parts[2].Text)
	}
	if len(p.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want 0", len(p.Diagnostics))
	}
}

func TestParseQualifiedNameGlobal(t *testing.T) {
	p := newParser("$::Root::X")
	qn := p.parseQualifiedName()
	if qn == nil {
		t.Fatal("got nil qualified name")
	}
	if !qn.Global {
		t.Error("expected global")
	}
	if len(qn.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(qn.Parts))
	}
	if qn.Parts[0].Text != "Root" || qn.Parts[1].Text != "X" {
		t.Errorf("parts = %q/%q", qn.Parts[0].Text, qn.Parts[1].Text)
	}
}

func TestParseQualifiedNameUnrestricted(t *testing.T) {
	p := newParser("'my name'::B")
	qn := p.parseQualifiedName()
	if qn == nil {
		t.Fatal("got nil qualified name")
	}
	if len(qn.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(qn.Parts))
	}
	if qn.Parts[0].Text != "'my name'" {
		t.Errorf("part0 = %q, want quotes kept", qn.Parts[0].Text)
	}
}

func TestParseQualifiedNameNoName(t *testing.T) {
	p := newParser(";")
	qn := p.parseQualifiedName()
	if qn != nil {
		t.Errorf("expected nil, got %+v", qn)
	}
	if len(p.Diagnostics) != 1 {
		t.Errorf("got %d diagnostics, want 1", len(p.Diagnostics))
	}
}

func TestParseIdentificationShortAndName(t *testing.T) {
	p := newParser("<v1> Vehicle")
	id := p.parseIdentification()
	if id.ShortName != "v1" {
		t.Errorf("ShortName = %q, want v1", id.ShortName)
	}
	if id.Name != "Vehicle" {
		t.Errorf("Name = %q, want Vehicle", id.Name)
	}
}

func TestParseIdentificationNameOnly(t *testing.T) {
	p := newParser("Vehicle")
	id := p.parseIdentification()
	if id.ShortName != "" {
		t.Errorf("ShortName = %q, want empty", id.ShortName)
	}
	if id.Name != "Vehicle" {
		t.Errorf("Name = %q, want Vehicle", id.Name)
	}
}

func TestParseIdentificationEmpty(t *testing.T) {
	p := newParser("{")
	id := p.parseIdentification()
	if id.Name != "" || id.ShortName != "" {
		t.Errorf("expected empty id, got %+v", id)
	}
}
