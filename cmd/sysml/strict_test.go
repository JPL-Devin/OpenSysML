package main

import "testing"

// extensionModel uses OpenSysML notation no SysML v2 production admits.
const extensionModel = `package Mission {
    state def Monitor {
        initial off;
        state off;
        state on;
        transition off to on;
    }
}
`

// -strict answers a different question about the same file: the notation stays
// parsed, and the warnings it draws by default become the errors that decide the
// exit status.
func TestStrictConformanceDecidesTheExitStatus(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, extensionModel, "-validate"),
		0, "warning:", "is an OpenSysML extension with no SysML v2 production")

	strict := check(t, binary, extensionModel, "-validate", "-strict")
	wantReport(t, strict, 2, "error:", "is an OpenSysML extension with no SysML v2 production",
		"did not analyse cleanly")
	rejectReport(t, strict, "warning:")
}

// A model in standard notation is unaffected, so -strict is a check and not a
// second dialect.
func TestStrictConformanceLeavesStandardNotationAlone(t *testing.T) {
	binary := buildCLI(t)
	const standard = `package Mission {
    state def Monitor {
        entry; then off;
        state off;
        state on;
        transition first off accept Signal then on;
    }
    attribute def Signal;
}
`
	for _, args := range [][]string{{"-validate"}, {"-validate", "-strict"}} {
		wantReport(t, check(t, binary, standard, args...), 0, "no errors")
	}
}
