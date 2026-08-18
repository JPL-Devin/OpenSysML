package libs

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestStdlibReservedKeywordNames pins the places where the official OMG library
// spells a declaration name with a reserved keyword. SysML reserves keywords in
// name position, so these are reported — as warnings, not errors, precisely
// because the normative library relies on them and must keep parsing clean.
// The set is pinned so it cannot grow silently: a new entry means either an
// upstream library change or a parser bug that mistook a grammar keyword for a
// name (`binding [1] bind x = y` used to land here).
func TestStdlibReservedKeywordNames(t *testing.T) {
	want := []string{
		`Domain Libraries/Metadata/ImageMetadata.sysml: "type"`,
		`Domain Libraries/Quantities and Units/ISQChemistryMolecular.sysml: "multiplicity"`,
		// KerML has no `frame` keyword, so the Kernel Semantic Library writes
		// `in frame : SpatialFrame[1]`. The name is read and warned about here
		// rather than dropped, which is what it used to be.
		`Kernel Libraries/Kernel Semantic Library/SpatialFrames.kerml: "frame"`,
		`Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml: "entry"`,
		`Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml: "exit"`,
		`Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml: "accept"`,
		`Systems Library/Actions.sysml: "done"`,
		`Systems Library/Items.sysml: "done"`,
		`Systems Library/Parts.sysml: "done"`,
		`Systems Library/States.sysml: "done"`,
		`Systems Library/UseCases.sysml: "done"`,
	}

	src := &embedSource{}
	seen := map[string]bool{}
	for _, path := range src.List() {
		data, err := src.Read(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		p := parser.New(source.New(path, data))
		p.ParseFile()
		for _, w := range p.Warnings {
			name := strings.SplitN(w.Message, " is a reserved keyword", 2)[0]
			seen[fmt.Sprintf("%s: %s", path, name)] = true
		}
	}

	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
		if !seen[w] {
			t.Errorf("no longer warned about, remove it from the pinned set: %s", w)
		}
	}
	var unexpected []string
	for s := range seen {
		if !wantSet[s] {
			unexpected = append(unexpected, s)
		}
	}
	sort.Strings(unexpected)
	for _, s := range unexpected {
		t.Errorf("new reserved-keyword name in the library (parser bug, or upstream change): %s", s)
	}
}
