package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Loader lazily loads library files from a Source and registers their symbols
// into a target index, using a Cache to skip parsing on repeat loads (Task 6).
type Loader struct {
	src   Source
	cache *Cache
}

// NewLoader returns a Loader over src, using cache for persistence.
func NewLoader(src Source, cache *Cache) *Loader {
	return &Loader{src: src, cache: cache}
}

// Load reads the named library file and registers its symbols into idx. On a
// cache hit the reduced record is restored directly, skipping lexing/parsing;
// on a miss the file is parsed, registered, and a reduced record is persisted.
func (l *Loader) Load(name string, idx *symbols.Index) error {
	content, err := l.src.Read(name)
	if err != nil {
		return err
	}

	// No cache: parse and register directly, skipping persistence.
	if l.cache == nil {
		p := parser.New(source.New(name, content))
		idx.AddDocument(name, p.ParseFile())
		return nil
	}

	key := l.cache.keyFor(content)

	// Cache hit: restore reduced records, skip lexing/parsing entirely.
	if rec, ok := l.cache.Load(key); ok {
		idx.AddRecords(name, recordEntries(rec))
		return nil
	}

	// Miss: parse, register, extract a reduced record, persist it.
	p := parser.New(source.New(name, content))
	root := p.ParseFile()
	idx.AddDocument(name, root)
	if rec := recordFromIndex(name, idx); rec != nil {
		_ = l.cache.Store(key, rec) // cache write failure is non-fatal
	}
	return nil
}

// recordEntries projects a persisted IndexRecord onto symbols.RecordEntry.
func recordEntries(rec *IndexRecord) []symbols.RecordEntry {
	out := make([]symbols.RecordEntry, len(rec.Symbols))
	for i, s := range rec.Symbols {
		out[i] = symbols.RecordEntry{
			FQN:             s.FQN,
			ShortName:       s.ShortName,
			Kind:            s.Kind,
			Span:            s.Span,
			WildcardImports: s.WildcardImports,
			AliasTarget:     s.AliasTarget,
		}
	}
	return out
}
