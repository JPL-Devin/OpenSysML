package libs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Loader lazily loads library files from a Source and registers their symbols
// into a target index, using a Cache to skip parsing on repeat loads (Task 6).
type Loader struct {
	src   Source
	cache *Cache
	// RequireResolved makes Persist skip a document with an unresolved
	// specialization target: a key does not describe the index a record was built
	// in, so one built in a partially populated index must not be reused where
	// the target exists.
	RequireResolved bool
	parsed          []pending // documents parsed this session, awaiting Persist
	digest          string    // memoized digest of the library set (see setDigest)
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

	key := l.cache.keyFor(content, l.setDigest())

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
		rec, resolved := recordFromIndex(p.name, idx, r)
		if rec == nil || (l.RequireResolved && !resolved) {
			continue
		}
		_ = l.cache.Store(p.key, rec) // cache write failure is non-fatal
	}
	l.parsed = nil
}

// setDigest hashes the name and content of every file in the source, memoized.
// It goes into the cache key so a record cannot outlive an edit to a sibling
// file it drew a value from: a unit reduction follows a reference unit or a
// prefix declared in another file, and keying on this file alone kept the
// reduction computed against the old definition. An unreadable file digests as
// its read error, which still changes the key once it becomes readable.
func (l *Loader) setDigest() string {
	if l.digest != "" {
		return l.digest
	}
	names := append([]string(nil), l.src.List()...)
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		content, err := l.src.Read(name)
		if err != nil {
			fmt.Fprintf(h, "%s\x00!%v\x00", name, err)
			continue
		}
		fmt.Fprintf(h, "%s\x00%x\x00", name, sha256.Sum256(content))
	}
	l.digest = hex.EncodeToString(h.Sum(nil))
	return l.digest
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
			WildcardImports: wildcardImportEntries(s.WildcardImports),
			AliasTarget:     s.AliasTarget,
			Unit:            unitFactsEntry(s.Unit),
		}
	}
	return out
}

// unitFactsEntry projects a persisted unit reduction onto its index form.
func unitFactsEntry(facts *unitFacts) *symbols.UnitFacts {
	if facts == nil {
		return nil
	}
	out := &symbols.UnitFacts{ScaleNum: facts.ScaleNum, ScaleDen: facts.ScaleDen, Irreducible: facts.Irreducible}
	for _, f := range facts.Factors {
		out.Factors = append(out.Factors, symbols.UnitFactorFacts{FQN: f.FQN, Exponent: f.Exponent})
	}
	return out
}

// wildcardImportEntries projects persisted wildcard imports onto their index form.
func wildcardImportEntries(imports []wildcardImport) []symbols.WildcardImport {
	if len(imports) == 0 {
		return nil
	}
	out := make([]symbols.WildcardImport, len(imports))
	for i, imp := range imports {
		out[i] = symbols.WildcardImport{Target: imp.Target, Private: imp.Private}
	}
	return out
}
