package model

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// filterWorkspace is a two-document workspace exercising both element-filter
// forms over the same model: a namespace whose `filter` member restricts what it
// re-exports, imports carrying their own filter clause, and a view exposing a
// filtered subtree. The second document references the filtered namespaces from
// outside, which is where the difference between "surfaced" and "hidden" shows.
const filterModelSource = `package Vehicles {
	private import ScalarValues::Boolean;

	metadata def Safety {
		attribute isMandatory : Boolean;
	}
	metadata def CrashSafety :> Safety;

	part vehicle {
		part seatBelt { @Safety{isMandatory = true;} }
		part airBag { @CrashSafety{isMandatory = false;} }
		part keylessEntry;
	}
}

package SafetyFeatures {
	public import Vehicles::vehicle::*;
	filter @Vehicles::Safety;
}

package FilteredImport {
	public import Vehicles::vehicle::*[@Vehicles::Safety];
}

package MandatorySafety {
	public import Vehicles::vehicle::*[@Vehicles::Safety and Vehicles::Safety::isMandatory == true];
}

package OnwardSafety {
	public import SafetyFeatures::*;
}

package FurtherOnward {
	public import OnwardSafety::*;
}

package Nested {
	package SafetyInner {
		public import Vehicles::vehicle::*;
		filter @Vehicles::Safety;
	}
}

package UnqualifiedFilter {
	public import Vehicles::*;
	public import Vehicles::vehicle::*;
	filter @Safety;
}`

const filterClientSource = `package Client {
	private import SafetyFeatures::*;

	part a :> seatBelt;
	part b :> airBag;
}`

// openFilterWorkspace opens the filter model plus one client document and
// returns the client's diagnostic messages.
func openFilterWorkspace(t *testing.T, client string) []string {
	t.Helper()
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	ws.Open("file:///client.sysml", []byte(client), 1)
	var msgs []string
	for _, d := range ws.Diagnostics("file:///client.sysml") {
		msgs = append(msgs, d.Message)
	}
	return msgs
}

func TestNamespaceFilterSurfacesAnnotatedMembers(t *testing.T) {
	if got := openFilterWorkspace(t, filterClientSource); len(got) != 0 {
		t.Fatalf("annotated members should stay visible through a filtered namespace, got %v", got)
	}
}

func TestNamespaceFilterHidesUnannotatedMember(t *testing.T) {
	client := `package Client {
		private import SafetyFeatures::*;
		part c :> keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("an unannotated member must not be visible through a filtered namespace, got %v", got)
	}
}

func TestFilteredImportHidesUnannotatedMember(t *testing.T) {
	client := `package Client {
		private import FilteredImport::*;
		part c :> keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filtered import must not bring in an unannotated element, got %v", got)
	}
}

// The reflective metadata types of the standard library classify an element by
// what it is, which is the form the OMG corpus filters views with.
func TestFilterOnReflectiveMetadataType(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("file:///reflective.sysml", []byte(`package Reflective {
		part def Wheel;
		part vehicle {
			part seatBelt;
		}
	}

	package ReflectiveUsages {
		public import Reflective::vehicle::*;
		filter @SysML::PartUsage;
	}

	package ReflectiveAll {
		public import Reflective::*[@SysML::PartUsage];
	}`), 1)
	for _, tc := range []struct {
		client  string
		wantErr string
	}{
		{"package C1 { private import ReflectiveUsages::*; part a :> seatBelt; }", ""},
		{"package C2 { private import ReflectiveAll::*; part a :> vehicle; }", ""},
		{"package C3 { private import ReflectiveAll::*; part a :> Wheel; }", "Wheel"},
	} {
		ws.Open("file:///client.sysml", []byte(tc.client), 1)
		var msgs []string
		for _, d := range ws.Diagnostics("file:///client.sysml") {
			msgs = append(msgs, d.Message)
		}
		switch {
		case tc.wantErr == "" && len(msgs) != 0:
			t.Fatalf("%s: unexpected diagnostics %v", tc.client, msgs)
		case tc.wantErr != "" && (len(msgs) != 1 || !strings.Contains(msgs[0], tc.wantErr)):
			t.Fatalf("%s: diagnostics %v, want one mentioning %q", tc.client, msgs, tc.wantErr)
		}
	}
}

func TestFilteredImportComparesAnnotationFeature(t *testing.T) {
	client := `package Client {
		private import MandatorySafety::*;
		part a :> seatBelt;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("the mandatory-safety element should be visible, got %v", got)
	}

	client = `package Client {
		private import MandatorySafety::*;
		part b :> airBag;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "airBag") {
		t.Fatalf("a non-mandatory element must not pass the feature comparison, got %v", got)
	}

	// The guard decides an unannotated element without reading a feature it has
	// no annotation to read from, so it is filtered out rather than kept.
	client = `package Client {
		private import MandatorySafety::*;
		part c :> keylessEntry;
	}`
	got = openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("an unannotated element must not pass a guarded filter, got %v", got)
	}
}

// A filtered element is gone from every route to it, not merely absent from the
// unqualified one: naming it under the filtering namespace does not resolve
// either.
func TestFilteredElementIsUnresolvableQualified(t *testing.T) {
	client := `package Client {
		part c :> SafetyFeatures::keylessEntry;
		part d :> FilteredImport::keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 2 {
		t.Fatalf("a filtered element must not resolve under a qualified name either, got %v", got)
	}
	for _, msg := range got {
		if !strings.Contains(msg, "keylessEntry") {
			t.Fatalf("diagnostics %v should both be about keylessEntry", got)
		}
	}

	// The elements the filters admit do resolve under the same qualified route,
	// so the restriction is the filter's verdict and not the route itself.
	client = `package Client {
		part a :> SafetyFeatures::seatBelt;
		part b :> FilteredImport::airBag;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("admitted elements should resolve qualified, got %v", got)
	}
}

