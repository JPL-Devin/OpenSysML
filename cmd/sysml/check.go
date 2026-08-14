package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
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

// checks are the model checks and runs named on the command line, in the order
// they are carried out: objects are created first, so a verdict is about them,
// and behavior runs after the conditions the model states about it.
type checks struct {
	validate     bool
	instantiate  stringSlice
	constraints  stringSlice
	requirements stringSlice
	satisfy      satisfyTargets
	calcs        stringSlice
	actions      stringSlice
	states       stringSlice
	advance      float64
	advanceGiven bool
	jsonOut      bool
}

// requested reports whether checking mode was asked for rather than a prompt.
// -json and -advance check nothing themselves, but are included so their misuse
// is reported rather than leaving a script at a prompt it cannot answer.
func (c *checks) requested() bool {
	return c.validate || c.jsonOut || c.advanceGiven || len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.satisfy) > 0 || len(c.calcs) > 0 ||
		len(c.actions) > 0 || len(c.states) > 0
}

// checksOnly reports whether anything was asked about the model itself, as
// against how to report the answer.
func (c *checks) checksOnly() bool {
	return c.validate || len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.satisfy) > 0 || len(c.calcs) > 0 ||
		len(c.actions) > 0 || len(c.states) > 0
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

// runChecks loads the model, creates the objects asked for, evaluates the checks
// and runs the behavior named on the command line, reporting each outcome as it
// is decided. The result is the exit status: every verdict held, one of them
// failed, or a check could not be made at all.
func runChecks(files []string, exprs []string, c checks) int {
	rep := newReporter(c.jsonOut)

	if c.advanceGiven && len(c.states) == 0 {
		rep.failed("-advance is the time a state machine runs for; name one, as -state <name>")
		return rep.finish()
	}
	if !c.checksOnly() {
		rep.failed("-json reports a check; name one, as -validate or -constraint <name>")
		return rep.finish()
	}
	if len(files) == 0 {
		rep.failed("no model to check; name the file the checked elements are declared in")
		return rep.finish()
	}

	sess := newSession()

	// A model that did not analyse cleanly answers nothing, so its diagnostics end
	// the run rather than a verdict being reported about a model nobody could read.
	for _, file := range files {
		output, err := sess.LoadFile(file)
		if err != nil {
			rep.failed(err.Error())
			if c.satisfy.tookNoValue() && !fileExists(file) {
				rep.failed(fmt.Sprintf("%s is read as a file to load; -satisfy takes a name as -satisfy=%s", file, file))
			}
			return rep.finish()
		}
		if sess.HasErrors() {
			rep.diags(sess.LocatedDiagnostics())
			rep.problem(output)
			rep.failed(fmt.Sprintf("%s did not analyse cleanly; no check was made", file))
			return rep.finish()
		}
		rep.info(output)
	}

	// What analysis found is reported as data whatever was checked, so a caller
	// parsing the report reads the warnings the printed load output carries.
	rep.diags(sess.LocatedDiagnostics())
	if c.validate {
		rep.info([]string{fmt.Sprintf("✓ %s: no errors", strings.Join(files, ", "))})
	}

	for _, expr := range exprs {
		output, err := sess.EvalExpr(expr)
		if err != nil {
			rep.failed(fmt.Sprintf("%s: %v", expr, err))
			return rep.finish()
		}
		rep.info(output)
	}

	// An object first: a constraint or requirement of a part is checked against
	// the object that carries it, and only an existing one can be.
	for _, name := range c.instantiate {
		output, err := sess.InstantiateNamed(name)
		if err != nil {
			rep.failed(err.Error())
			return rep.finish()
		}
		rep.info(output)
	}

	for _, name := range c.constraints {
		rep.verdict(sess.CheckConstraint(name))
	}
	for _, name := range c.requirements {
		rep.verdict(sess.CheckRequirement(name))
	}
	for _, target := range c.satisfy {
		for _, v := range sess.CheckSatisfy(target) {
			rep.verdict(v)
		}
	}
	for _, invocation := range c.calcs {
		rep.verdict(sess.RunCalc(invocation))
	}
	for _, value := range c.actions {
		name, performer := splitPerformer(value)
		rep.verdict(sess.RunAction(name, performer...))
	}
	for _, value := range c.states {
		name, performer := splitPerformer(value)
		if c.advanceGiven {
			rep.verdict(sess.RunStateMachineFor(name, c.advance, performer...))
			continue
		}
		rep.verdict(sess.RunStateMachine(name, performer...))
	}

	return rep.finish()
}

// splitPerformer splits a `-action`/`-state` value into the behavior's name and
// the object performing it, which is the word after it as `%action` takes it:
// `-action "Drive rover1"`.
func splitPerformer(value string) (string, []string) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseAdvance reads the -advance value, which is the simulated time a state
// machine is run for.
func parseAdvance(value string) (float64, error) {
	duration, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("-advance takes a number of time units, not %q", value)
	}
	if math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, fmt.Errorf("-advance takes a duration to run for, and %q is not one", value)
	}
	if duration < 0 {
		return 0, fmt.Errorf("-advance takes a duration to run for, and %v runs backwards", duration)
	}
	return duration, nil
}
