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

func TestWorkspaceDeleteOnDiskKeepsOpenBuffer(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.Open("a.sysml", []byte("package Buf { namespace B; }"), 1)
	ws.DeleteOnDisk("a.sysml")
	if syms := ws.LookupQualified("Buf::B"); len(syms) != 1 {
		t.Fatal("open buffer should survive the file's deletion")
	}
	ws.Close("a.sysml")
	if ws.Document("a.sysml") != nil {
		t.Fatal("closing a deleted file should drop the document")
	}
}

func TestWorkspaceDeleteOnDiskRemovesClosedDocument(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.DeleteOnDisk("a.sysml")
	if syms := ws.LookupQualified("Disk::D"); len(syms) != 0 {
		t.Fatalf("Disk::D = %d, want 0 after the file was deleted", len(syms))
	}
	if ws.Document("a.sysml") != nil {
		t.Fatal("document should be gone after the file was deleted")
	}
}

func TestWorkspaceTracksOpenNames(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("disk.sysml", []byte("package D;"))
	ws.Open("open.sysml", []byte("package O;"), 1)
	if !ws.IsOpen("open.sysml") || ws.IsOpen("disk.sysml") {
		t.Fatalf("IsOpen: open=%v disk=%v", ws.IsOpen("open.sysml"), ws.IsOpen("disk.sysml"))
	}
	if names := ws.OpenNames(); len(names) != 1 || names[0] != "open.sysml" {
		t.Fatalf("OpenNames = %v, want [open.sysml]", names)
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
