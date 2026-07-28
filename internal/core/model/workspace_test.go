package model

import "testing"

func TestWorkspaceOpenIndexesDocument(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace N; }"), 1)
	if syms := ws.LookupQualified("P::N"); len(syms) != 1 {
		t.Fatalf("P::N = %d symbols, want 1", len(syms))
	}
}

func TestWorkspaceUpdateReindexesIncrementally(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace Old; }"), 1)
	ws.Update("a.sysml", []byte("package P { namespace New; }"), 2)
	if syms := ws.LookupQualified("P::Old"); len(syms) != 0 {
		t.Fatalf("P::Old = %d, want 0 (stale entry not cleared)", len(syms))
	}
	if syms := ws.LookupQualified("P::New"); len(syms) != 1 {
		t.Fatalf("P::New = %d, want 1", len(syms))
	}
	if syms := ws.LookupQualified("P"); len(syms) != 1 {
		t.Fatalf("P = %d, want 1 (not doubled)", len(syms))
	}
}

func TestWorkspaceCloseKeepsOnDiskContent(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.Open("a.sysml", []byte("package Buf { namespace B; }"), 1)
	if syms := ws.LookupQualified("Buf::B"); len(syms) != 1 {
		t.Fatal("open buffer should be authoritative")
	}
	ws.Close("a.sysml")
	if syms := ws.LookupQualified("Disk::D"); len(syms) != 1 {
		t.Fatal("closing should revert to on-disk content")
	}
	if syms := ws.LookupQualified("Buf::B"); len(syms) != 0 {
		t.Fatal("buffer content should be gone after close")
	}
}

func TestWorkspaceRemoveDropsFromIndex(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P;"), 1)
	ws.Remove("a.sysml")
	if syms := ws.LookupQualified("P"); len(syms) != 0 {
		t.Fatalf("P = %d, want 0 after remove", len(syms))
	}
	if ws.Document("a.sysml") != nil {
		t.Fatal("document should be gone after remove")
	}
}
