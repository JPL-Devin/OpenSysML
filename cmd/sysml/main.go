// Command sysml is an interactive SysML v2 REPL (spec §13).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/Systemica/internal/repl"
)

var (
	// Version is set via ldflags during build
	Version = "dev"
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

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("sysml %s\n", Version)
		os.Exit(0)
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sysml:", err)
		os.Exit(1)
	}
}

func run() error {
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
