package main

import "flag"

// permuteArgs moves the positional arguments after the flags, so that a flag may
// be written after the model it applies to: `sysml model.sysml -convert ttl`
// reads the same as `sysml -convert ttl model.sysml`. Go's flag package stops at
// the first non-flag argument, which would otherwise leave the flags unparsed.
//
// A flag that takes a value carries the argument after it, so that value is not
// mistaken for a file. An unrecognized flag is left where it is, for flag.Parse
// to report.
func permuteArgs(fs *flag.FlagSet, args []string) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			// Everything after it is positional by definition.
			positional = append(positional, args[i+1:]...)
			break
		}
		if !isFlag(arg) {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		if takesSeparateValue(fs, arg) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

// isFlag reports whether arg is written as a flag rather than as a file name. A
// lone "-" names stdin by convention, not a flag.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// takesSeparateValue reports whether the flag consumes the argument after it:
// it must be a known flag, spelled without "=", and not a boolean one, since
// boolean flags only accept -flag=value.
func takesSeparateValue(fs *flag.FlagSet, arg string) bool {
	name := arg
	for len(name) > 0 && name[0] == '-' {
		name = name[1:]
	}
	for i := 0; i < len(name); i++ {
		if name[i] == '=' {
			return false
		}
	}
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolFlag.IsBoolFlag()
}
