package examples

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
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

	stated := map[string][]string{
		"quality.sysml/OpenSysMLInvariants": {
			"treeIsImmutable",
			"parserRecovers",
			"resolutionIsLazy",
			"tiersAreGated",
			"loweringIsLossless",
			"executionIsBounded",
			"libraryIsClean",
			"exportRoundTrips",
		},
		"identity.sysml/OpenSysMLIdentity": {
			"identityRoundTrips",
			"idsDoNotCollide",
			"identityIsBesideTheTree",
		},
	}

	for where, requirements := range stated {
		document, pkg, _ := strings.Cut(where, "/")
		scope := packageScope(t, idx, document, pkg)
		for _, name := range requirements {
			sym, ok := scope.LookupLocal(name)
			if !ok {
				t.Errorf("requirement %s is not declared in %s", name, pkg)
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
}

// TestSelfModelFiguresMatchImplementation reads the figures and package names
// the model declares and compares them against this implementation, so the
// model's numbers cannot quietly drift from the code they describe.
func TestSelfModelFiguresMatchImplementation(t *testing.T) {
	pipeline := readModelFile(t, "pipeline.sysml")

	figures := []struct {
		attribute string
		actual    int
	}{
		{"keywordCount", len(lexer.Keywords())},
		{"bundledFileCount", len(libs.DefaultSource().List())},
		{"tierCount", int(passes.LevelConstraint) + 1},
	}
	for _, figure := range figures {
		declared, ok := declaredInteger(pipeline, figure.attribute)
		if !ok {
			t.Errorf("pipeline.sysml declares no integer %s", figure.attribute)
			continue
		}
		if declared != figure.actual {
			t.Errorf("pipeline.sysml says %s = %d, the implementation has %d",
				figure.attribute, declared, figure.actual)
		}
	}

	// Every modelled unit names the directory that implements it.
	for _, file := range []string{"pipeline.sysml", "surfaces.sysml", "identity.sysml"} {
		matches := goPackagePattern.FindAllStringSubmatch(readModelFile(t, file), -1)
		if len(matches) == 0 {
			t.Errorf("%s names no implementing package", file)
			continue
		}
		for _, match := range matches {
			if _, err := os.Stat(filepath.Join("..", match[1])); err != nil {
				t.Errorf("%s points a unit at %s, which does not exist", file, match[1])
			}
		}
	}
}

// TestSelfModelIdentityMatchesImplementation checks the identity model against
// the identity implementation it describes: the library file it points at, the
// metadata definitions it names, and the tier its validation pass runs at.
func TestSelfModelIdentityMatchesImplementation(t *testing.T) {
	text := readModelFile(t, "identity.sysml")

	names := []struct {
		attribute string
		actual    string
	}{
		{"elementIdDefinition", identity.ElementIdFQN},
		{"projectRefDefinition", identity.ProjectRefFQN},
	}
	for _, figure := range names {
		declared, ok := declaredString(text, figure.attribute)
		if !ok {
			t.Errorf("identity.sysml declares no %s", figure.attribute)
			continue
		}
		if declared != figure.actual {
			t.Errorf("identity.sysml says %s = %q, the implementation has %q",
				figure.attribute, declared, figure.actual)
		}
	}

	file, ok := declaredString(text, "libraryFile")
	if !ok {
		t.Fatal("identity.sysml names no identity library file")
	}
	if _, err := os.Stat(filepath.Join("..", file)); err != nil {
		t.Errorf("identity.sysml points at %s, which does not exist", file)
	}

	if level := (passes.IdentityMetadataPass{}).Level(); level != passes.LevelConstraint {
		t.Errorf("identity.sysml models the identity pass at the constraint tier, the implementation runs it at %v", level)
	}
}

var goPackagePattern = regexp.MustCompile(`goPackage = "([^"]+)"`)

// readModelFile returns the text of one file of the self-model.
func readModelFile(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(selfModelDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

// declaredInteger returns the value the model text assigns to an integer attribute.
func declaredInteger(text, attribute string) (int, bool) {
	match := regexp.MustCompile(attribute + ` : Integer = (\d+);`).FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	return value, err == nil
}

// declaredString returns the value the model text assigns to a string attribute.
func declaredString(text, attribute string) (string, bool) {
	match := regexp.MustCompile(attribute + `\s*:\s*String\s*=\s*\n?\s*"([^"]+)";`).FindStringSubmatch(text)
	if match == nil {
		return "", false
	}
	return match[1], true
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
