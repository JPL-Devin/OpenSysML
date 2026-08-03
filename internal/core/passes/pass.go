package passes

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// PassLevel is the dependency tier of a validation pass. Passes at a higher
// level are skipped when a lower level produced an error, avoiding cascade
// noise (e.g. no type-check on an unresolved reference).
type PassLevel int

const (
	LevelSyntax PassLevel = iota
	LevelNameResolution
	LevelType
	LevelConstraint
)

var passLevelNames = map[PassLevel]string{
	LevelSyntax:         "syntax",
	LevelNameResolution: "name-resolution",
	LevelType:           "type",
	LevelConstraint:     "constraint",
}

// String returns the lowercase name of the level, or "unknown".
func (l PassLevel) String() string {
	if name, ok := passLevelNames[l]; ok {
		return name
	}
	return "unknown"
}

// Pass is a single validation rule. Level reports its dependency tier; Run
// executes it over a whole document and returns any diagnostics found.
type Pass interface {
	Level() PassLevel
	Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic
}

// Context carries shared state made available to every pass in a run: the
// document name, the global symbol index, a lazily-created shared resolver,
// and the syntax diagnostics produced by the caller's parse (passes must not
// import the parser, so these enter here).
type Context struct {
	Name             string
	Index            *symbols.Index
	ParseDiagnostics []Diagnostic

	resolver *resolve.Resolver
	model    *semantics.Model
}

// NewContext builds a Context for a document.
func NewContext(name string, idx *symbols.Index, parseDiags []Diagnostic) *Context {
	return &Context{Name: name, Index: idx, ParseDiagnostics: parseDiags}
}

// Resolver returns the shared resolver for this context, creating it on first
// use so multiple passes reuse one memoized instance.
func (c *Context) Resolver() *resolve.Resolver {
	if c.resolver == nil {
		c.resolver = resolve.New(c.Index)
	}
	return c.resolver
}

// Model returns the shared semantic model (specialization graph, multiplicity,
// inherited members, evaluator) for this context, creating it on first use over
// the shared resolver so constraint passes reuse one memoized instance.
func (c *Context) Model() *semantics.Model {
	if c.model == nil {
		c.model = semantics.NewModel(c.Resolver())
		// Attach model to resolver for inheritance-aware member resolution
		c.Resolver().SetModel(c.model)
	}
	return c.model
}
