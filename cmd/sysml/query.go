package main

import (
	"fmt"
	"os"
)

func runQuery(files []string, text string) int {
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "sysml: no model to query; name a model file")
		return exitUnevaluable
	}
	sess := newSession()
	status, err := loadFiles(sess, files)
	if err != nil {
		return fail(err)
	}
	if status != exitHolds {
		return status
	}
	lines, err := sess.Query(text)
	if err != nil {
		return fail(err)
	}
	if len(lines) == 0 {
		// Stdout carries one matched element per line, so this report stays out
		// of it.
		fmt.Fprintln(os.Stderr, "sysml: no elements matched")
		return exitHolds
	}
	writeLines(os.Stdout, lines)
	return exitHolds
}
