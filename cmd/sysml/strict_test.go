package main

import "testing"

// extensionModel uses OpenSysML notation no SysML v2 production admits.
const extensionModel = `package Mission {
    attribute def Alarm;
    state def Monitor {
        entry; then off;
        state off {
            defer Alarm;
        }
        state on;
        transition first off accept Alarm then on;
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

// A notation error does not stop a check, so the run succeeds — but a run that
// reported an error must not also report that the model has none.
func TestValidateDoesNotCallAReportedErrorNone(t *testing.T) {
	binary := buildCLI(t)
	const bare = "package Q {\n    part def A;\n}\npackage P {\n    import Q::*;\n}\n"

	got := check(t, binary, bare, "-validate")
	wantReport(t, got, 0, "error: import without a visibility indicator",
		"no error that stops a check")
	rejectReport(t, got, ": no errors")
}

func TestKeywordAsNameReportsOnceInEitherMode(t *testing.T) {
	binary := buildCLI(t)
	const keywordName = "package P {\n    part def part;\n}\n"

	for _, tc := range []struct {
		status int
		args   []string
	}{
		{0, []string{"-validate"}},
		{2, []string{"-validate", "-strict"}},
	} {
		got := check(t, binary, keywordName, tc.args...)
		wantReport(t, got, tc.status, "error: \"part\" is a reserved keyword, not a name the ID terminal admits")
		rejectReport(t, got, "warning:")
	}
}
