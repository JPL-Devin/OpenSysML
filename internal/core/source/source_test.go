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

func TestLineIndex(t *testing.T) {
	// bytes:  p a r t \n d e f  \n E
	// offset: 0 1 2 3 4  5 6 7   8 9
	sf := New("t.sysml", []byte("part\ndef\nE"))
	li := sf.Lines()
	cases := []struct {
		offset    int
		line, col int
	}{
		{0, 1, 1}, // 'p'
		{3, 1, 4}, // 't'
		{4, 1, 5}, // '\n' belongs to line 1
		{5, 2, 1}, // 'd'
		{9, 3, 1}, // 'E' (offset 8 is the second '\n', belongs to line 2)
	}
	for _, c := range cases {
		got := li.PosAt(c.offset)
		if got.Line != c.line || got.Col != c.col {
			t.Errorf("PosAt(%d) = %+v, want Line %d Col %d", c.offset, got, c.line, c.col)
		}
	}
}

func TestLineIndexOffsetAt(t *testing.T) {
	sf := New("t.sysml", []byte("part\ndef\nE"))
	li := sf.Lines()
	if got := li.OffsetAt(Pos{Line: 2, Col: 1}); got != 5 {
		t.Fatalf("OffsetAt(2,1) = %d, want 5", got)
	}
}