// A filter keeps holding when a further namespace imports the filtering one
// onward: what the filtered namespace re-exports is what any chain of imports of
// it carries, however many hops long.
func TestNamespaceFilterHoldsThroughChainedImports(t *testing.T) {
	for _, route := range []string{"OnwardSafety", "FurtherOnward"} {
		client := `package Client {
			private import ` + route + `::*;
			part c :> keylessEntry;
			part d :> ` + route + `::keylessEntry;
		}`
		got := openFilterWorkspace(t, client)
		if len(got) != 2 {
			t.Fatalf("%s: a filtered element must stay hidden through an onward import, got %v", route, got)
		}

		client = `package Client {
			private import ` + route + `::*;
			part a :> seatBelt;
			part b :> ` + route + `::seatBelt;
		}`
		if got := openFilterWorkspace(t, client); len(got) != 0 {
			t.Fatalf("%s: an admitted element should still resolve onward, got %v", route, got)
		}
	}
}

// A qualified name reaches a filtering namespace at any depth, not only as the
// second segment of the name.
func TestFilterAppliesToDeeplyQualifiedRoute(t *testing.T) {
	client := `package Client {
		part c :> Nested::SafetyInner::keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filtered element must not resolve through a nested qualified route, got %v", got)
	}

	client = `package Client {
		part a :> Nested::SafetyInner::seatBelt;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("an admitted element should resolve through a nested qualified route, got %v", got)
	}
}

// The metadata type a condition names is resolved from the namespace the filter
// is written in, so an imported type may be named unqualified.
func TestFilterNamesMetadataTypeThroughAnImport(t *testing.T) {
	client := `package Client {
		private import UnqualifiedFilter::*;
		part c :> keylessEntry;
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filter naming an imported metadata type must apply, got %v", got)
	}

	client = `package Client {
		private import UnqualifiedFilter::*;
		part a :> seatBelt;
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("the annotated element should pass a filter naming an imported type, got %v", got)
	}

	// The condition's own names are not subject to the condition: a namespace's
	// filter restricts what it re-exports, not what resolves in its body.
	ws := NewWorkspace()
	ws.Open("file:///vehicles.sysml", []byte(filterModelSource), 1)
	for _, d := range ws.Diagnostics("file:///vehicles.sysml") {
		if strings.Contains(d.Message, "filter condition") {
			t.Fatalf("a filter naming an imported type should be evaluable, got %q", d.Message)
		}
	}
}

// A view's `expose` is an import, and its filter restricts what the view
// surfaces: the recursive form walks the subtree and keeps the matching subset.
func TestFilteredExposeSurfacesSubset(t *testing.T) {
	client := `package Client {
		view safetyView {
			expose Vehicles::vehicle::**[@Vehicles::Safety];
			part a :> seatBelt;
			part b :> airBag;
		}
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("a filtered expose should surface the matching elements, got %v", got)
	}

	client = `package Client {
		view safetyView {
			expose Vehicles::vehicle::**[@Vehicles::Safety];
			part c :> keylessEntry;
		}
	}`
	got := openFilterWorkspace(t, client)
	if len(got) != 1 || !strings.Contains(got[0], "keylessEntry") {
		t.Fatalf("a filtered expose must not surface a non-matching element, got %v", got)
	}

	// Unfiltered, the same expose surfaces the whole subtree, which is what makes
	// the filtered case a strict subset.
	client = `package Client {
		view allView {
			expose Vehicles::vehicle::**;
			part c :> keylessEntry;
		}
	}`
	if got := openFilterWorkspace(t, client); len(got) != 0 {
		t.Fatalf("an unfiltered expose should surface the whole subtree, got %v", got)
	}
}

// A library's filters decide the same way whether its records were parsed or
// restored from the symbol cache, so a workspace's second run over an unchanged
// library resolves exactly the names its first one did.
func TestFilteredLibraryIsCacheStateIndependent(t *testing.T) {
	libDir := t.TempDir()
	lib := `package Meta {
	metadata def Safety;
}

package Vehicles {
	part vehicle {
		part seatBelt { @Meta::Safety; }
		part keylessEntry;
	}
}

package SafeReexport {
	public import Vehicles::vehicle::*;
	filter @Meta::Safety;
}

package Nested {
	package SafeInner {
		public import Vehicles::vehicle::*;
		filter @Meta::Safety;
	}
}
`
	if err := os.WriteFile(filepath.Join(libDir, "lib.sysml"), []byte(lib), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", libDir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client := `package C {
		private import SafeReexport::*;
		part a :> keylessEntry;
		part b :> SafeReexport::keylessEntry;
		part c :> SafeReexport::seatBelt;
		part d :> Nested::SafeInner::keylessEntry;
		part e :> Nested::SafeInner::seatBelt;
	}`
	var first []string
	for run := 1; run <= 3; run++ { // run 1 parses the library, later runs restore it
		ws := NewWorkspace()
		ws.Open("file:///c.sysml", []byte(client), 1)
		var msgs []string
		for _, d := range ws.Diagnostics("file:///c.sysml") {
			msgs = append(msgs, d.Message)
		}
		if run == 1 {
			if len(msgs) != 3 {
				t.Fatalf("run 1: want the three routes to keylessEntry filtered out, got %v", msgs)
			}
			for _, msg := range msgs {
				if !strings.Contains(msg, "keylessEntry") {
					t.Fatalf("run 1: diagnostics %v should all be about keylessEntry", msgs)
				}
			}
			first = msgs
			continue
		}
		if !slices.Equal(msgs, first) {
			t.Fatalf("run %d over a cache-restored library reports %v, but %v when parsed", run, msgs, first)
		}
	}
}
