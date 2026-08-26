// Package errata is the declared overlay of defects in the OMG-published
// material this project reads: what the published text says, the clause that
// makes it wrong, and what the oracles read instead. The published bytes on disk
// are never rewritten; corrections live only here.
package errata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IssuesPath is the page every entry must be documented in.
const IssuesPath = "docs/project/omg-issues.md"

// publishedRoots are the repository paths holding OMG-published material. An
// entry outside them would be a correction to our own material, which is a
// defect of ours to fix rather than an erratum to declare.
var publishedRoots = []string{
	"examples/pilot-corpora",
	"build/pilot-xpect-corpus",
}

// Entry is one defect in published material. The span is one whole line:
// AsPublished is that line's exact bytes without its terminator, which is what
// lets a re-vendored corpus invalidate the entry instead of silently rotting it.
type Entry struct {
	// ID is the entry's stable internal label, as the oracles report it.
	ID string
	// Heading titles the docs/project/omg-issues.md section documenting the
	// defect, without its `### ` marker.
	Heading string
	// Path is the defective file, relative to the repository root.
	Path string
	// Line is the 1-based line the entry covers.
	Line int
	// AsPublished is the published line, verbatim.
	AsPublished string
	// Corrected replaces AsPublished when non-empty. Empty means documented
	// without a correction: nothing is substituted, and the defect stands.
	Corrected string
	// Citation is the specification clause the published text violates.
	Citation string
	// Derivation states in one line why that clause is violated.
	Derivation string
}

// Corrects reports whether the entry substitutes text.
func (e Entry) Corrects() bool { return e.Corrected != "" }

// Validate reports why an entry may not be accepted.
func (e Entry) Validate() error {
	switch {
	case e.ID == "":
		return fmt.Errorf("entry for %s:%d names no %s row", e.Path, e.Line, IssuesPath)
	case e.Heading == "":
		return fmt.Errorf("entry %s names no %s section documenting it", e.ID, IssuesPath)
	case e.Path == "":
		return fmt.Errorf("entry %s names no file", e.ID)
	case e.Line < 1:
		return fmt.Errorf("entry %s: line %d is not a line", e.ID, e.Line)
	case strings.TrimSpace(e.AsPublished) == "":
		return fmt.Errorf("entry %s: no as-published text to match against the corpus", e.ID)
	case e.Citation == "":
		return fmt.Errorf("entry %s: no specification citation makes the published text wrong", e.ID)
	case e.Derivation == "":
		return fmt.Errorf("entry %s: no derivation from %s is written down", e.ID, e.Citation)
	case e.Corrected == e.AsPublished:
		return fmt.Errorf("entry %s: the correction repeats the published text", e.ID)
	case strings.Contains(e.AsPublished, "\n") || strings.Contains(e.Corrected, "\n"):
		return fmt.Errorf("entry %s: an entry covers one line", e.ID)
	}
	if !underPublishedRoot(e.Path) {
		return fmt.Errorf("entry %s: %s is not OMG-published material (%s)", e.ID, e.Path, strings.Join(publishedRoots, ", "))
	}
	return nil
}

