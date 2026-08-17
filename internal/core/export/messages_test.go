package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/export"
)

// TestUnsupportedConversionMessages pins the text of a conversion refusal: the
// position of the construct, its SysML name rather than a Go type, and the
// remedy docs/reference/rdf-mapping.md documents.
func TestUnsupportedConversionMessages(t *testing.T) {
	const remedy = "save to .sysml or .kerml instead, which writes the source exactly; " +
		"see docs/reference/rdf-mapping.md § Limitations"

	cases := []struct {
		name string
		src  string
		want []string
	}{{
		name: "metadata_prefix",
		src:  "package P {\n\tmetadata def M;\n\tpart p {@M{isSet = true;}}\n}",
		want: []string{"cannot convert the prefix metadata at m.sysml:3:10", remedy},
	}, {
		// The parser records an `@` annotation on the declaration ahead of the one
		// it prefixes, so writing it back would annotate a different element.
		name: "misplaced_annotation",
		src:  "package P {\n\tmetadata def M;\n\t@M part def Car;\n}",
		want: []string{"cannot convert the metadata annotation at m.sysml:3:3", "would be a different model", remedy},
	}, {
		// The entry member is unnamed, so the `then` beside it sequences an end
		// no reference can name.
		name: "succession_with_an_unnamed_end",
		src:  "package P {\n\tstate def M {\n\t\tentry; then s1;\n\t\tstate s1;\n\t}\n}",
		want: []string{"cannot convert the succession at m.sysml:3:10", "does not name both of the members it sequences"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := export.Convert("m.sysml", []byte(tc.src), export.FormatSysML, export.FormatTurtle)
			if err == nil {
				t.Fatal("expected the conversion to be refused")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err.Error())
				}
			}
			if strings.Contains(err.Error(), "*ast.") {
				t.Errorf("error names a Go type: %s", err.Error())
			}
		})
	}
}

// TestFormatRemedyNamesBothSurfaces pins that a format that cannot be told from
// the name advises the flag of the surface asked and the extension remedy the
// other surface uses, so the two agree.
func TestFormatRemedyNamesBothSurfaces(t *testing.T) {
	_, err := export.FormatOfPath("model.txt")
	if err == nil {
		t.Fatal("expected an unknown-format error")
	}
	advised := export.Advise(err, "pass -from, or "+export.ExtensionAdvice)
	want := `cannot tell the format of "model.txt": expected .sysml, .kerml or .ttl, ` +
		"so pass -from, or name the file with a .sysml, .kerml or .ttl extension"
	if advised.Error() != want {
		t.Errorf("err = %q; want %q", advised.Error(), want)
	}
}
