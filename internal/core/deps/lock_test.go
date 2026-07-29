package deps

import "testing"

func TestReadLockRoundTrip(t *testing.T) {
	src := []byte("# lockfile\nsi = \"abc123\"\ngeometry = \"def456\"\n")
	lock, err := ReadLock(src)
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	if got := lock.SHA["si"]; got != "abc123" {
		t.Errorf("si sha = %q, want abc123", got)
	}
	if got := lock.SHA["geometry"]; got != "def456" {
		t.Errorf("geometry sha = %q, want def456", got)
	}

	out := lock.Bytes()
	round, err := ReadLock(out)
	if err != nil {
		t.Fatalf("ReadLock(round): %v", err)
	}
	if round.SHA["si"] != "abc123" || round.SHA["geometry"] != "def456" {
		t.Errorf("round-trip mismatch: %#v", round.SHA)
	}
}

func TestNewLockEmpty(t *testing.T) {
	lock := NewLock()
	if len(lock.SHA) != 0 {
		t.Errorf("new lock not empty: %#v", lock.SHA)
	}
	if got := string(lock.Bytes()); got != "" {
		t.Errorf("empty lock Bytes = %q, want \"\"", got)
	}
}

func TestReadLockIgnoresBlankAndComments(t *testing.T) {
	lock, err := ReadLock([]byte("\n  # comment\n\nsi = \"x\"\n"))
	if err != nil {
		t.Fatalf("ReadLock: %v", err)
	}
	if len(lock.SHA) != 1 || lock.SHA["si"] != "x" {
		t.Errorf("unexpected: %#v", lock.SHA)
	}
}
