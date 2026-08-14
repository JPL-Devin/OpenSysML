package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// checkModel states conditions that decide both ways, so a check's exit status
// is exercised rather than only its report.
const checkModel = `package Rover {
    part def Battery {
        attribute capacity;
        attribute charge;
    }

    part pack : Battery {
        attribute :>> capacity = 100.0;
        attribute :>> charge = 80.0;
        constraint notOvercharged { charge <= capacity }
    }

    constraint MassBudget { assert 180.0 <= 200.0; }
    constraint TooHeavy { assert 210.0 <= 200.0; }

    requirement PowerMargin {
        require 600.0 >= 450.0;
    }

    part def Lander {
        attribute verticalSpeed;
    }
    requirement def TouchdownRequirement {
        subject lander : Lander;
        attribute maxVerticalSpeed;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }
    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }
    part slowLander : Lander {
        attribute :>> verticalSpeed = 1.2;
    }
    part fastLander : Lander {
        attribute :>> verticalSpeed = 2.4;
    }
    part analysisContext {
        assert satisfy touchdown by slowLander;
        assert satisfy touchdown by fastLander;
    }
}
`

// runOutcome is what a build step sees of a model check.
type runOutcome struct {
	status         int
	stdout, stderr string
}

func (r runOutcome) output() string { return r.stdout + r.stderr }

// check runs the binary on a model written to a temporary file and reports the
// exit status without failing the test, since the status is what is under test.
func check(t *testing.T, binary, model string, args ...string) runOutcome {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, append(args, path)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		result.status = exit.ExitCode()
	default:
		t.Fatalf("%v: %v\n%s", args, err, result.output())
	}
	return result
}

func wantReport(t *testing.T, got runOutcome, status int, substrings ...string) {
	t.Helper()
	if got.status != status {
		t.Errorf("exit status = %d, want %d\n%s", got.status, status, got.output())
	}
	for _, want := range substrings {
		if !strings.Contains(got.output(), want) {
			t.Errorf("report is missing %q:\n%s", want, got.output())
		}
	}
}

// TestCheckExitStatus checks the contract a build step depends on: a model that
// holds succeeds, a verdict against it fails with 1, and a check that could not
// be made at all is distinguished as 2.
func TestCheckExitStatus(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::MassBudget", "-requirement", "Rover::PowerMargin"),
		0, "✓ Constraint Rover::MassBudget passed", "✓ Requirement Rover::PowerMargin satisfied")

	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::TooHeavy"),
		1, "✗ Constraint Rover::TooHeavy failed", "Assertion evaluated to false: 210.0 <= 200.0")

	// A verdict that failed is reported even when a later check holds.
	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::TooHeavy", "-constraint", "Rover::MassBudget"),
		1, "✗ Constraint Rover::TooHeavy failed", "✓ Constraint Rover::MassBudget passed")

	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::nosuch"),
		2, `symbol "Rover::nosuch" not found`)

	// Not deciding outranks a failed verdict: the model was not fully checked.
	wantReport(t, check(t, binary, checkModel, "-constraint", "Rover::TooHeavy", "-requirement", "nosuch"),
		2, "✗ Constraint Rover::TooHeavy failed", `symbol "nosuch" not found`)

	// A requirement whose subject nothing binds decided nothing about the model,
	// however the prompt words it, so it is not reported as a failure.
	wantReport(t, check(t, binary, checkModel, "-requirement", "Rover::touchdown"),
		2, "no value for feature lander")
}

// TestCheckSatisfyThroughCLI checks the requirement-traceability gate, both over
// the whole model and for one element's assertions.
func TestCheckSatisfyThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-satisfy"), 1,
		"✓ satisfy touchdown by slowLander holds",
		"✗ satisfy touchdown by fastLander fails")

	wantReport(t, check(t, binary, checkModel, "-satisfy=Rover::analysisContext"), 1,
		"✗ satisfy touchdown by fastLander fails")

	// An element stating no assertion decided nothing, so the check failed to run.
	wantReport(t, check(t, binary, checkModel, "-satisfy=Rover::touchdown"), 2,
		"no satisfaction assertion in Rover::touchdown")
}

// TestSatisfyCanBeTurnedOff checks that the off spelling of a flag declared
// boolean asks for no satisfaction check, rather than for an element named
// "false", so -satisfy=$on works in a script.
func TestSatisfyCanBeTurnedOff(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, checkModel, "-satisfy=false", "-constraint", "Rover::MassBudget")
	wantReport(t, got, 0, "✓ Constraint Rover::MassBudget passed")
	if strings.Contains(got.output(), "satisfy touchdown") {
		t.Errorf("-satisfy=false checked satisfaction anyway:\n%s", got.output())
	}
}

// TestCheckAgainstInstantiatedObject checks that -instantiate makes a following
// verdict be about that object, which is the only way a part's own constraint
// reaches concrete slot values.
func TestCheckAgainstInstantiatedObject(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-instantiate", "Rover::pack", "-constraint", "Rover::pack::notOvercharged"),
		0, "✓ Created instance of Rover::pack",
		"✓ Constraint Rover::pack::notOvercharged passed (on Rover::pack ID: 1)")

	wantReport(t, check(t, binary, checkModel, "-instantiate", "Rover::nosuch", "-constraint", "Rover::MassBudget"),
		2, `symbol "Rover::nosuch" not found`)
}

// TestCheckReportsVerdictsOnStdout checks that a verdict is a result on stdout,
// while a check that could not be made is reported on stderr like any other
// failure to run.
func TestCheckReportsVerdictsOnStdout(t *testing.T) {
	binary := buildCLI(t)

	failed := check(t, binary, checkModel, "-constraint", "Rover::TooHeavy")
	if !strings.Contains(failed.stdout, "✗ Constraint Rover::TooHeavy failed") {
		t.Errorf("a verdict was not reported on stdout:\n%s", failed.output())
	}

	unresolved := check(t, binary, checkModel, "-constraint", "nosuch")
	if !strings.Contains(unresolved.stderr, `symbol "nosuch" not found`) {
		t.Errorf("an unmade check was not reported on stderr:\n%s", unresolved.output())
	}
}

// TestCheckWithoutModelFile checks that a check asked for with nothing to check
// exits 2 rather than waiting at a prompt a build step cannot answer.
func TestCheckWithoutModelFile(t *testing.T) {
	binary := buildCLI(t)
	cmd := exec.Command(binary, "-constraint", "Rover::MassBudget")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v, want status 2\n%s", err, out)
	}
	if !strings.Contains(string(out), "no model to check") {
		t.Errorf("report does not say a model is needed:\n%s", out)
	}
}

// TestCheckFlagsAreOptional checks that the modes that existed before checking
// was added are unchanged: a bare expression still evaluates and succeeds.
func TestCheckFlagsAreOptional(t *testing.T) {
	binary := buildCLI(t)
	out, err := exec.Command(binary, "-e", "5 + 3").CombinedOutput()
	if err != nil {
		t.Fatalf("-e: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "8") {
		t.Errorf("-e did not evaluate:\n%s", out)
	}
}
