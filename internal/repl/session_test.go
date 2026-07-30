package repl

import "testing"

func TestNewSessionEmpty(t *testing.T) {
	s := NewSession()
	if s == nil {
		t.Fatal("NewSession returned nil")
	}
	if got := s.List(); len(got) != 0 {
		t.Fatalf("new session should have no declarations, got %v", got)
	}
}
