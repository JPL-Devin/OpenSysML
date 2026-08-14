package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Open-MBEE/Systemica/internal/repl"
)

// Exit statuses of a model check. A verdict the model decided is reported by
// status 1, which is a report about the model; anything that stopped the check
// from being made is status 2, the same status a misused flag exits with, since
// in neither case did the model answer.
const (
	exitHolds       = 0
	exitFailed      = 1
	exitUnevaluable = 2
)

// checks are the model checks named on the command line, in the order they are
// carried out: objects are created first, so a verdict is about them.
type checks struct {
	instantiate  stringSlice
	constraints  stringSlice
	requirements stringSlice
	satisfy      satisfyTargets
}

// requested reports whether any check was asked for, which is what puts the
// binary in checking mode rather than starting a prompt.
func (c *checks) requested() bool {
	return len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.satisfy) > 0
}

// satisfyTargets collects -satisfy values. The flag takes an optional value: a
// bare -satisfy evaluates every satisfaction assertion in the model, and
// -satisfy=<name> evaluates the ones the named element states. Go's flag package
// passes "true" for the valueless spelling, which no name can be mistaken for
// because `true` is a literal keyword rather than a declarable name.
type satisfyTargets []string

func (t *satisfyTargets) String() string { return fmt.Sprint([]string(*t)) }

// IsBoolFlag makes the value optional, so -satisfy alone is accepted.
func (t *satisfyTargets) IsBoolFlag() bool { return true }

func (t *satisfyTargets) Set(value string) error {
	if value == "true" {
		// Every assertion in the model, which CheckSatisfy names with "".
		*t = append(*t, "")
		return nil
	}
	*t = append(*t, value)
	return nil
}

// tookNoValue reports whether -satisfy was given without a value. A name written
// after it (`-satisfy Landing::touchdown`) is then a positional argument, i.e. a
// file to load, which is worth explaining when no such file exists.
func (t *satisfyTargets) tookNoValue() bool {
	for _, target := range *t {
		if target == "" {
			return true
		}
	}
	return false
}

// runChecks loads the model, creates the objects asked for and evaluates the
// checks named on the command line, reporting each verdict as it is decided. The
// result is the exit status: every verdict held, one of them failed, or a check
// could not be made at all.
func runChecks(files []string, exprs []string, c checks) int {
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "sysml: no model to check; name the file the checked elements are declared in")
		return exitUnevaluable
	}

	sess := newSession()

	for _, file := range files {
		output, _, err := sess.RunMeta("%load " + file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			if c.satisfy.tookNoValue() && !fileExists(file) {
				fmt.Fprintf(os.Stderr, "sysml: %s is read as a file to load; -satisfy takes a name as -satisfy=%s\n", file, file)
			}
			return exitUnevaluable
		}
		report(output)
	}

	for _, expr := range exprs {
		output, _, err := sess.RunMeta("%eval " + expr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			return exitUnevaluable
		}
		report(output)
	}

	// An object first: a constraint or requirement of a part is checked against
	// the object that carries it, and only an existing one can be.
	for _, name := range c.instantiate {
		output, err := sess.InstantiateNamed(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			return exitUnevaluable
		}
		report(output)
	}

	var verdicts []repl.Verdict
	decide := func(v repl.Verdict) {
		// A check that could not be made is not a result about the model, so it is
		// reported where the other failures to run are.
		if v.Status == repl.VerdictUnresolved {
			reportTo(os.Stderr, v.Lines)
		} else {
			report(v.Lines)
		}
		verdicts = append(verdicts, v)
	}
	for _, name := range c.constraints {
		decide(sess.CheckConstraint(name))
	}
	for _, name := range c.requirements {
		decide(sess.CheckRequirement(name))
	}
	for _, target := range c.satisfy {
		for _, v := range sess.CheckSatisfy(target) {
			decide(v)
		}
	}

	switch repl.WorstStatus(verdicts) {
	case repl.VerdictFails:
		return exitFailed
	case repl.VerdictUnresolved:
		return exitUnevaluable
	default:
		return exitHolds
	}
}

func report(lines []string) {
	reportTo(os.Stdout, lines)
}

func reportTo(w io.Writer, lines []string) {
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
