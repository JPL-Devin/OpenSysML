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

// runCLI carries out what the command line asked for and returns the exit
// status, so a profile started for the run is written before the process exits.
func runCLI() int {
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
		fmt.Fprintf(os.Stderr, "  sysml model.sysml -convert ttl              # SysML notation to RDF Turtle, on stdout\n")
		fmt.Fprintf(os.Stderr, "  sysml model.ttl -convert sysml              # RDF Turtle to SysML notation\n")
		fmt.Fprintf(os.Stderr, "  sysml model.sysml -convert ttl -o m.ttl     # Write the conversion to a file\n")
		fmt.Fprintf(os.Stderr, "  sysml in.txt -convert ttl -from sysml       # Name the input format explicitly\n")
		fmt.Fprintf(os.Stderr, "\nThe input format is taken from the file extension (.sysml, .kerml, .ttl) unless\n")
		fmt.Fprintf(os.Stderr, "-from names it. Converting to the format it is already in rewrites the input:\n")
		fmt.Fprintf(os.Stderr, "notation is reformatted, Turtle is normalized.\n")
		fmt.Fprintf(os.Stderr, "\nFlags may be written before or after the model they apply to. A file named like\n")
		fmt.Fprintf(os.Stderr, "a flag is read as a file after --, which ends the flags: sysml -trace -- -m.sysml\n")
		fmt.Fprintf(os.Stderr, "\nProfiling a run:\n")
		fmt.Fprintf(os.Stderr, "  sysml -validate -memstats model.sysml            # Report what the run cost, on stderr\n")
		fmt.Fprintf(os.Stderr, "  sysml -validate -memprofile heap.out model.sysml # Write a heap profile for go tool pprof\n")
		fmt.Fprintf(os.Stderr, "  sysml -validate -cpuprofile cpu.out model.sysml  # Write a CPU profile for go tool pprof\n")
	}

	flag.Var(&evalExprs, "eval", "Evaluate expression and exit (can be specified multiple times)")
	flag.Var(&evalExprs, "e", "Evaluate expression and exit (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	flag.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	flag.BoolVar(&traceMode, "trace", false, "Report each execution step: expression evaluation, calc invocation, action tokens, state transitions")
	flag.StringVar(&convertFormat, "convert", "", "Convert the model to this format instead of running it: sysml, kerml, ttl, turtle or rdf")
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
			fmt.Fprintln(os.Stderr, "sysml:", err)
			return 1
		}
		return 0
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

	// Non-interactive mode: files + eval expressions, execute and exit
	if len(args) > 0 && len(evalExprs) > 0 {
		if err := runNonInteractive(args, evalExprs); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			return 1
		}
		return 0
	}

	// Eval-only mode: just evaluate expressions and exit
	if len(evalExprs) > 0 {
		if err := runNonInteractive(nil, evalExprs); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			return 1
		}
		return 0
	}

	if len(args) == 0 {
		// No files: interactive REPL
		if err := runInteractive(); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			return 1
		}
		return 0
	}

	// Files provided: load and enter interactive mode
	if err := runInteractiveWithFiles(args); err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		return 1
	}
	return 0
}

func runInteractive() error {
	return runInteractiveWithFiles(nil)
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
	sess := newSession()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "sysml> ",
		HistoryFile:     historyPath(),
		AutoComplete:    &sessionCompleter{sess: sess},
		InterruptPrompt: "^C",
		EOFPrompt:       "bye",
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	// Load files before starting interactive loop
	if err := loadFiles(sess, files); err != nil {
		return err
	}

	fmt.Println("SysML v2 REPL — %help for commands, Ctrl-D to exit")
	return repl.Loop(&rlReader{rl: rl}, os.Stdout, sess)
}

// loadFiles submits every positional path — a file, a directory to walk or a
// glob — as a single load, so a declaration in one file resolves against the
// others whichever order they were named in.
func loadFiles(sess *repl.Session, files []string) error {
	if len(files) == 0 {
		return nil
	}
	output, err := sess.LoadPaths(files)
	if err != nil {
		// Returned unwrapped: the caller reports it, so the operation and the
		// path are named once.
		return err
	}
	for _, line := range output {
		fmt.Println(line)
	}
	return nil
}

func runNonInteractive(files []string, exprs []string) error {
	sess := newSession()

	// Load files first
	if err := loadFiles(sess, files); err != nil {
		return err
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