func underPublishedRoot(path string) bool {
	for _, root := range publishedRoots {
		if strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// Registry is the declared errata, in report order.
func Registry() []Entry {
	return []Entry{{
		ID:      "F82",
		Heading: "`radius = 22/2*25.4 + 110 [mm]` adds a dimensionless value to a length",
		Path:    "examples/pilot-corpora/sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml",
		Line:    38,
		// The published line keeps its trailing space; the correction keeps it too.
		AsPublished: "            :>> radius = 22/2*25.4 + 110 [mm]; ",
		Corrected:   "            :>> radius = (22/2*25.4 + 110) [mm]; ",
		Citation:    "SysML v2 §9.8.9.1",
		Derivation:  "the unit postfix binds to PrimaryExpression (KerMLExpressions.xtext:308), below AdditiveExpression, so `[mm]` qualifies 110 alone and `+` adds a dimensionless value to a length, which §9.8.9.1 forbids.",
	}, {
		ID:          "F83",
		Heading:     "`1/(2 * Cp) * V^2 + T_static` adds L^6 to Θ",
		Path:        "examples/pilot-corpora/sysml-examples/Analysis Examples/Turbojet Stage Analysis.sysml",
		Line:        25,
		AsPublished: "\t    \treturn : TemperatureValue = 1/(2 * Cp) * V^2 + T_static;",
		Citation:    "SysML v2 §9.8.9.1",
		Derivation:  "V is a VolumeValue and Cp dimensionless, so the first operand has dimension L^6 while T_static has Θ; no reading of the published text shares a dimension, so the defect is documented without a correction.",
	}, {
		ID:          "F111",
		Heading:     "`fuelConsumption : FuelEconomyAnalysis_1` redefines an action typed by `FuelConsumption`",
		Path:        "examples/pilot-corpora/sysml-examples/Individuals Examples/AnalysisIndividualExample.sysml",
		Line:        86,
		AsPublished: "\t\t\tindividual action :>> fuelConsumption : FuelEconomyAnalysis_1 {",
		Corrected:   "\t\t\tindividual action :>> fuelConsumption : FuelConsumption_1 {",
		Citation:    "KerML 7.4.9, 8.3.4.2",
		Derivation:  "a redefinition is a subsetting, so the redefining feature's type must conform to the redefined one's, and an analysis definition is not a FuelConsumption; the file declares `individual action def FuelConsumption_1 :> FuelConsumption` and uses it nowhere else, which is the conforming type the line names by mistake.",
	}}
}

// Overlay is a registry indexed by file, ready to apply.
type Overlay struct {
	entries []Entry
	byPath  map[string]Entry
}

// Load returns the declared registry as an overlay.
func Load() (*Overlay, error) { return New(Registry()) }

// New validates entries and indexes them by path.
func New(entries []Entry) (*Overlay, error) {
	overlay := &Overlay{byPath: make(map[string]Entry, len(entries))}
	ids := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		if ids[entry.ID] {
			return nil, fmt.Errorf("entry %s is declared twice", entry.ID)
		}
		if other, ok := overlay.byPath[entry.Path]; ok {
			// One entry per file keeps application order irrelevant.
			return nil, fmt.Errorf("entries %s and %s both cover %s", other.ID, entry.ID, entry.Path)
		}
		ids[entry.ID] = true
		overlay.byPath[entry.Path] = entry
		overlay.entries = append(overlay.entries, entry)
	}
	return overlay, nil
}

// Entries returns every declared entry.
func (o *Overlay) Entries() []Entry {
	if o == nil {
		return nil
	}
	return o.entries
}

// Corrections returns the entries that substitute text.
func (o *Overlay) Corrections() []Entry {
	var out []Entry
	for _, entry := range o.Entries() {
		if entry.Corrects() {
			out = append(out, entry)
		}
	}
	return out
}

// Documented returns the entries documented without a correction.
func (o *Overlay) Documented() []Entry {
	var out []Entry
	for _, entry := range o.Entries() {
		if !entry.Corrects() {
			out = append(out, entry)
		}
	}
	return out
}

// Under returns the correcting entries whose file lies under a repository path,
// keyed by the path relative to it.
func (o *Overlay) Under(dir string) map[string]Entry {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	out := map[string]Entry{}
	for _, entry := range o.Corrections() {
		if rel, ok := strings.CutPrefix(entry.Path, prefix); ok {
			out[rel] = entry
		}
	}
	return out
}

// Apply returns content with the entry's correction substituted. It fails
// unless the declared line still reads as published, so a re-vendored corpus is
// a hard error rather than a silently skipped correction.
func Apply(entry Entry, content []byte) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	if entry.Line > len(lines) {
		return nil, fmt.Errorf("%s: %s has %d lines, entry names line %d", entry.ID, entry.Path, len(lines), entry.Line)
	}
	if got := lines[entry.Line-1]; got != entry.AsPublished {
		return nil, fmt.Errorf("%s: %s:%d reads %q, the entry records %q", entry.ID, entry.Path, entry.Line, got, entry.AsPublished)
	}
	if !entry.Corrects() {
		return content, nil
	}
	lines[entry.Line-1] = entry.Corrected
	return []byte(strings.Join(lines, "\n")), nil
}

// Materialize copies the corpus root at repo/dir into dst and applies every
// correction inside it, leaving the published tree untouched. It returns the
// applied entries, keyed by the path relative to the root.
func (o *Overlay) Materialize(repo, dir, dst string) (map[string]Entry, error) {
	applied := o.Under(dir)
	if len(applied) == 0 {
		return nil, fmt.Errorf("no correction lies under %s", dir)
	}
	if err := os.RemoveAll(dst); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return nil, err
	}
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(repo, filepath.FromSlash(dir)))); err != nil {
		return nil, fmt.Errorf("copy %s: %w", dir, err)
	}
	for rel, entry := range applied {
		path := filepath.Join(dst, filepath.FromSlash(rel))
		content, err := os.ReadFile(path) // #nosec G304 -- the path is inside the copy this function just made
		if err != nil {
			return nil, err
		}
		corrected, err := Apply(entry, content)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, corrected, 0o600); err != nil {
			return nil, err
		}
	}
	return applied, nil
}
