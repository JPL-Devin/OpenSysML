package source

import "testing"

func TestSpanText(t *testing.T) {
	sf := New("test.sysml", []byte("part def Engine;"))
	sp := Span{Offset: 9, Len: 6}
	if got := sf.Text(sp); got != "Engine" {
		t.Fatalf("Text(%v) = %q, want %q", sp, got, "Engine")
	}
	if sf.Name() != "test.sysml" {
		t.Fatalf("Name() = %q, want %q", sf.Name(), "test.sysml")
	}
	if sf.Len() != 16 {
		t.Fatalf("Len() = %d, want 16", sf.Len())
	}
}

func TestSpanEnd(t *testing.T) {
	sp := Span{Offset: 4, Len: 3}
	if sp.End() != 7 {
		t.Fatalf("End() = %d, want 7", sp.End())
	}
}
