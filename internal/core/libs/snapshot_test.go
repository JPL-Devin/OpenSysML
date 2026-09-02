package libs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/pack"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/testutil/graphcmp"
)

// TestEmbeddedSnapshotIsCurrent fails when the committed snapshot was not
// regenerated after the bundled library, or the snapshot format, changed.
func TestEmbeddedSnapshotIsCurrent(t *testing.T) {
	want, err := BuildSnapshot(EmbeddedSource())
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if !bytes.Equal(want, stdlibSnapshot) {
		t.Fatalf("stdlib.snapshot is stale (%d bytes embedded, %d regenerated); run `go generate ./internal/core/libs`",
			len(stdlibSnapshot), len(want))
	}
}

// TestSnapshotIndexMatchesFreshLoad locks the decoded snapshot to the index a
// load of the same files builds, on the same surface the concurrent load is
// held to, and checks that re-encoding it gives the snapshot back.
func TestSnapshotIndexMatchesFreshLoad(t *testing.T) {
	fresh := symbols.NewIndex()
	loader := NewLoader(EmbeddedSource(), nil)
	if err := loader.LoadAll(fresh); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	fresh.Freeze()

	decoded, err := DecodeSnapshot(stdlibSnapshot, loader.setDigest())
	if err != nil {
		t.Fatalf("DecodeSnapshot: %v", err)
	}
	if !decoded.Frozen() {
		t.Fatal("decoded index is not frozen")
	}
	// The whole object graph, less the lookup caches an index fills lazily and
	// the inline storage a multi-part name leaves behind (see symbols' tests).
	if err := graphcmp.Equal(fresh, decoded, graphcmp.SkipFields(
		"Index.directChildrenGeneration", "Index.directChildrenCache", "QualifiedName.part0",
	)); err != nil {
		t.Errorf("decoded index differs from a fresh load: %v", err)
	}
	want, got := indexFingerprint(fresh), indexFingerprint(decoded)
	if want != got {
		t.Errorf("decoded index differs from a fresh load:\n%s", firstDifference(want, got))
	}
	if want, got := indexRecords(fresh), indexRecords(decoded); !reflect.DeepEqual(want, got) {
		t.Errorf("derived records differ from a fresh load")
	}

	w := pack.NewWriter()
	if err := decoded.WriteSnapshot(w); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}
	if again, err := BuildSnapshot(EmbeddedSource()); err != nil {
		t.Fatal(err)
	} else if !bytes.HasSuffix(again, w.Bytes()) {
		t.Errorf("re-encoding the decoded index does not reproduce the snapshot stream")
	}
}

func TestDecodeSnapshotRefusesOtherFiles(t *testing.T) {
	if _, err := DecodeSnapshot(stdlibSnapshot, "0000"); !errors.Is(err, ErrSnapshotStale) {
		t.Errorf("digest mismatch: got %v, want ErrSnapshotStale", err)
	}
	other := append([]byte(snapshotMagic), byte(snapshotFormatVersion+1))
	other = append(other, "\x04abcd"...)
	if _, err := DecodeSnapshot(other, "abcd"); !errors.Is(err, ErrSnapshotStale) {
		t.Errorf("format mismatch: got %v, want ErrSnapshotStale", err)
	}
}

// TestDecodeSnapshotRejectsCorruption feeds truncated and altered snapshots to
// the decoder, which must report each rather than panic or return an index.
func TestDecodeSnapshotRejectsCorruption(t *testing.T) {
	digest := NewLoader(EmbeddedSource(), nil).setDigest()
	if _, err := DecodeSnapshot([]byte("not a snapshot"), digest); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("garbage: got %v, want ErrCorrupt", err)
	}
	for _, n := range []int{len(snapshotMagic) + 3, len(stdlibSnapshot) / 2, len(stdlibSnapshot) - 1} {
		if _, err := DecodeSnapshot(stdlibSnapshot[:n], digest); err == nil {
			t.Errorf("truncated to %d bytes: decoded without error", n)
		}
	}
	// Damage that still parses — a byte inside a string, a run in the string
	// table — is caught by the checksum, so no altered blob is decoded.
	header := len(snapshotMagic) + len(digest) + 3
	for _, at := range []int{header, 200000, len(stdlibSnapshot) * 3 / 4, len(stdlibSnapshot) - 200} {
		data := bytes.Clone(stdlibSnapshot)
		data[at] ^= 0xff
		if _, err := DecodeSnapshot(data, digest); !errors.Is(err, pack.ErrCorrupt) {
			t.Errorf("byte %d flipped: got %v, want ErrCorrupt", at, err)
		}
	}
	data := bytes.Clone(stdlibSnapshot)
	for i := 200000; i < 200064; i++ {
		data[i] ^= 0xff
	}
	if _, err := DecodeSnapshot(data, digest); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("64 bytes flipped: got %v, want ErrCorrupt", err)
	}
	if _, err := DecodeSnapshot(append(bytes.Clone(stdlibSnapshot), 0), digest); !errors.Is(err, pack.ErrCorrupt) {
		t.Errorf("trailing byte: got %v, want ErrCorrupt", err)
	}
}

// TestSnapshotIndexFollowsLibraryPath checks that a library override with
// other content is loaded from its files, while one with the bundled content
// is served from the snapshot.
func TestSnapshotIndexFollowsLibraryPath(t *testing.T) {
	dir := t.TempDir()
	for _, name := range EmbeddedSource().List() {
		data, err := EmbeddedSource().Read(name)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(LibraryPathEnvVar, dir)
	if _, err := SnapshotIndex(); err != nil {
		t.Errorf("identical override: %v, want the snapshot", err)
	}

	edited := filepath.Join(dir, "Kernel Libraries", "Kernel Data Type Library", "ScalarValues.kerml")
	if err := os.WriteFile(edited, []byte("standard library package ScalarValues { datatype Edited; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotIndex(); !errors.Is(err, ErrSnapshotStale) {
		t.Errorf("edited override: got %v, want ErrSnapshotStale", err)
	}
	idx := symbols.NewIndex()
	if err := NewLoader(DefaultSource(), nil).LoadAll(idx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(idx.LookupQualified("ScalarValues::Edited")) == 0 {
		t.Error("the edited library was not loaded from the override directory")
	}
}

// BenchmarkDecodeSnapshot times what a process pays to start from the
// snapshot instead of from the files.
func BenchmarkDecodeSnapshot(b *testing.B) {
	digest := NewLoader(EmbeddedSource(), nil).setDigest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeSnapshot(stdlibSnapshot, digest); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSetDigest times the check that the snapshot matches the files.
func BenchmarkSetDigest(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewLoader(EmbeddedSource(), nil).setDigest()
	}
}
