package libs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Loader loads library files from a Source and registers their symbols into a
// target index, using a Cache to skip parsing on repeat loads (Task 6).
// A library is index-only: it contributes the names, kinds and specializations a
// record holds, never declarations, whether it was parsed or restored.
type Loader struct {
	src   Source
	cache *Cache
	// RequireResolved makes LoadAll cache no document with an unresolved
	// specialization target, since a key does not describe the index a record was
	// built in.
	RequireResolved bool
	parsed          []pending // documents parsed this session, awaiting reduce
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

// LoadAll registers every file of the source into idx as library content and
// leaves it in record form. It loads the whole set at once because a record
// holds supertype names a sibling file may declare; an incomplete library is
// reported and not cached.
func (l *Loader) LoadAll(idx *symbols.Index) error {
	var errs []error
	for _, name := range l.src.List() {
		if err := l.load(name, idx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	// A facade package re-exports what another library package declares.
	idx.ExpandWildcardImports()

	l.reduce(idx, len(errs) == 0)
	return errors.Join(errs...)
}

// load registers the named library file into idx: a cache hit restores its
// record, a miss parses it for reduce to replace with one. Loading without a
// cache still reduces.
func (l *Loader) load(name string, idx *symbols.Index) error {
	content, err := l.src.Read(name)
	if err != nil {
		return err
	}

	var key string
	if l.cache != nil {
		key = l.cache.keyFor(content, l.setDigest())
		// Cache hit: restore reduced records, skip lexing/parsing entirely.
		if rec, ok := l.cache.Load(key); ok {
			idx.AddRecords(name, recordEntries(rec))
			idx.MarkLibrary(name)
			return nil
		}
	}

	// Miss: parse and register now; reduce records it once the whole library is
	// indexed and cross-file supertypes resolve.
	p := parser.New(source.New(name, content))
	root := p.ParseFile()
	idx.AddDocument(name, root)
	idx.MarkLibrary(name)
	l.parsed = append(l.parsed, pending{name: name, key: key})
	return nil
}

// reduce replaces every document this loader parsed with the record a cache hit
// would have restored, caching those records when store is set. This is what
// makes a parsed and a restored library leave the same index.
func (l *Loader) reduce(idx *symbols.Index, store bool) {
	parsed := l.parsed
	l.parsed = nil
	if len(parsed) == 0 {
		return
	}

	r := resolve.New(idx)
	model := semantics.NewModel(r) // shared: its whole-index memoization is per-model
	type built struct {
		doc      pending
		rec      *IndexRecord
		resolved bool
	}
	records := make([]built, 0, len(parsed))
	for _, p := range parsed {
		rec, resolved := recordFromIndex(p.name, idx, r, model)
		if rec == nil {
			continue
		}
		records = append(records, built{doc: p, rec: rec, resolved: resolved})
	}

	// Every record is built before any document is replaced, since a record reads
	// the index the not-yet-replaced documents are in.
	for _, b := range records {
		idx.AddRecords(b.doc.name, recordEntries(b.rec))
		idx.MarkLibrary(b.doc.name)
	}
	idx.ExpandWildcardImports() // the records replacing the documents state imports of their own

	if !store || l.cache == nil {
		return
	}
	// Records the library stopped asking for are pruned here, where a write happened.
	defer l.cache.Prune()
	for _, b := range records {
		if l.RequireResolved && !b.resolved {
			continue
		}
		_ = l.cache.Store(b.doc.key, b.rec) // cache write failure is non-fatal
	}
}

// setDigest hashes the name and content of every file in the source, memoized.
// It goes into the cache key so a record cannot outlive an edit to a sibling file
// it drew a value from, such as a unit reduction following a reference unit
// declared elsewhere. An unreadable file digests as its read error.
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
			FeaturedBy:      s.FeaturedBy,
			WildcardImports: wildcardImportEntries(s.WildcardImports),
			AliasTarget:     s.AliasTarget,
			Unit:            unitFactsEntry(s.Unit),
			Dimension:       s.Dimension,
			Behavior:        s.Behavior,

			Annotations:      s.Annotations,
			NamespaceFilters: s.NamespaceFilters,
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
		if imp.Filter != nil {
			out[i].Filter = symbols.ElementFilter{Pred: imp.Filter, Span: imp.Filter.Span}
		}
	}
	return out
}
