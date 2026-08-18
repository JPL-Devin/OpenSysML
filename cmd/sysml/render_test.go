package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renderModel declares a view stating no rendering, one exposing nothing, and a
// part def to ask for by mistake.
const renderModel = `package Demo {
    part def Vehicle { part wheel : Wheel; }
    part def Wheel;
    view overview { expose Demo::Vehicle; }
    view empty;
}
`

// The rendering is the run's result on stdout, and everything about the run —
// what was loaded, what the rendering could not show — is on stderr.
func TestRenderWritesTheArtifactOnStdout(t *testing.T) {
	binary := buildCLI(t)

	got := runStreams(t, binary, renderModel, "-render", "Demo::overview")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"flowchart TD", "part def Demo::Vehicle"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	if strings.Contains(got.stdout, "✓ package Demo") {
		t.Errorf("the load report landed in the artifact:\n%s", got.stdout)
	}
}

func TestRenderTextFormAndOutputFile(t *testing.T) {
	binary := buildCLI(t)

	out := filepath.Join(t.TempDir(), "view.txt")
	got := runStreams(t, binary, renderModel, "-render", "Demo::overview", "-render-form", "text", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stderr, "wrote "+out) {
		t.Errorf("stderr should name the file written, got:\n%s", got.stderr)
	}
	written, err := os.ReadFile(out) // #nosec G304 -- the test wrote this path.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "Demo::overview — tree rendering") {
		t.Errorf("the rendering is missing from the file:\n%s", written)
	}
	if strings.Contains(string(written), "wrote ") {
		t.Errorf("the file carries a line about the run:\n%s", written)
	}
}

func TestRenderReportsWhatItCouldNotDo(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name   string
		args   []string
		status int
		stderr []string
	}{{
		name:   "a view exposing nothing renders empty and says so",
		args:   []string{"-render", "Demo::empty"},
		status: exitHolds,
		stderr: []string{"note: Demo::empty renders empty"},
	}, {
		name:   "a name that is no view is reported under the command's prefix",
		args:   []string{"-render", "Demo::Vehicle"},
		status: exitUnevaluable,
		stderr: []string{"sysml: ", "not a view"},
	}, {
		name:   "an unknown name is reported",
		args:   []string{"-render", "Demo::Nope"},
		status: exitUnevaluable,
		stderr: []string{"sysml: "},
	}, {
		name:   "a form that is not a form is reported",
		args:   []string{"-render", "Demo::overview", "-render-form", "dot"},
		status: exitUnevaluable,
		stderr: []string{"unknown rendering form \"dot\""},
	}, {
		name:   "a form without a view to render is reported",
		args:   []string{"-render-form", "text"},
		status: exitUnevaluable,
		stderr: []string{"name the view to render with -render"},
	}, {
		name:   "rendering and converting in one run is refused",
		args:   []string{"-render", "Demo::overview", "-convert", "ttl"},
		status: exitUnevaluable,
		stderr: []string{"ask for one per run"},
	}, {
		name:   "rendering decides nothing, so a check may not be asked for with it",
		args:   []string{"-render", "Demo::overview", "-validate"},
		status: exitUnevaluable,
		stderr: []string{"check it in its own run"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStreams(t, binary, renderModel, tc.args...)
			if got.status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s", got.status, tc.status, got.output())
			}
			for _, want := range tc.stderr {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr is missing %q:\n%s", want, got.stderr)
				}
			}
		})
	}
}
