package examples

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const selfModelDir = "self-model"

// selfModelFiles returns the model files of the self-model, sorted, so the two
// gates below load the same set.
func selfModelFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(selfModelDir)
	if err != nil {
		t.Fatalf("read %s: %v", selfModelDir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || source.KindOf(entry.Name()) != source.KindSysML {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("%s holds no model files", selfModelDir)
	}
	return files
}

// TestSelfModelClean holds the architecture self-model to the standard the
// documentation it illustrates claims: it analyses without a diagnostic of any
// severity. Every file is opened before any is diagnosed, because the packages
// import across files.
func TestSelfModelClean(t *testing.T) {
	files := selfModelFiles(t)

	ws := model.NewWorkspace()
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(selfModelDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		ws.Open(name, content, 1)
	}

	for _, name := range files {
		var messages []string
		for _, d := range ws.Diagnostics(name) {
			messages = append(messages, d.Severity.String()+": "+d.Message)
		}
		if len(messages) > 0 {
			t.Errorf("%s/%s: %d diagnostic(s): %s",
				selfModelDir, name, len(messages), strings.Join(messages, "; "))
		}
	}
}

// TestSelfModelInvariantsHold evaluates the architecture invariants the model
// states, so a stated invariant that no longer describes this implementation
// fails here rather than reading as true in a diagram.
func TestSelfModelInvariantsHold(t *testing.T) {
	files := selfModelFiles(t)

	idx := model.NewIndexWithStdlib()
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(selfModelDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		idx.AddDocument(name, parser.New(source.New(name, content)).ParseFile())
	}

	resolver := resolve.New(idx)
	ctx := runtime.NewContext(semantics.NewModel(resolver), resolver, 100000)

	scope := packageScope(t, idx, "quality.sysml", "OpenSysMLInvariants")
	requirements := []string{
		"treeIsImmutable",
		"parserRecovers",
		"resolutionIsLazy",
		"tiersAreGated",
		"loweringIsLossless",
		"executionIsBounded",
		"libraryIsClean",
		"exportRoundTrips",
	}

	for _, name := range requirements {
		sym, ok := scope.LookupLocal(name)
		if !ok {
			t.Errorf("requirement %s is not declared in OpenSysMLInvariants", name)
			continue
		}
		satisfied, err := ctx.EvaluateRequirement(sym, scope)
		if err != nil {
			t.Errorf("evaluate %s: %v", name, err)
			continue
		}
		if !satisfied {
			t.Errorf("%s does not hold: the model states an invariant this implementation no longer satisfies", name)
		}
	}
}

// packageScope returns the scope of a top-level package of one document.
func packageScope(t *testing.T, idx *symbols.Index, document, name string) *symbols.Scope {
	t.Helper()

	root := idx.DocumentRoot(document)
	if root == nil {
		t.Fatalf("%s is not indexed", document)
	}
	for _, child := range root.Children() {
		if child.Node() == nil {
			continue
		}
		if sym, ok := root.LookupLocal(name); ok && sym.Decl == child.Node() {
			return child
		}
	}
	t.Fatalf("package %s is not declared in %s", name, document)
	return nil
}
