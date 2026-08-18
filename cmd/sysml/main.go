// Command sysml is an interactive SysML v2 REPL (spec §13).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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

// sessionCompleter completes prompt input from the session: meta commands,
// declared and library names, and file paths after %load and %save.
type sessionCompleter struct{ sess *repl.Session }

// Do answers a tab press with the remainder of each candidate, as readline's
// AutoCompleter expects, and how many runes of the word were already typed.
func (c *sessionCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if pos < 0 || pos > len(line) {
		pos = len(line)
	}
	comp := c.sess.Complete(string(line), len(string(line[:pos])))
	out := make([][]rune, 0, len(comp.Candidates))
	for _, cand := range comp.Candidates {
		out = append(out, []rune(strings.TrimPrefix(cand, comp.Prefix)))
	}
	return out, utf8.RuneCountInString(comp.Prefix)
}

// historyPath returns the file the prompt keeps its history in:
// $XDG_STATE_HOME/sysml/history when that is set, and ~/.sysml_history
// otherwise. It returns "" when neither can be written, which leaves the
// history in memory for the session rather than failing the prompt.
func historyPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		if path, ok := writableFile(filepath.Join(dir, "sysml"), "history"); ok {
			return path
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path, ok := writableFile(home, ".sysml_history")
	if !ok {
		return ""
	}
	return path
}

// writableFile returns the path of name in dir, creating dir and confirming the
// file can be appended to.
func writableFile(dir, name string) (string, bool) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", false
	}
	if cerr := f.Close(); cerr != nil {
		return "", false
	}
	return path, true
}

// CLI flags
var (
	evalExprs     stringSlice
	showHelp      bool
	showVersion   bool
	debugMode     bool
	quietMode     bool
	traceMode     bool
	convertFormat string
	outputPath    string
	fromFormat    string
	modelChecks   checks
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
	os.Exit(runCLI())
}

