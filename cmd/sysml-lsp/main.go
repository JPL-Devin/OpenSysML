// Command sysml-lsp is a stdio Language Server for SysML v2 / KerML.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

// Exit statuses: 0 for the protocol served to its end, 2 for a command line the
// server cannot act on, as for sysml.
const (
	exitServed      = 0
	exitUnservable  = 2
	commandPrefix   = "sysml-lsp: "
	protocolMessage = "the protocol is spoken over stdin/stdout, so an editor starts this server rather than a shell"
)

// stdio adapts os.Stdin/os.Stdout into a single io.ReadWriteCloser.
type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error {
	return errors.Join(os.Stdin.Close(), os.Stdout.Close())
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run carries out what the command line asked for: the version, the help, or
// the protocol itself. A flag it could not read is reported with the usage,
// rather than entering protocol mode and dying on the first header line.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sysml-lsp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(fs.Output(), fs) }

	// -h/-help are defined here so that help asked for is a result on stdout.
	var showVersion, showHelp bool
	fs.BoolVar(&showVersion, "version", false, "Show version information")
	fs.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	fs.BoolVar(&showHelp, "help", false, "Show this help and exit")
	fs.BoolVar(&showHelp, "h", false, "Show this help (shorthand)")
	if err := fs.Parse(args); err != nil {
		// Parse has already reported the flag and printed the usage on stderr.
		return exitUnservable
	}

	if showHelp {
		printUsage(stdout, fs)
		return exitServed
	}
	if showVersion {
		fmt.Fprintf(stdout, "sysml-lsp %s\n", Version)
		fmt.Fprintf(stdout, "  Commit:     %s\n", Commit)
		fmt.Fprintf(stdout, "  Build time: %s\n", BuildTime)
		fmt.Fprintf(stdout, "  Go version: %s\n", GoVersion)
		return exitServed
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%sunexpected argument %q; %s\n", commandPrefix, fs.Arg(0), protocolMessage)
		printUsage(stderr, fs)
		return exitUnservable
	}

	return serve(stderr)
}

// printUsage writes the help to w, which the caller chooses: help asked for is
// a result, while help over a misuse belongs with the error.
func printUsage(w io.Writer, fs *flag.FlagSet) {
	// PrintDefaults writes to the flag set's own stream, restored afterwards.
	previous := fs.Output()
	fs.SetOutput(w)
	defer fs.SetOutput(previous)

	fmt.Fprintf(w, "Usage: sysml-lsp [options]\n\n")
	fmt.Fprintf(w, "Options:\n")
	fs.PrintDefaults()
	fmt.Fprintf(w, "\nWith no options, %s.\n", protocolMessage)
}

// serve speaks the protocol over stdin/stdout until the client ends it.
func serve(stderr io.Writer) int {
	ws := model.NewWorkspace()
	srv := lsp.NewServer(ws)
	err := srv.Run(context.Background(), stdio{})

	// LSP spec: Exit after Shutdown should return 0, Exit without Shutdown
	// should return 1. The exit code is handled by Exit() calling conn.Close().
	if err != nil {
		// Ignore "file already closed" from clean exit
		if !errors.Is(err, os.ErrClosed) && err.Error() != "failed reading header line: read /dev/stdin: file already closed" {
			fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
			return 1
		}
	}
	return exitServed
}
