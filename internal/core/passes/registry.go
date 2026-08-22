package passes

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// Registry holds an ordered set of validation passes.
type Registry struct {
	passes []Pass
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a pass to the registry.
func (r *Registry) Register(p Pass) {
	r.passes = append(r.passes, p)
}

// Run executes all registered passes in ascending PassLevel order and returns
// their accumulated diagnostics. If a pass at some level emits an
// Error-severity diagnostic, passes at strictly higher levels are skipped to
// avoid cascade noise. Passes at the same level always run.
func (r *Registry) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	ordered := make([]Pass, len(r.passes))
	copy(ordered, r.passes)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Level() < ordered[j].Level()
	})

	var all []Diagnostic
	// failedLevel is the lowest level at which an Error occurred; passes at a
	// strictly higher level are skipped.
	failed := false
	var failedLevel PassLevel
	for _, p := range ordered {
		if failed && p.Level() > failedLevel {
			continue
		}
		diags := p.Run(ctx, name, root)
		all = append(all, diags...)
		if hasError(diags) {
			if !failed || p.Level() < failedLevel {
				failedLevel = p.Level()
			}
			failed = true
		}
	}
	return all
}

// hasError reports whether a level failed in a way the next one depends on: a
// notation error says how the model is written, not what it means, so it never
// gates a higher tier.
func hasError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError && !d.Notation {
			return true
		}
	}
	return false
}
