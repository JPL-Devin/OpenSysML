package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/Systemica/internal/core/export"
)

// deprecatedFlag rejects a flag that has been replaced, so the old spelling
// reports what to write instead of "flag provided but not defined".
type deprecatedFlag struct{ instead string }

func (f *deprecatedFlag) String() string { return "" }

func (f *deprecatedFlag) Set(string) error { return errors.New(f.instead) }

// runConvert converts the model named on the command line to the format
// -convert asks for, writing to -o or to stdout.
//
// The input format is taken from -from when given and from the file extension
// otherwise; the model itself is a positional argument, as it is for every other
// mode of the command.
func runConvert(files []string) error {
	to, err := parseTargetFormat(convertFormat)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no model to convert; name the file to convert, as `sysml model.sysml -convert ttl`")
	}
	if len(files) > 1 {
		return fmt.Errorf("-convert converts one file; unexpected extra argument %q", files[1])
	}
	input := files[0]

	from, err := resolveFormat(fromFormat, input)
	if err != nil {
		return err
	}

	// #nosec G304 -- the input file is the one named on the command line.
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	out, err := export.Convert(input, data, from, to)
	if err != nil {
		return err
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	replaced, err := export.WriteFile(outputPath, out)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", outputPath, to, len(out), what)
	return nil
}

// parseTargetFormat resolves the -convert value, explaining the flag when a file
// name was passed where a format belongs — the spelling this flag used to take.
func parseTargetFormat(value string) (export.Format, error) {
	f, err := export.ParseFormat(value)
	if err != nil && namesAFile(value) {
		return 0, fmt.Errorf("%w; -convert names the format to convert to, so write `sysml %s -convert ttl`", err, value)
	}
	return f, err
}

// namesAFile reports whether the value looks like a path rather than a format
// name: it exists on disk, or is written with a directory or an extension.
func namesAFile(value string) bool {
	if _, err := os.Stat(value); err == nil {
		return true
	}
	return filepath.Ext(value) != "" || filepath.Dir(value) != "."
}

// resolveFormat returns the format named by the flag, or the one the path's
// extension implies.
func resolveFormat(flagValue, path string) (export.Format, error) {
	if flagValue != "" {
		return export.ParseFormat(flagValue)
	}
	f, err := export.FormatOfPath(path)
	return f, export.Advise(err, "pass -from, or "+export.ExtensionAdvice)
}
