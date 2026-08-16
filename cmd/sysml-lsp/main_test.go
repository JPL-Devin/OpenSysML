package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCommandLine checks what the server does with a command line before it
// speaks the protocol: every spelling of the version flag is answered, and
// anything it could not read is a usage on stderr rather than protocol mode.
func TestCommandLine(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		status int
		stdout []string
		stderr []string
	}{{
		name:   "-version reports the version",
		args:   []string{"-version"},
		status: exitServed,
		stdout: []string{"sysml-lsp ", "Commit:", "Build time:", "Go version:"},
	}, {
		name:   "--version reports the version",
		args:   []string{"--version"},
		status: exitServed,
		stdout: []string{"sysml-lsp "},
	}, {
		name:   "-v reports the version",
		args:   []string{"-v"},
		status: exitServed,
		stdout: []string{"sysml-lsp "},
	}, {
		name:   "the help asked for is a result on stdout",
		args:   []string{"-h"},
		status: exitServed,
		stdout: []string{"Usage: sysml-lsp [options]", "-version", "-stdio"},
	}, {
		name:   "a flag that is not defined is reported with the usage",
		args:   []string{"-nosuchflag"},
		status: exitUnservable,
		stderr: []string{"flag provided but not defined: -nosuchflag", "Usage: sysml-lsp [options]"},
	}, {
		name:   "an argument that is not a flag is reported with the usage",
		args:   []string{"model.sysml"},
		status: exitUnservable,
		stderr: []string{`sysml-lsp: unexpected argument "model.sysml"`, "Usage: sysml-lsp [options]"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := run(tc.args, &stdout, &stderr)
			if status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s%s", status, tc.status, stdout.String(), stderr.String())
			}
			for _, want := range tc.stdout {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout is missing %q:\n%s", want, stdout.String())
				}
				if strings.Contains(stderr.String(), want) {
					t.Errorf("%q was reported on stderr:\n%s", want, stderr.String())
				}
			}
			for _, want := range tc.stderr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr is missing %q:\n%s", want, stderr.String())
				}
				if strings.Contains(stdout.String(), want) {
					t.Errorf("%q was reported on stdout:\n%s", want, stdout.String())
				}
			}
		})
	}
}
