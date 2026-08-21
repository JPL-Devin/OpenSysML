package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// warningCodes parses input and returns the codes of the warnings it produced,
// failing the test if the parse is ill-formed.
func warningCodes(t *testing.T, input string) []string {
	return warningCodesIn(t, "modifier_kind.sysml", input)
}

// warningCodesIn parses input as the named source, so a case can state which
// language reserves the words it writes.
func warningCodesIn(t *testing.T, name, input string) []string {
	t.Helper()
	p := New(source.New(name, []byte(input)))
	p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error in %q: %s", input, d.Message)
	}
	var codes []string
	for _, w := range p.Warnings {
		codes = append(codes, w.Code)
	}
	return codes
}

func hasCode(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// `individual part : Vehicle` reads `part` as the kind, leaving the usage unnamed;
// the parser keeps the kind and reports the ambiguity for every such modifier.
func TestAmbiguousModifierKindWarns(t *testing.T) {
	ambiguous := []string{
		"individual part : Vehicle;",
		"individual item : Integer;",
		"individual occurrence;",
		"ref item : Integer;",
		"ref part { }",
		"snapshot part : Vehicle;",
		"timeslice item : Integer;",
		"action def A { in individual part : Vehicle; }",
	}
	for _, input := range ambiguous {
		t.Run(input, func(t *testing.T) {
			if codes := warningCodes(t, input); !hasCode(codes, codeAmbiguousModifierKind) {
				t.Errorf("no %s warning, warnings = %v", codeAmbiguousModifierKind, codes)
			}
		})
	}
}

// A named declaration is unambiguous, and so is a modifier with no kind keyword
// after it, whether or not it declares a name.
func TestUnambiguousModifiedUsagesDoNotWarn(t *testing.T) {
	unambiguous := []string{
		"individual part ip : Vehicle;",
		"individual ip : Vehicle;",
		"individual 'part' : Vehicle;",
		"individual : Vehicle;",
		"individual part def IP;",
		"ref x : Integer;",
		"snapshot s : Flight;",
		"snapshot part sp : Vehicle;",
		"part : Vehicle;",
		"timeslice item ts : Integer;",
	}
	for _, input := range unambiguous {
		t.Run(input, func(t *testing.T) {
			if codes := warningCodes(t, input); hasCode(codes, codeAmbiguousModifierKind) {
				t.Errorf("unexpected %s warning, warnings = %v", codeAmbiguousModifierKind, codes)
			}
		})
	}
}

// The Kernel Semantic Library names features `frame` and `render`, which only
// KerML admits: SysML.xtext holds both literals, and the pinned validator
// answers `no viable alternative at input 'frame'` to `ref frame : F;`.
func TestFrameAndRenderNameKerMLFeatures(t *testing.T) {
	for _, input := range []string{"feature frame : SpatialFrame[1];", "feature render : Rendering;"} {
		t.Run(input, func(t *testing.T) {
			if codes := warningCodesIn(t, "modifier_kind.kerml", input); hasCode(codes, codeAmbiguousModifierKind) {
				t.Errorf("unexpected %s warning, warnings = %v", codeAmbiguousModifierKind, codes)
			}
		})
	}
}
