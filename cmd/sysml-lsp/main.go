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
	// Version information - set via ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
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
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		os.Exit(0)
	}

	ws := model.NewWorkspace()
	srv := lsp.NewServer(ws)
	err := srv.Run(context.Background(), stdio{})

	// LSP spec: Exit after Shutdown should return 0, Exit without Shutdown should return 1
	if err != nil {
		// Ignore "file already closed" from clean exit
		if !errors.Is(err, os.ErrClosed) && err.Error() != "failed reading header line: read /dev/stdin: file already closed" {
			log.Fatalf("sysml-lsp: %v", err)
		}
	}

	// Exit code handled by Exit() calling conn.Close()
	os.Exit(0)
}
