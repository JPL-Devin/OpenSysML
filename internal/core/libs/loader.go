package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Loader lazily loads library files from a Source and registers their symbols
// into a target index, using a Cache to skip parsing on repeat loads (Task 6).
type Loader struct {
	src    Source
	cache  *Cache
	parsed []pending // documents parsed this session, awaiting Persist
}

// pending is a parsed document whose cache record has not been written yet.
type pending struct {
	name string
	key  string
}

// NewLoader returns a Loader over src, using cache for persistence.
func NewLoader(src Source, cache *Cache) *Loader {
	return &Loader{src: src, cache: cache}
}

// Load reads the named library file and registers its symbols into idx. On a
// cache hit the reduced record is restored directly, skipping lexing/parsing;
// on a miss the file is parsed and registered, and its record is written by a
// later call to Persist.
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

	// Miss: parse and register now; the record is written by Persist, once the
	// whole library is indexed and cross-file supertypes resolve.
	p := parser.New(source.New(name, content))
	root := p.ParseFile()
	idx.AddDocument(name, root)
	l.parsed = append(l.parsed, pending{name: name, key: key})
	return nil
}

// Persist caches a reduced record of every document this loader parsed. It is
// separate from Load because a record holds resolved supertype names: a
// specialization target in one library file may be declared in another, so
// records can only be built once every file has been indexed.
func (l *Loader) Persist(idx *symbols.Index) {
	if l.cache == nil {
		l.parsed = nil
		return
	}
	r := resolve.New(idx)
	for _, p := range l.parsed {
		if rec := recordFromIndex(p.name, idx, r); rec != nil {
			_ = l.cache.Store(p.key, rec) // cache write failure is non-fatal
		}
	}
	l.parsed = nil
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
			Supers:          s.Supers,
			WildcardImports: s.WildcardImports,
			AliasTarget:     s.AliasTarget,
		}
	}
	return out
}
