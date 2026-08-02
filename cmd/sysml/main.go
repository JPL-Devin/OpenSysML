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
	if err == readline.ErrInterrupt { // Ctrl-C clears the current line, not EOF
		return "", nil
	}
	if err == io.EOF {
		return "", io.EOF
	}
	return line, err
}

// CLI flags
var (
	loadFiles  stringSlice
	evalExprs  stringSlice
	quietMode  bool
	showVersion bool
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
	flag.Var(&loadFiles, "load", "Load SysML file (can be specified multiple times)")
	flag.Var(&loadFiles, "l", "Load SysML file (shorthand)")
	flag.Var(&evalExprs, "eval", "Evaluate expression (can be specified multiple times)")
	flag.Var(&evalExprs, "e", "Evaluate expression (shorthand)")
	flag.BoolVar(&quietMode, "quiet", false, "Quiet mode: suppress prompts and decorations")
	flag.BoolVar(&quietMode, "q", false, "Quiet mode (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	flag.Parse()

	// Handle version flag
	if showVersion {
		fmt.Printf("sysml %s\n", Version)
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		os.Exit(0)
	}

	// Non-interactive mode: execute commands and exit
	if len(loadFiles) > 0 || len(evalExprs) > 0 {
		if err := runNonInteractive(); err != nil {
			fmt.Fprintln(os.Stderr, "sysml:", err)
			os.Exit(1)
		}
		return
	}

	// Interactive mode (default)
	if err := runInteractive(); err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		os.Exit(1)
	}
}

func runInteractive() error {
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
	fmt.Println("SysML v2 REPL — %help for commands, Ctrl-D to exit")
	return repl.Loop(&rlReader{rl: rl}, os.Stdout, repl.NewSession())
}

func runNonInteractive() error {
	sess := repl.NewSession()
	
	// Build command sequence
	var commands []string
	for _, file := range loadFiles {
		commands = append(commands, "%load "+file)
	}
	for _, expr := range evalExprs {
		commands = append(commands, "%eval "+expr)
	}
	
	// Execute commands
	for _, cmd := range commands {
		output, quit, err := sess.RunMeta(cmd)
		if err != nil {
			if quietMode {
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			return err
		}
		
		// Print output
		if quietMode {
			// Quiet mode: only print actual results, strip decorations
			for _, line := range output {
				trimmed := strings.TrimSpace(line)
				// Skip empty lines, checkmarks, package declarations
				if trimmed == "" || 
				   strings.HasPrefix(trimmed, "✓") || 
				   strings.HasPrefix(trimmed, "sysml>") ||
				   strings.HasPrefix(trimmed, "package ") {
					continue
				}
				fmt.Println(line)
			}
		} else {
			// Normal mode: print everything
			for _, line := range output {
				fmt.Println(line)
			}
		}
		
		if quit {
			break
		}
	}
	
	return nil
}
