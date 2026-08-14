// Command sysml is an interactive SysML v2 REPL (spec §13).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/Systemica/internal/core/export"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/repl"
)

var (
	// Version information - set via ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

type rlReader struct{ rl *readline.Instance }

func (r *rlReader) ReadLine(prompt string) (string, error) {
	r.rl.SetPrompt(prompt)
	line, err := r.rl.Readline()
	if err == readline.ErrInterrupt { // Ctrl-C clears line (continue REPL)
		return "", nil
	}
	if err == io.EOF { // Ctrl-D exits REPL
		return "", io.EOF
	}
	return line, err
}

// CLI flags
var (
	evalExprs   stringSlice
	showVersion bool
	debugMode   bool
	quietMode   bool
	traceMode   bool
	convertPath string
	outputPath  string
	fromFormat  string
	toFormat    string
	modelChecks checks
	advanceTime string
)

// budgets holds the run bounds the environment resolves to, read once at startup.
var budgets = runtime.DefaultBudgets()

// stringSlice is a custom flag type for multiple values
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sysml [options] [file...]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  sysml                     # Start interactive REPL\n")
		fmt.Fprintf(os.Stderr, "  sysml -e \"5 + 3\"          # Evaluate and exit\n")
		fmt.Fprintf(os.Stderr, "  sysml -e \"expr\" file.sysml # Load file, evaluate, and exit\n")
		fmt.Fprintf(os.Stderr, "  sysml file.sysml          # Load file and start REPL\n")
		fmt.Fprintf(os.Stderr, "  sysml -debug file.sysml   # Load file, reporting every diagnostic\n")
		fmt.Fprintf(os.Stderr, "  sysml -trace file.sysml   # Load file, reporting each execution step\n")
		fmt.Fprintf(os.Stderr, "\nChecking a model:\n")
		fmt.Fprintf(os.Stderr, "  sysml -constraint MassBudget model.sysml       # Evaluate one constraint and exit\n")
		fmt.Fprintf(os.Stderr, "  sysml -requirement PowerMargin model.sysml     # Evaluate one requirement and exit\n")
		fmt.Fprintf(os.Stderr, "  sysml -satisfy model.sysml                     # Evaluate every satisfaction assertion\n")
		fmt.Fprintf(os.Stderr, "  sysml -satisfy=Ctx model.sysml                 # ...only the ones Ctx states\n")
		fmt.Fprintf(os.Stderr, "  sysml -instantiate p -constraint C model.sysml  # Check C against an object of p\n")
		fmt.Fprintf(os.Stderr, "  sysml -validate model.sysml                    # Report diagnostics only\n")
		fmt.Fprintf(os.Stderr, "  sysml -calc \"Fall(3, 4)\" model.sysml           # Invoke a calculation\n")
		fmt.Fprintf(os.Stderr, "  sysml -action Drive model.sysml                # Run an action to completion\n")
		fmt.Fprintf(os.Stderr, "  sysml -state Mission -advance 10 model.sysml   # Run a state machine for 10 time units\n")
		fmt.Fprintf(os.Stderr, "  sysml -satisfy -json model.sysml               # Report the verdicts as JSON\n")
		fmt.Fprintf(os.Stderr, "\nA check reports its verdict and exits 0 when every verdict holds, 1 when one\n")
		fmt.Fprintf(os.Stderr, "fails, and 2 when a check could not be made at all — an unresolved name, a\n")
		fmt.Fprintf(os.Stderr, "model that did not analyse cleanly — so a model check can gate a build. Each\n")
		fmt.Fprintf(os.Stderr, "flag may be repeated.\n")
		fmt.Fprintf(os.Stderr, "\nConversion:\n")
		fmt.Fprintf(os.Stderr, "  sysml -convert model.sysml -o model.ttl    # SysML notation to RDF Turtle\n")
		fmt.Fprintf(os.Stderr, "  sysml -convert model.ttl -o model.sysml    # RDF Turtle to SysML notation\n")
		fmt.Fprintf(os.Stderr, "  sysml -convert model.sysml                 # Write the conversion to stdout\n")
		fmt.Fprintf(os.Stderr, "  sysml -convert in.txt -from sysml -to ttl   # Name the formats explicitly\n")
		fmt.Fprintf(os.Stderr, "\nThe format is taken from the file extension (.sysml, .kerml, .ttl) unless\n")
		fmt.Fprintf(os.Stderr, "-from/-to name it. Converting to the same format rewrites the input:\n")
		fmt.Fprintf(os.Stderr, "notation is reformatted, Turtle is normalized.\n")
	}

	flag.Var(&evalExprs, "eval", "Evaluate expression and exit (can be specified multiple times)")
	flag.Var(&evalExprs, "e", "Evaluate expression and exit (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	flag.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	flag.BoolVar(&traceMode, "trace", false, "Report each execution step: expression evaluation, calc invocation, action tokens, state transitions")
	flag.StringVar(&convertPath, "convert", "", "Convert this model between SysML notation and RDF Turtle instead of running it")
	flag.StringVar(&outputPath, "output", "", "Write conversion output to this file (default: stdout)")
	flag.StringVar(&outputPath, "o", "", "Write conversion output to this file (shorthand)")
	flag.StringVar(&fromFormat, "from", "", "Input format for -convert: sysml, kerml, ttl, turtle or rdf (default: from the input's extension)")
	flag.StringVar(&toFormat, "to", "", "Output format for -convert: sysml, kerml, ttl, turtle or rdf (default: from the output's extension)")
	flag.Var(&modelChecks.instantiate, "instantiate", "Create an object of this definition before the checks, so a verdict is about it (repeatable)")
	flag.Var(&modelChecks.constraints, "constraint", "Evaluate this constraint and exit (repeatable)")
	flag.Var(&modelChecks.requirements, "requirement", "Evaluate this requirement and exit (repeatable)")
	flag.Var(&modelChecks.satisfy, "satisfy", "Evaluate every satisfaction assertion, or with -satisfy=<name> those the named element states (repeatable)")
	flag.BoolVar(&modelChecks.validate, "validate", false, "Analyse the model and report its diagnostics, exiting nonzero on an error")
	flag.Var(&modelChecks.calcs, "calc", "Invoke this calculation and report what it computed, as -calc \"Fall(3, 4)\" (repeatable)")
	flag.Var(&modelChecks.actions, "action", "Run this action to completion, as -action \"Drive rover1\" to run it on an object (repeatable)")
	flag.Var(&modelChecks.states, "state", "Run this state machine, as -state \"Mission rover1\" to run it on an object (repeatable)")
	flag.StringVar(&advanceTime, "advance", "", "Simulated time units to run each -state machine for (default: only its initial transition)")
	flag.BoolVar(&modelChecks.jsonOut, "json", false, "Report checks as one JSON document rather than as lines")
	flag.Parse()

	if advanceTime != "" {
		duration, err := parseAdvance(advanceTime)
		if err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			os.Exit(2)
		}
		modelChecks.advance = duration
		modelChecks.advanceGiven = true
	}

	if debugMode && quietMode {
		fmt.Fprintln(os.Stderr, "sysml: -debug and -quiet are mutually exclusive")
		os.Exit(2)
	}

	// Handle version flag
	if showVersion {
		fmt.Printf("sysml %s\n", Version)
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		os.Exit(0)
	}

	// Get positional arguments (files to load)
	args := flag.Args()

	if convertPath != "" {
		if err := runConvert(convertPath, args); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			os.Exit(1)
		}
		return
	}

	// Resolve the run bounds before any model runs, so a bad value is reported at
	// startup rather than mistaken for the default at execution time. Reporting the
	// version and converting a model evaluate nothing, so they are handled above.
	var err error
	budgets, err = runtime.BudgetsFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		os.Exit(2)
	}

	// Checking mode: load, check what was named, and exit on the verdict.
	if modelChecks.requested() {
		os.Exit(runChecks(args, evalExprs, modelChecks))
	}

	// Non-interactive mode: files + eval expressions, execute and exit
	if len(args) > 0 && len(evalExprs) > 0 {
		if err := runNonInteractive(args, evalExprs); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			os.Exit(1)
		}
		return
	}

	// Eval-only mode: just evaluate expressions and exit
	if len(evalExprs) > 0 {
		if err := runNonInteractive(nil, evalExprs); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			os.Exit(1)
		}
		return
	}

	if len(args) == 0 {
		// No files: interactive REPL
		if err := runInteractive(); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			os.Exit(1)
		}
		return
	}

	// Files provided: load and enter interactive mode
	if err := runInteractiveWithFiles(args); err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		os.Exit(1)
	}
}

