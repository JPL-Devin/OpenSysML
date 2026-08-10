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
)

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
	}

	flag.Var(&evalExprs, "eval", "Evaluate expression and exit (can be specified multiple times)")
	flag.Var(&evalExprs, "e", "Evaluate expression and exit (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	flag.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	flag.Parse()

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

// newSession returns a session at the verbosity the flags asked for.
func newSession() *repl.Session {
	sess := repl.NewSession()
	switch {
	case debugMode:
		sess.SetVerbosity(repl.VerbosityDebug)
	case quietMode:
		sess.SetVerbosity(repl.VerbosityQuiet)
	}
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
