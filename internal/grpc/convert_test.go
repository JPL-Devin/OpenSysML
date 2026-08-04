package grpc

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TestSymbolToProto verifies exported SymbolToProto API.
func TestSymbolToProto(t *testing.T) {
	sym := &symbols.Symbol{
		Name:       "TestPart",
		Kind:       symbols.SymbolPartDef,
		Visibility: ast.VisibilityPublic,
		DeclSpan:   source.Span{Offset: 10, Len: 10},
	}

	idx := symbols.NewIndex()

	proto := SymbolToProto(sym, idx)

	if proto.Name != "TestPart" {
		t.Errorf("expected name TestPart, got %s", proto.Name)
	}
	if proto.Kind != "partDef" {
		t.Errorf("expected kind partDef, got %s", proto.Kind)
	}
	if proto.Id == "" {
		t.Error("expected non-empty ID")
	}
	if proto.Metadata["visibility"] != "public" {
		t.Errorf("expected visibility public, got %s", proto.Metadata["visibility"])
	}
}

// TestDiagnosticToProto verifies DiagnosticToProto conversion.
func TestDiagnosticToProto(t *testing.T) {
	diag := passes.Diagnostic{
		Severity: passes.SeverityError,
		Message:  "test error",
		Span:     source.Span{Offset: 5, Len: 4}, // "Test" at position 5
	}

	sf := source.New("test.sysml", []byte("part Test { }"))
	proto := DiagnosticToProto(diag, sf)

	if proto.Severity != "error" {
		t.Errorf("expected severity error, got %s", proto.Severity)
	}
	if proto.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", proto.Message)
	}
	if proto.Span.File != "test.sysml" {
		t.Error("expected file test.sysml")
	}
	if proto.Span.StartLine != 1 {
		t.Errorf("expected StartLine 1, got %d", proto.Span.StartLine)
	}
}

// TestConvertSpan verifies source.Span → proto.Span conversion.
func TestConvertSpan(t *testing.T) {
	// Create a simple source file
	content := []byte("line1\nline2\nline3")
	sf := source.New("test.sysml", content)
	li := sf.Lines()

	// Span covering "line2" (bytes 6-11)
	sp := source.Span{Offset: 6, Len: 5}

	pb := convertSpan(sp, sf, li)
	if pb.File != "test.sysml" {
		t.Errorf("File: got %q, want %q", pb.File, "test.sysml")
	}
	if pb.StartLine != 2 {
		t.Errorf("StartLine: got %d, want 2", pb.StartLine)
	}
	if pb.EndLine != 2 {
		t.Errorf("EndLine: got %d, want 2", pb.EndLine)
	}
	// Columns are 1-based byte columns
	if pb.StartCol != 1 {
		t.Errorf("StartCol: got %d, want 1", pb.StartCol)
	}
	if pb.EndCol != 6 {
		t.Errorf("EndCol: got %d, want 6", pb.EndCol)
	}
}

// TestConvertSymbolBasic verifies Symbol → SymbolInfo conversion for a simple part def.
func TestConvertSymbolBasic(t *testing.T) {
	// Mock symbol
	sym := &symbols.Symbol{
		Name:     "MyPart",
		Kind:     symbols.SymbolPartDef,
		DeclSpan: source.Span{Offset: 0, Len: 10},
	}
	sf := source.New("test.sysml", []byte("part MyPart{}"))
	li := sf.Lines()

	pbSym := convertSymbol(sym, sf, li)
	if pbSym.Id != "MyPart" {
		t.Errorf("Id: got %q, want %q", pbSym.Id, "MyPart")
	}
	if pbSym.Name != "MyPart" {
		t.Errorf("Name: got %q, want %q", pbSym.Name, "MyPart")
	}
	if pbSym.Kind != "partDef" {
		t.Errorf("Kind: got %q, want %q", pbSym.Kind, "partDef")
	}
}

// TestConvertSymbolWithChildren verifies child_ids population.
func TestConvertSymbolWithChildren(t *testing.T) {
	// Parent with a scope containing two children
	parent := &symbols.Symbol{
		Name:     "Parent",
		Kind:     symbols.SymbolPackage,
		DeclSpan: source.Span{Offset: 0, Len: 5},
	}
	child1 := &symbols.Symbol{
		Name:     "Parent::Child1",
		Kind:     symbols.SymbolPartDef,
		DeclSpan: source.Span{Offset: 10, Len: 5},
	}
	child2 := &symbols.Symbol{
		Name:     "Parent::Child2",
		Kind:     symbols.SymbolAttributeDef,
		DeclSpan: source.Span{Offset: 20, Len: 5},
	}
	scope := symbols.NewScope(nil, nil)
	scope.Define("Child1", child1)
	scope.Define("Child2", child2)
	parent.Scope = scope

	sf := source.New("test.sysml", []byte("package Parent { part Child1; attribute Child2; }"))
	li := sf.Lines()

	pbSym := convertSymbol(parent, sf, li)
	if len(pbSym.ChildIds) != 2 {
		t.Fatalf("ChildIds: got %d, want 2", len(pbSym.ChildIds))
	}
	// Order not guaranteed, check both are present
	found := map[string]bool{}
	for _, id := range pbSym.ChildIds {
		found[id] = true
	}
	if !found["Parent::Child1"] {
		t.Errorf("Missing Parent::Child1 in ChildIds")
	}
	if !found["Parent::Child2"] {
		t.Errorf("Missing Parent::Child2 in ChildIds")
	}
}

// TestConvertSymbolMetadata verifies metadata extraction from Usage node.
func TestConvertSymbolMetadata(t *testing.T) {
	// Usage with multiplicity and typing
	usage := &ast.Usage{
		Kind: ast.UsageAttribute,
		Ident: ast.Identification{
			Name: "mass",
		},
		Multiplicity: &ast.Multiplicity{
			Lower:   &ast.LiteralInteger{Value: "1"},
			Upper:   &ast.LiteralInteger{Value: "1"},
			IsRange: true,
		},
		Relationships: []*ast.Relationship{
			{
				Kind: ast.RelTyping,
				Target: &ast.QualifiedName{
					Parts: []ast.NameSegment{{Text: "Real"}},
				},
			},
		},
	}
	sym := &symbols.Symbol{
		Name:     "MyPart::mass",
		Kind:     symbols.SymbolAttributeUsage,
		Decl:     usage,
		DeclSpan: source.Span{Offset: 0, Len: 10},
	}
	sf := source.New("test.sysml", []byte("attribute mass : Real [1];"))
	li := sf.Lines()

	pbSym := convertSymbol(sym, sf, li)

	// Check metadata
	if pbSym.Metadata["multiplicity"] != "1..1" {
		t.Errorf("Metadata[multiplicity]: got %q, want %q", pbSym.Metadata["multiplicity"], "1..1")
	}
	if pbSym.Metadata["type"] != "Real" {
		t.Errorf("Metadata[type]: got %q, want %q", pbSym.Metadata["type"], "Real")
	}
}
