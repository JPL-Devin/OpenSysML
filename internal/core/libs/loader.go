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

// Load reads the named library file, parses it, and registers the resulting
// scope into idx. Cache integration is added in Task 6.
func (l *Loader) Load(name string, idx *symbols.Index) error {
	content, err := l.src.Read(name)
	if err != nil {
		return err
	}
	p := parser.New(source.New(name, content))
	root := p.ParseFile()
	idx.AddDocument(name, root)
	return nil
}