// wrapped breaks text into lines of at most width characters, so a sentence the
// help prints rather than restates still reads as a paragraph.
func wrapped(text string, width int) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// printUsage writes the help to w: the caller chooses the stream, since help
// asked for is a result and help shown over a misuse belongs with the error.
func printUsage(w io.Writer) {
	// PrintDefaults writes to the flag set's own stream, restored after so it does
	// not decide where a later error is reported.
	previous := flag.CommandLine.Output()
	flag.CommandLine.SetOutput(w)
	defer flag.CommandLine.SetOutput(previous)

	fmt.Fprintf(w, "Usage: sysml [options] [file...]\n\n")
	fmt.Fprintf(w, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(w, "\nExamples:\n")
	fmt.Fprintf(w, "  sysml                     # Start interactive REPL\n")
	fmt.Fprintf(w, "  sysml -e \"5 + 3\"          # Evaluate and exit\n")
	fmt.Fprintf(w, "  sysml -e \"expr\" file.sysml # Load file, evaluate, and exit\n")
	fmt.Fprintf(w, "  sysml file.sysml          # Load file and start REPL\n")
	fmt.Fprintf(w, "  sysml -debug file.sysml   # Load file, reporting every diagnostic\n")
	fmt.Fprintf(w, "  sysml -trace file.sysml   # Load file, reporting each execution step\n")
	fmt.Fprintf(w, "\nChecking a model:\n")
	fmt.Fprintf(w, "  sysml -constraint MassBudget model.sysml       # Evaluate one constraint and exit\n")
	fmt.Fprintf(w, "  sysml -requirement PowerMargin model.sysml     # Evaluate one requirement and exit\n")
	fmt.Fprintf(w, "  sysml -satisfy model.sysml                     # Evaluate every satisfaction assertion\n")
	fmt.Fprintf(w, "  sysml -satisfy=Ctx model.sysml                 # ...only the ones Ctx states\n")
	fmt.Fprintf(w, "  sysml -instantiate p -constraint C model.sysml  # Check C against an object of p\n")
	fmt.Fprintf(w, "  sysml -validate model.sysml                    # Report diagnostics only\n")
	fmt.Fprintf(w, "  sysml -calc \"Fall(3, 4)\" model.sysml           # Invoke a calculation\n")
	fmt.Fprintf(w, "  sysml -action Drive model.sysml                # Run an action to completion\n")
	fmt.Fprintf(w, "  sysml -state Mission -advance 10 model.sysml   # Run a state machine for 10 time units\n")
	fmt.Fprintf(w, "  sysml -satisfy -json model.sysml               # Report the verdicts as JSON\n")
	fmt.Fprintf(w, "\nEvery run that is not a prompt exits 0 when it did what was asked, 1 when the\n")
	fmt.Fprintf(w, "model answered false for a check, and 2 when what was asked could not be\n")
	fmt.Fprintf(w, "carried out at all — an unreadable file, a model that did not analyse cleanly,\n")
	fmt.Fprintf(w, "an unresolved name, a failed conversion — so a run can gate a build. What was\n")
	fmt.Fprintf(w, "asked for is reported on stdout and what went wrong on stderr, prefixed\n")
	fmt.Fprintf(w, "\"sysml: \" unless it locates a finding in the source. Each check flag may be\n")
	fmt.Fprintf(w, "repeated.\n")
	fmt.Fprintf(w, "\nConversion:\n")
	fmt.Fprintf(w, "  sysml model.sysml -convert ttl              # SysML notation to RDF Turtle, on stdout\n")
	fmt.Fprintf(w, "  sysml model.ttl -convert sysml              # RDF Turtle to SysML notation\n")
	fmt.Fprintf(w, "  sysml model.sysml -convert ttl -o m.ttl     # Write the conversion to a file\n")
	fmt.Fprintf(w, "  sysml in.txt -convert ttl -from sysml       # Name the input format explicitly\n")
	fmt.Fprintf(w, "\nThe input format is taken from the file extension (.sysml, .kerml, .ttl) unless\n")
	fmt.Fprintf(w, "-from names it. Converting to the format it is already in rewrites the input:\n")
	fmt.Fprintf(w, "notation is reformatted, Turtle is normalized.\n")
	// The notice is printed, not restated, so the help cannot drift from what a
	// conversion reports.
	fmt.Fprintf(w, "\n%s\n", wrapped(export.ExperimentalNotice, 78))
	fmt.Fprintf(w, "Every run that converts RDF says so on stderr. Saving to .sysml or .kerml is\n")
	fmt.Fprintf(w, "stable.\n")
	fmt.Fprintf(w, "\nFlags may be written before or after the model they apply to. A file named like\n")
	fmt.Fprintf(w, "a flag is read as a file after --, which ends the flags: sysml -trace -- -m.sysml\n")
	fmt.Fprintf(w, "\nReading from standard input:\n")
	fmt.Fprintf(w, "  cat model.sysml | sysml -validate -           # A lone - names standard input\n")
	fmt.Fprintf(w, "  cat model.sysml | sysml - -convert ttl -from sysml\n")
	fmt.Fprintf(w, "\nWhat was read from standard input is called <stdin> in diagnostics, and a file\n")
	fmt.Fprintf(w, "really named \"-\" is read by naming it ./- instead.\n")
	fmt.Fprintf(w, "\nProfiling a run:\n")
	fmt.Fprintf(w, "  sysml -validate -memstats model.sysml            # Report what the run cost, on stderr\n")
	fmt.Fprintf(w, "  sysml -validate -memprofile heap.out model.sysml # Write a heap profile for go tool pprof\n")
	fmt.Fprintf(w, "  sysml -validate -cpuprofile cpu.out model.sysml  # Write a CPU profile for go tool pprof\n")
}

// runCLI carries out what the command line asked for and returns the exit
// status, so a profile started for the run is written before the process exits.
func runCLI() int {
	// Usage shown over a misuse goes on the stream the error naming it goes on.
	flag.Usage = func() { printUsage(flag.CommandLine.Output()) }

	flag.BoolVar(&showHelp, "help", false, "Show this help and exit")
	flag.BoolVar(&showHelp, "h", false, "Show this help (shorthand)")
	flag.Var(&evalExprs, "eval", "Evaluate expression and exit (can be specified multiple times)")
	flag.Var(&evalExprs, "e", "Evaluate expression and exit (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	flag.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	flag.BoolVar(&traceMode, "trace", false, "Report each execution step: expression evaluation, calc invocation, action tokens, state transitions")
	flag.StringVar(&convertFormat, "convert", "", "Convert the model to this format instead of running it: sysml, kerml, ttl, turtle or rdf (RDF is experimental)")
	flag.StringVar(&outputPath, "output", "", "Write conversion output to this file (default: stdout)")
	flag.StringVar(&outputPath, "o", "", "Write conversion output to this file (shorthand)")
	flag.StringVar(&fromFormat, "from", "", "Input format for -convert: sysml, kerml, ttl, turtle or rdf (default: from the input's extension)")
	flag.Var(&deprecatedFlag{instead: "-to has been replaced by -convert, as `sysml model.sysml -convert ttl`"}, "to", "Replaced by -convert, which names the output format")
	flag.Var(&modelChecks.instantiate, "instantiate", "Create an object of this definition before the checks, so a verdict is about it (repeatable)")
	flag.Var(&modelChecks.constraints, "constraint", "Evaluate this constraint and exit (repeatable)")
	flag.Var(&modelChecks.requirements, "requirement", "Evaluate this requirement and exit (repeatable)")
	flag.Var(&modelChecks.satisfy, "satisfy", "Evaluate every satisfaction assertion, or with -satisfy=<name> those the named element states (repeatable)")
	flag.BoolVar(&modelChecks.validate, "validate", false, "Analyse the model and report its diagnostics, exiting nonzero on an error")
	flag.Var(&modelChecks.calcs, "calc", "Invoke this calculation and report what it computed, as -calc \"Fall(3, 4)\" (repeatable)")
	flag.Var(&modelChecks.actions, "action", "Run this action to completion, as -action \"Drive rover1\" to run it on an object (repeatable)")
	flag.Var(&modelChecks.states, "state", "Run this state machine, as -state \"Mission rover1\" to run it on an object (repeatable)")
	flag.Var(&modelChecks.advance, "advance", "Simulated time units to run each -state machine for (default: only its initial transition)")
	flag.BoolVar(&modelChecks.jsonOut, "json", false, "Report checks as one JSON document rather than as lines")
	flag.StringVar(&cpuProfilePath, "cpuprofile", "", "Write a CPU profile of the run to this file, for go tool pprof")
	flag.StringVar(&memProfilePath, "memprofile", "", "Write a heap profile of the run to this file, for go tool pprof")
	flag.BoolVar(&memStats, "memstats", false, "Report on stderr what the run cost: wall time, memory allocated, memory taken from the OS")
	if err := flag.CommandLine.Parse(permuteArgs(flag.CommandLine, os.Args[1:])); err != nil {
		// flag.CommandLine exits on error; unreachable unless that changes.
		return 2
	}

	// Help that was asked for is the result of the run: it belongs on stdout, where
	// it can be piped, and the run did what was asked.
	if showHelp {
		printUsage(os.Stdout)
		return exitHolds
	}

	if debugMode && quietMode {
		fmt.Fprintln(os.Stderr, "sysml: -debug and -quiet are mutually exclusive")
		return 2
	}

	stopProfiling, err := startProfiling()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		return 2
	}
	defer stopProfiling()

	// Handle version flag
	if showVersion {
		fmt.Printf("sysml %s\n", Version)
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		return 0
	}

	// Get positional arguments (files to load)
	args := flag.Args()

	if convertFormat != "" {
		if modelChecks.requested() {
			return refuse(modelChecks,
				"-convert writes the model out and decides nothing about it; check it in its own run")
		}
		if err := runConvert(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	// Resolve the run bounds before any model runs, so a bad value is reported at
	// startup rather than mistaken for the default at execution time. Reporting the
	// version and converting a model evaluate nothing, so they are handled above.
	budgets, err = runtime.BudgetsFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		return 2
	}

	// Checking mode: load, check what was named, and exit on the verdict.
	if modelChecks.requested() {
		return runChecks(args, evalExprs, modelChecks)
	}

	// Non-interactive mode: evaluate the expressions against the model and exit.
	if len(evalExprs) > 0 {
		return runNonInteractive(args, evalExprs)
	}

	// No expression to evaluate: load whatever was named and take lines.
	return runInteractiveWithFiles(args)
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

// runInteractiveWithFiles loads what was named and takes lines from the prompt.
// A model that did not analyse, or a feature value a command could not materialize, leaves
// the status of a run that decided nothing, except at a terminal, where the
// prompt that opens is where it gets fixed.
func runInteractiveWithFiles(files []string) int {
	sess := newSession()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "sysml> ",
		HistoryFile:     historyPath(),
		AutoComplete:    &sessionCompleter{sess: sess},
		InterruptPrompt: "^C",
		EOFPrompt:       "bye",
	})
	if err != nil {
		return fail(err)
	}
	defer rl.Close()

	loaded, err := loadFiles(sess, files)
	if err != nil {
		return fail(err)
	}
	terminal := atTerminal()

	fmt.Println("SysML v2 REPL — %help for commands, Ctrl-D to exit")
	if err := repl.Loop(&rlReader{rl: rl}, os.Stdout, sess); err != nil {
		return fail(err)
	}
	return sessionStatus(loaded, terminal, sess.MaterializationFailures())
}

// sessionStatus is the status a prompt session leaves: at a terminal the session
// is where an unusable model gets fixed, so it decides nothing, and otherwise the
// run is undecided if the model did not analyse or a command reported a feature value it
// could not materialize.
func sessionStatus(loaded int, terminal bool, materializeFailures []error) int {
	if terminal {
		return exitHolds
	}
	if len(materializeFailures) > 0 {
		return exitUnevaluable
	}
	return loaded
}

// loadFiles submits every positional path — a file, a directory to walk or a
// glob — as a single load, so a declaration in one file resolves against the
// others whichever order they were named in. What the load declared is reported
// on stdout and what the analysis found on stderr; the status is that of a run
// that decided nothing when the model did not analyse cleanly.
func loadFiles(sess *repl.Session, files []string) (int, error) {
	if len(files) == 0 {
		return exitHolds, nil
	}
	report, err := sess.LoadPathsReport(files)
	if err != nil {
		// Returned unwrapped: the caller reports it, so the operation and the
		// path are named once.
		return exitUnevaluable, err
	}
	writeLines(os.Stdout, report.Loaded)
	writeLines(os.Stderr, report.Found)
	if report.Errors {
		fmt.Fprintf(os.Stderr, "sysml: %s did not analyse cleanly\n", namedModels(files))
		return exitUnevaluable, nil
	}
	writeLines(os.Stdout, report.Declared)
	return exitHolds, nil
}

// runNonInteractive loads the model and evaluates every expression asked for,
// reporting the values on stdout and anything that stopped it on stderr.
func runNonInteractive(files []string, exprs []string) int {
	sess := newSession()
	status, err := loadFiles(sess, files)
	if err != nil {
		return fail(err)
	}
	if status != exitHolds {
		return status
	}

	for _, expr := range exprs {
		output, err := sess.EvalExpr(expr)
		if err != nil {
			return fail(err)
		}
		writeLines(os.Stdout, output)
	}
	return exitHolds
}
