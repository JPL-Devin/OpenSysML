package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// metadataDiags analyzes src as KerML against the standard library and returns
// the findings about metadata annotations, with the source text each covers.
func metadataDiags(t *testing.T, src string) []metadataFinding {
	t.Helper()
	return metadataDiagsNamed(t, "<t>.kerml", src)
}

// metadataDiagsNamed is metadataDiags over a document of the given name, whose
// extension decides the language it is analyzed as.
func metadataDiagsNamed(t *testing.T, name, src string) []metadataFinding {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()

	var out []metadataFinding
	for _, d := range Analyze(name, root, nil, idx) {
		out = append(out, metadataFinding{
			Code: d.Code,
			Text: src[d.Span.Offset:d.Span.End()],
			Msg:  d.Message,
		})
	}
	return out
}

type metadataFinding struct {
	Code string
	Text string
	Msg  string
}

// findingsWithCode returns the findings carrying one code.
func findingsWithCode(found []metadataFinding, code string) []metadataFinding {
	var out []metadataFinding
	for _, f := range found {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

const metadataEvaluableSrc = `package P {
	metadata def A {
		feature x;
		feature y;
	}
	feature f { in p; }
	feature a {
		@A {
			x = ~3;
			y = 1 + 2;
		}
	}
}`

// A metadata feature is a model-level element, so a value it binds must be one
// the model alone decides (KerML §7.4.9).
func TestMetadataBodyValueMustBeModelLevelEvaluable(t *testing.T) {
	found := findingsWithCode(metadataDiags(t, metadataEvaluableSrc), "metadata-value-not-evaluable")
	if len(found) != 1 {
		t.Fatalf("want one inevaluable value, got %v", found)
	}
	if found[0].Text != "= ~3" {
		t.Errorf("reported %q, want the binding and value `= ~3`", found[0].Text)
	}
	if found[0].Msg != msgFilterNotEvaluable {
		t.Errorf("message %q, want %q", found[0].Msg, msgFilterNotEvaluable)
	}
}

func TestNestedMetadataBodyValueUsesBindingSpan(t *testing.T) {
	src := `package P {
		metadata def A {
			feature x { feature y; feature z; }
		}
		feature f { in p; }
		feature a {
			@A {
				x {
					y = ~3;
					z = 1 + 2;
				}
			}
		}
	}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	if len(found) != 1 || found[0].Text != "= ~3" {
		t.Fatalf("want one nested finding on `= ~3`, got %v", found)
	}
}

// A declaration in an annotation body restates a feature of the metadata type;
// one that restates nothing is reported where it is written.
func TestMetadataBodyFeatureMustRestateAnOwningTypeFeature(t *testing.T) {
	src := `package P {
	metadata def A { feature x; }
	feature a {
		@A {
			x = 1;
			bad;
		}
	}
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-owning-type-feature")
	if len(found) != 1 || strings.TrimSpace(found[0].Text) != "bad;" {
		t.Fatalf("want one finding on `bad;`, got %v", found)
	}
}

// The metaclass of an annotated element conforms to every type of the
// annotatedElement feature of the metadata type (KerML §8.3.4.9).
func TestMetadataAnnotatedElementMustConform(t *testing.T) {
	src := `package P {
	metaclass Only {
		:>> annotatedElement : KerML::Structure;
	}
	#Only struct Ok;
	#Only class Bad;
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-annotated-element")
	if len(found) != 1 {
		t.Fatalf("want one annotated-element finding, got %v", found)
	}
	if want := msgCannotAnnotate + "Class"; found[0].Msg != want {
		t.Errorf("message %q, want %q", found[0].Msg, want)
	}
}

// The annotatedElement feature is read through inheritance, redefinition and
// subsetting, and in the `about`, usage and prefix spellings alike.
func TestMetadataAnnotatedElementReadsEffectiveFeatures(t *testing.T) {
	src := `package P {
	metaclass M { :>> annotatedElement : KerML::Class; }
	metaclass Inherits :> M;
	metaclass Narrows :> M { :>> annotatedElement : KerML::Structure; }
	metaclass Either { :>> annotatedElement : KerML::Class; feature alt :> annotatedElement : KerML::Package; }
	metaclass Any;
	class C { feature f; }
	struct S;
	package Q;
	@M about C, C::f;
	metadata mm : Inherits about C::f;
	#Narrows class NC;
	#Narrows struct NS;
	@Either about Q, C, C::f;
	@Any about Q, C, C::f;
}`
	var got []string
	for _, f := range findingsWithCode(metadataDiags(t, src), "metadata-annotated-element") {
		got = append(got, strings.TrimSpace(f.Text)+" => "+f.Msg)
	}
	want := []string{
		"@M about C, C::f; => Cannot annotate Feature",
		"metadata mm : Inherits about C::f; => Cannot annotate Feature",
		"#Narrows => Cannot annotate Class",
		"@Either about Q, C, C::f; => Cannot annotate Feature",
		"@Either about Q, C, C::f; => Cannot annotate Package",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The SysML spelling reads the same feature, redefined to a SysML metaclass.
func TestMetadataAnnotatedElementInSysML(t *testing.T) {
	src := `package P {
	metadata def M { :>> annotatedElement : SysML::PartUsage; }
	part def PD;
	part p : PD;
	attribute a;
	#M part q : PD;
	@M about p;
	metadata m : M about p, a;
	#M attribute b;
}`
	var got []string
	for _, d := range only(w8cLibraryDiagnostics(t, "meta-annotated.sysml", src), "metadata-annotated-element") {
		got = append(got, strings.TrimSpace(src[d.Span.Offset:d.Span.End()])+" => "+d.Message)
	}
	want := []string{
		"metadata m : M about p, a; => Cannot annotate AttributeUsage",
		"#M => Cannot annotate AttributeUsage",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An explicit `:>>` in a body, nested or not, names a feature of the metadata
// type or of a type it specializes.
func TestMetadataBodyRedefinitionMustNameAnOwningTypeFeature(t *testing.T) {
	src := `package P {
	feature g;
	metaclass Base { feature inherited; }
	metaclass M :> Base {
		feature x;
		feature u { feature v; }
	}
	class C { feature own; }
	@M about C { :>> g; }
	@M about C { :>> C::own; }
	@M about C { :>> x; :>> inherited; u { :>> v; } }
	@M about C { u { :>> g; } }
}`
	var got []string
	for _, f := range findingsWithCode(metadataDiags(t, src), "metadata-owning-type-feature") {
		got = append(got, strings.TrimSpace(f.Text))
	}
	want := []string{":>> g;", ":>> C::own;", ":>> g;"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings %q, want %q", got, want)
	}
}

// A subject, assume or require member annotated by a prefix is checked against
// the annotatedElement of the metadata definition the way a part is: a usage-
// only definition accepts all four, a definition-only one rejects all four.
func TestMetadataAnnotatedElementOnRequirementMembers(t *testing.T) {
	const body = `package P {
	metadata def OnUsage { :>> annotatedElement : SysML::Usage; }
	metadata def OnDef { :>> annotatedElement : SysML::Definition; }
	part def Vehicle;
	constraint def C;
	requirement def R {
		subject #%[1]s v : Vehicle;
		assume #%[1]s constraint a : C;
		require #%[1]s constraint r : C;
		#%[1]s part p : Vehicle;
		assume #%[1]s constraint { true }
		require #%[1]s constraint : C { true }
	}
}`
	ok := metadataDiagsNamed(t, "<t>.sysml", fmt.Sprintf(body, "OnUsage"))
	if found := findingsWithCode(ok, "metadata-annotated-element"); len(found) != 0 {
		t.Errorf("usage-only metadata on usages: unexpected findings %v", found)
	}

	bad := findingsWithCode(metadataDiagsNamed(t, "<t>.sysml", fmt.Sprintf(body, "OnDef")), "metadata-annotated-element")
	want := []string{
		msgCannotAnnotate + "PartUsage",
		msgCannotAnnotate + "ConstraintUsage",
		msgCannotAnnotate + "ConstraintUsage",
		msgCannotAnnotate + "PartUsage",
		msgCannotAnnotate + "ConstraintUsage",
		msgCannotAnnotate + "ConstraintUsage",
	}
	if len(bad) != len(want) {
		t.Fatalf("definition-only metadata on usages: findings %v, want %d", bad, len(want))
	}
	for i, f := range bad {
		if f.Msg != want[i] || strings.TrimSpace(f.Text) != "#OnDef" {
			t.Errorf("finding %d = %q on %q, want %q on #OnDef", i, f.Msg, f.Text, want[i])
		}
	}
}

// A metadata value calling an overloaded name is judged by the overload its
// arguments select — the checker's and runtime's choice — not the first found.
func TestMetadataBodyValueSelectsTheOverloadItCalls(t *testing.T) {
	const body = `package P {
	private import ScalarValues::*;
	private import %s::*;
	private import %s::*;
	package Q {
		private import ScalarValues::*;
		function 'if' { in test : Boolean; in t : String; in f : String; return : String; }
	}
	metadata def A { feature x; feature y; }
	feature a {
		@A {
			x = 'if'(true, 1, 2);
			y = 'if'(true, "a", "b");
		}
	}
}`
	for _, imports := range [][2]string{{"ControlFunctions", "Q"}, {"Q", "ControlFunctions"}} {
		src := fmt.Sprintf(body, imports[0], imports[1])
		found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
		if len(found) != 1 || found[0].Text != `= 'if'(true, "a", "b")` {
			t.Errorf("imports %v: findings %v, want only the call selecting Q::'if'", imports, found)
		}
	}
}

// A body value is judged in the body's own scope, where the metadata type's
// members shadow what the annotated element sees: the call and the read below
// name A's own function and feature, not the imported one and the unfoldable one.
func TestMetadataBodyValueIsJudgedInTheBodyScope(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	feature k = ~3;
	metadata def A {
		feature x;
		feature y;
		feature k;
		function 'if' { in test : Boolean; in t : Integer; in f : Integer; return : Integer; }
	}
	feature a {
		@A {
			x = 'if'(true, 1, 2);
			y = k;
		}
	}
}`
	found := findingsWithCode(metadataDiags(t, src), "metadata-value-not-evaluable")
	want := []string{"= 'if'(true, 1, 2)"}
	if len(found) != len(want) {
		t.Fatalf("findings %v, want %v", found, want)
	}
	for i, f := range found {
		if f.Text != want[i] {
			t.Errorf("finding %d is %q, want %q", i, f.Text, want[i])
		}
	}
}

// A body with no fault draws nothing: a value the model folds and a feature that
// restates one of the metadata type are both legal.
func TestMetadataBodyWithoutFaultsIsSilent(t *testing.T) {
	src := `package P {
	metadata def A { feature x; }
	feature a {
		@A { x = 1 + 2; }
	}
}`
	for _, f := range metadataDiags(t, src) {
		switch f.Code {
		case "metadata-value-not-evaluable", "metadata-owning-type-feature", "metadata-annotated-element":
			t.Errorf("unexpected finding %v", f)
		}
	}
}
