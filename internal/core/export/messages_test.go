package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// TestUnsupportedConversionMessages pins the text of a conversion refusal: the
// position of the construct, its SysML name rather than a Go type, and the
// reason it has no place in the graph.
func TestUnsupportedConversionMessages(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{{
		// `event e;` refers to e rather than declaring an element of its own.
		name: "event_reference",
		src:  "package P {\n\tpart def A {\n\t\tevent e;\n\t}\n}",
		want: []string{"cannot convert the `event` declaration at m.sysml:3:3",
			"would come back as `occurrence`, a different declaration"},
	}, {
		name: "duplicate_declaration",
		src:  "package P {\n\tpart def A {\n\t\tattribute x : Real;\n\t\tattribute x : Real;\n\t}\n}",
		want: []string{"cannot convert the duplicate declaration of \"x\" at m.sysml:4:3",
			"two members of one namespace cannot share it"},
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
