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
)

// maxSteps is the evaluation step budget SYSML_MAX_STEPS resolves to, read once
// at startup.
var maxSteps = runtime.DefaultMaxSteps

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
	flag.Parse()

	if debugMode && quietMode {
		fmt.Fprintln(os.Stderr, "sysml: -debug and -quiet are mutually exclusive")
		os.Exit(2)
	}

	// Resolve the evaluation step budget before anything runs, so a bad value is
	// reported at startup rather than mistaken for the default at execution time.
	var err error
	maxSteps, err = runtime.MaxStepsFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
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
			return err
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
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes)\n", outputPath, to, len(out))
	return nil
}

// resolveFormat returns the format named by the flag, or the one the path's
// extension implies.
func resolveFormat(flagValue, path string) (export.Format, error) {
	if flagValue != "" {
		return export.ParseFormat(flagValue)
	}
	return export.FormatOfPath(path)
}

// otherFormat returns the format to convert to when only the input format is
// known: the other one of the pair.
func otherFormat(from export.Format) export.Format {
	if from == export.FormatSysML {
		return export.FormatTurtle
	}
	return export.FormatSysML
}

// newSession returns a session in the output modes the flags asked for, with
// the evaluation step budget resolved at startup.
func newSession() *repl.Session {
	sess := repl.NewSession()
	if err := sess.SetMaxSteps(maxSteps); err != nil {
		// Unreachable: maxSteps is validated in main before any session exists.
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
