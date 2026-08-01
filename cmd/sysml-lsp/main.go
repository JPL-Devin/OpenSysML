// Command sysml-lsp is a stdio Language Server for SysML v2 / KerML.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/lsp"
)

var (
	// Version is set via ldflags during build
	Version = "dev"
)

// stdio adapts os.Stdin/os.Stdout into a single io.ReadWriteCloser.
type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error {
	return errors.Join(os.Stdin.Close(), os.Stdout.Close())
}

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("sysml-lsp %s\n", Version)
		os.Exit(0)
	}

	ws := model.NewWorkspace()
	srv := lsp.NewServer(ws)
	if err := srv.Run(context.Background(), stdio{}); err != nil {
		log.Fatalf("sysml-lsp: %v", err)
	}
}
