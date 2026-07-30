// Command sysml-repl is an interactive SysML v2 REPL (spec §13).
package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/Systemica/internal/repl"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sysml-repl:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = repl.NewSession() // loop wired in Task 8
	return nil
}
