// Command sysml-lsp is a stdio Language Server for SysML v2 / KerML.
package main

import (
	"context"
	"log"
	"os"

	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/lsp"
)

// stdio adapts os.Stdin/os.Stdout into a single io.ReadWriteCloser.
type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}

func main() {
	ws := model.NewWorkspace()
	srv := lsp.NewServer(ws)
	if err := srv.Run(context.Background(), stdio{}); err != nil {
		log.Fatalf("sysml-lsp: %v", err)
	}
}
