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

// Loader loads library files from a Source and registers them into a target
// index, using a Cache to skip the derivation a load would otherwise repeat.
// Every file is parsed and indexed on every load path, so a library contributes
// its declarations whether or not the cache held anything for it; a record
// carries only the derived facts whose derivation dominates a cold load.
type Loader struct {
	src   Source
	cache *Cache
	// RequireResolved makes LoadAll cache no document with an unresolved
	// specialization target, since a key does not describe the index a record was
	// built in.
	RequireResolved bool
	loaded          []pending // documents loaded this session, awaiting their facts
	digest          string    // memoized digest of the library set (see setDigest)
	hits            int       // documents of the last LoadAll whose facts a record supplied
}

// pending is a parsed document whose facts are not installed yet, with the cache
// record they were restored from or nil when the cache held none.
type pending struct {
	name string
	key  string
	rec  *IndexRecord
}

// NewLoader returns a Loader over src, using cache for persistence.
func NewLoader(src Source, cache *Cache) *Loader {
	return &Loader{src: src, cache: cache}
}

// LoadAll registers every file of the source into idx as library content and
// installs the derived facts of each. It loads the whole set at once because a
// record holds supertype names a sibling file may declare; an incomplete library
// is reported and not cached.
func (l *Loader) LoadAll(idx *symbols.Index) error {
	var errs []error
	l.hits = 0
	for _, name := range l.src.List() {
		if err := l.load(name, idx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	// A facade package re-exports what another library package declares.
	idx.ExpandWildcardImports()

	l.installFacts(idx, len(errs) == 0)
	return errors.Join(errs...)
}

// load parses the named library file into idx and marks it library content,
// noting the cache record its facts are restored from when there is one.
func (l *Loader) load(name string, idx *symbols.Index) error {
	content, err := l.src.Read(name)
	if err != nil {
		return err
	}

	doc := pending{name: name}
	if l.cache != nil {
		doc.key = l.cache.keyFor(content, l.setDigest())
		if rec, ok := l.cache.Load(doc.key); ok {
			doc.rec = rec
		}
	}

	p := parser.New(source.New(name, content))
	idx.AddDocument(name, p.ParseFile())
	idx.MarkLibrary(name)
	l.loaded = append(l.loaded, doc)
	return nil
}

// installFacts installs the derived facts of every loaded document: restored
// from its cache record where there was one, derived where there was not, and
// stored for the next load when store is set.
func (l *Loader) installFacts(idx *symbols.Index, store bool) {
	loaded := l.loaded
	l.loaded = nil

	// Restored facts go in first, so deriving the rest reads the same facts a
	// fully warm load would.
	var missing []pending
	for _, doc := range loaded {
		if doc.rec == nil {
			missing = append(missing, doc)
			continue
		}
		idx.InstallLibraryFacts(doc.name, libraryFacts(doc.rec))
		l.hits++
	}
	if len(missing) == 0 {
		return
	}

	r := resolve.New(idx)
	model := semantics.NewModel(r) // shared: its whole-index memoization is per-model
	type built struct {
		doc      pending
		rec      *IndexRecord
		resolved bool
	}
	records := make([]built, 0, len(missing))
	for _, doc := range missing {
		rec, resolved := recordFromIndex(doc.name, idx, r, model)
		if rec == nil {
			continue
		}
		records = append(records, built{doc: doc, rec: rec, resolved: resolved})
	}

	// Every record is derived before any is installed, so what one document
	// derives cannot be read back as a fact while another is still deriving.
	for _, b := range records {
		idx.InstallLibraryFacts(b.doc.name, libraryFacts(b.rec))
	}

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

// Hits reports how many documents of the last LoadAll had their facts installed
// from a cache record instead of derived, which is how a caller tells a warm load
// from a cold one.
func (l *Loader) Hits() int {
	return l.hits
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
