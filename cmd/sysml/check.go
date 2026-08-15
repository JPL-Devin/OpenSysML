package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/repl"
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
	advance      advanceTime
	jsonOut      bool
}

// advanceTime is -advance as written, parsed where a bad value is reported in
// whichever form the caller asked for. An empty value is a misuse rather than
// silently no advance, so it records that the flag was written.
type advanceTime struct {
	value string
	given bool
}

func (a *advanceTime) String() string { return a.value }

func (a *advanceTime) Set(value string) error {
	a.value, a.given = value, true
	return nil
}

// requested reports whether checking mode was asked for rather than a prompt.
// -json and -advance check nothing themselves, but are included so their misuse
// is reported rather than leaving a script at a prompt it cannot answer.
func (c *checks) requested() bool {
	return c.validate || c.jsonOut || c.advance.given || c.satisfy.given || len(c.instantiate) > 0 ||
		len(c.constraints) > 0 || len(c.requirements) > 0 || len(c.calcs) > 0 ||
		len(c.actions) > 0 || len(c.states) > 0
}

// checksOnly reports whether anything was asked about the model itself, as
// against how to report the answer.
func (c *checks) checksOnly() bool {
	return c.validate || len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.satisfy.targets) > 0 || len(c.calcs) > 0 ||
		len(c.actions) > 0 || len(c.states) > 0
}

// satisfyTargets collects -satisfy values. The flag takes an optional value: a
// bare -satisfy evaluates every satisfaction assertion in the model, and
// -satisfy=<name> evaluates the ones the named element states. Go's flag package
// passes "true" for the valueless spelling, which no name can be mistaken for
// because `true` is a literal keyword rather than a declarable name.
type satisfyTargets struct {
	targets []string
	given   bool
}

func (t *satisfyTargets) String() string { return fmt.Sprint(t.targets) }

// IsBoolFlag makes the value optional, so -satisfy alone is accepted.
func (t *satisfyTargets) IsBoolFlag() bool { return true }

func (t *satisfyTargets) Set(value string) error {
	t.given = true
	switch value {
	case "true":
		// Every assertion in the model, which CheckSatisfy names with "".
		t.targets = append(t.targets, "")
	case "false":
		// The off spelling of a flag declared boolean, so -satisfy=$on works.
	default:
		t.targets = append(t.targets, value)
	}
	return nil
}

// tookNoValue reports whether -satisfy was given without a value. A name written
// after it (`-satisfy Landing::touchdown`) is then a positional argument, i.e. a
// file to load, which is worth explaining when no such file exists.
func (t *satisfyTargets) tookNoValue() bool {
	for _, target := range t.targets {
		if target == "" {
			return true
		}
	}
	return false
}

// refuse reports a misused flag in whichever form the caller asked for, so a
// script reading the JSON document reads why no check was made.
func refuse(c checks, message string) int {
	rep := newReporter(c.jsonOut)
	rep.failed(message)
	return rep.finish()
}

// runChecks loads the model, creates the objects asked for, evaluates the checks
// and runs the behavior named on the command line, reporting each outcome as it
// is decided. The result is the exit status: every verdict held, one of them
// failed, or a check could not be made at all.
func runChecks(files []string, exprs []string, c checks) int {
	rep := newReporter(c.jsonOut)

	var advance float64
	if c.advance.given {
		if len(c.states) == 0 {
			rep.failed("-advance is the time a state machine runs for; name one, as -state <name>")
			return rep.finish()
		}
		duration, err := parseAdvance(c.advance.value)
		if err != nil {
			rep.failed(err.Error())
			return rep.finish()
		}
		advance = duration
	}
	if !c.checksOnly() {
		if c.jsonOut {
			rep.failed("-json reports a check; name one, as -validate or -constraint <name>")
		} else {
			rep.failed("no check was named; name one, as -validate or -constraint <name>")
		}
		return rep.finish()
	}
	if len(files) == 0 {
		rep.failed("no model to check; name the file the checked elements are declared in")
		return rep.finish()
	}

	sess := newSession()

	// A checked model may be named as a directory or a glob as well as by file, so
	// the paths are expanded to the files they stand for before loading.
	paths, err := repl.ExpandPaths(files)
	if err != nil {
		rep.failed(err.Error())
		if c.satisfy.tookNoValue() && len(files) == 1 && !fileExists(files[0]) {
			rep.failed(fmt.Sprintf("%s is read as a file to load; -satisfy takes a name as -satisfy=%s", files[0], files[0]))
		}
		return rep.finish()
	}

	loaded := make([][]string, 0, len(paths))
	for _, file := range paths {
		output, err := sess.LoadFileSummary(file)
		if err != nil {
			rep.failed(err.Error())
			if c.satisfy.tookNoValue() && !fileExists(file) {
				rep.failed(fmt.Sprintf("%s is read as a file to load; -satisfy takes a name as -satisfy=%s", file, file))
			}
			return rep.finish()
		}
		loaded = append(loaded, output)
	}

	// A model that did not analyse cleanly answers nothing, so its diagnostics end
	// the run rather than a verdict being reported about a model nobody could read.
	// One file's reference to another only resolves once every file is loaded, so
	// the gate is about the whole model rather than about each file in turn.
	if sess.HasErrors() {
		rep.diags(sess.LocatedDiagnostics())
		rep.problem(sess.DiagnosticLines())
		rep.failed(fmt.Sprintf("%s did not analyse cleanly; no check was made", namedModels(files)))
		return rep.finish()
	}
	for _, output := range loaded {
		rep.info(output)
	}
	// A clean model's warnings are still findings rather than results, so they
	// are kept off the stream the verdicts are reported on.
	rep.problem(sess.DiagnosticLines())

	// What analysis found is reported as data whatever was checked, so a caller
	// parsing the report reads the warnings the printed load output carries.
	rep.diags(sess.LocatedDiagnostics())
	if c.validate {
		rep.info([]string{fmt.Sprintf("✓ %s: no errors", namedModels(files))})
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
	for _, target := range c.satisfy.targets {
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
		if c.advance.given {
			rep.verdict(sess.RunStateMachineFor(name, advance, performer...))
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
