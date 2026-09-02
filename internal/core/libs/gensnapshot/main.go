// Command gensnapshot writes the bundled library's snapshot, the derived
// artifact package libs embeds; run through `go generate ./internal/core/libs`.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
)

func main() {
	out := flag.String("out", "stdlib.snapshot", "file to write the snapshot to")
	check := flag.Bool("check", false, "fail if the file differs from a fresh snapshot instead of writing it")
	flag.Parse()

	data, err := libs.BuildSnapshot(libs.EmbeddedSource())
	if err != nil {
		fmt.Fprintln(os.Stderr, "gensnapshot:", err)
		os.Exit(1)
	}
	if *check {
		have, err := os.ReadFile(*out)
		if err != nil || !bytes.Equal(have, data) {
			fmt.Fprintf(os.Stderr, "gensnapshot: %s is stale; run `go generate ./internal/core/libs`\n", *out)
			os.Exit(1)
		}
		return
	}
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "gensnapshot:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, len(data))
}
