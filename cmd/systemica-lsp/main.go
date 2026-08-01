// Command systemica-lsp is a stdio Language Server for SysML v2 / KerML.
package main

import (
	"context"
	"errors"
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
	return errors.Join(os.Stdin.Close(), os.Stdout.Close())
}

func main() {
	ws := model.NewWorkspace()
	srv := lsp.NewServer(ws)
	if err := srv.Run(context.Background(), stdio{}); err != nil {
		log.Fatalf("systemica-lsp: %v", err)
	}
}