func runInteractive() error {
	return runInteractiveWithFiles(nil)
}

// runConvert converts one model between SysML notation and RDF Turtle.
//
// Each format is taken from -from/-to when given and from the file extension
// otherwise. Writing to stdout leaves no extension to read, so -to is required
// there unless -from names a format to convert away from.
func runConvert(input string, rest []string) error {
	if len(rest) > 0 {
		return fmt.Errorf("-convert converts one file; unexpected extra argument %q", rest[0])
	}

	from, err := resolveFormat(fromFormat, input)
	if err != nil {
		return err
	}

	var to export.Format
	switch {
	case toFormat != "":
		to, err = export.ParseFormat(toFormat)
		if err != nil {
			return err
		}
	case outputPath != "":
		to, err = export.FormatOfPath(outputPath)
		if err != nil {
			return export.Advise(err, "pass -from/-to")
		}
	default:
		// No -to and no output file: convert to the other format, which is the
		// only unambiguous reading of "convert this".
		to = otherFormat(from)
	}

	// #nosec G304 -- the input file is the one named on the command line.
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	out, err := export.Convert(input, data, from, to)
	if err != nil {
		return err
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	replaced, err := export.WriteFile(outputPath, out)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", outputPath, to, len(out), what)
	return nil
}

// resolveFormat returns the format named by the flag, or the one the path's
// extension implies.
func resolveFormat(flagValue, path string) (export.Format, error) {
	if flagValue != "" {
		return export.ParseFormat(flagValue)
	}
	f, err := export.FormatOfPath(path)
	return f, export.Advise(err, "pass -from/-to")
}

// otherFormat returns the format to convert to when only the input format is
// known: the other one of the pair.
func otherFormat(from export.Format) export.Format {
	if from == export.FormatSysML {
		return export.FormatTurtle
	}
	return export.FormatSysML
}

// newSession returns a session in the output modes the flags asked for, under
// the run bounds resolved at startup.
func newSession() *repl.Session {
	sess := repl.NewSession()
	if err := sess.SetBudgets(budgets); err != nil {
		// Unreachable: budgets are validated in main before any session exists.
		fmt.Fprintln(os.Stderr, "sysml:", err)
		os.Exit(2)
	}
	switch {
	case debugMode:
		sess.SetVerbosity(repl.VerbosityDebug)
	case quietMode:
		sess.SetVerbosity(repl.VerbosityQuiet)
	}
	sess.SetTracing(traceMode)
	return sess
}

func runInteractiveWithFiles(files []string) error {
	histPath := filepath.Join(os.TempDir(), "sysml-repl.history")
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "sysml> ",
		HistoryFile:     histPath,
		InterruptPrompt: "^C",
		EOFPrompt:       "bye",
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	sess := newSession()

	// Load files before starting interactive loop
	for _, file := range files {
		output, _, err := sess.RunMeta("%load " + file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", file, err)
			return err
		}
		// Print load results
		for _, line := range output {
			fmt.Println(line)
		}
	}

	fmt.Println("SysML v2 REPL — %help for commands, Ctrl-D to exit")
	return repl.Loop(&rlReader{rl: rl}, os.Stdout, sess)
}

func runNonInteractive(files []string, exprs []string) error {
	sess := newSession()

	// Load files first
	for _, file := range files {
		output, _, err := sess.RunMeta("%load " + file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", file, err)
			return err
		}
		// Print load results
		for _, line := range output {
			fmt.Println(line)
		}
	}

	// Then evaluate expressions
	for _, expr := range exprs {
		output, _, err := sess.RunMeta("%eval " + expr)
		if err != nil {
			return err
		}
		// Print evaluation results
		for _, line := range output {
			fmt.Println(line)
		}
	}

	return nil
}
