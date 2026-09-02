package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// parsedLibrary parses the bundled library once, outside the timed region.
func parsedLibrary(b *testing.B) []libraryFile {
	ld := NewLoader(DefaultSource(), nil)
	files := ld.readAll()
	parallelFor(len(files), func(i int) { files[i].parse() })
	for _, f := range files {
		if f.err != nil {
			b.Fatalf("%s: %v", f.name, f.err)
		}
	}
	return files
}

// unexpandedIndex registers parsed files into a fresh index, without expanding.
func unexpandedIndex(files []libraryFile) *symbols.Index {
	idx := symbols.NewIndex()
	for _, f := range files {
		idx.AddDocument(f.name, f.root)
		idx.MarkLibrary(f.name)
	}
	return idx
}

// BenchmarkExpandWildcardImports times wildcard-import expansion of the bundled
// library alone: the index is built outside the timed region.
func BenchmarkExpandWildcardImports(b *testing.B) {
	files := parsedLibrary(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		idx := unexpandedIndex(files)
		b.StartTimer()
		idx.ExpandWildcardImports()
	}
}

// BenchmarkIndexLibrary times registering the parsed library into an index,
// the step that precedes expansion.
func BenchmarkIndexLibrary(b *testing.B) {
	files := parsedLibrary(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unexpandedIndex(files)
	}
}
