package repl

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/codegen"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// compiledCase invokes one calc; the interpreter is the oracle for what the
// compiled program must print, or which failure it must report.
type compiledCase struct {
	calc string
	args []string
}

var compiledCases = []compiledCase{
	{"Fib", []string{"0"}}, {"Fib", []string{"1"}}, {"Fib", []string{"20"}},
	{"SumTo", []string{"0"}}, {"SumTo", []string{"1000"}}, {"SumTo", []string{"200000"}},
	{"Arith", []string{"7", "3"}}, {"Arith", []string{"-7", "3"}}, {"Arith", []string{"7", "0"}},
	{"Arith", []string{"9223372036854775807", "1"}},
	{"Quot", []string{"7", "2"}}, {"Quot", []string{"1", "3"}}, {"Quot", []string{"-1", "3"}},
	{"Quot", []string{"1", "0"}}, {"Quot", []string{"9007199254740993", "1"}},
	{"Quot", []string{"9223372036854775807", "-9223372036854775808"}},
	{"Mixed", []string{"1.5", "3"}}, {"Mixed", []string{"0.1", "0"}},
	{"Pow", []string{"-4"}}, {"Pow", []string{"3037000500"}},
	{"RealPow", []string{"2.0", "0.5"}}, {"RealPow", []string{"0.0", "-1.0"}}, {"RealPow", []string{"-8.0", "0.5"}},
	{"RealPow", []string{"10.0", "400.0"}},
	{"Neg", []string{"5"}}, {"Neg", []string{"-9223372036854775808"}},
	{"Logic", []string{"false", "true", "0"}}, {"Logic", []string{"false", "false", "0"}},
	{"Logic", []string{"true", "true", "0"}}, {"Logic", []string{"true", "true", "5"}},
	{"Compare", []string{"2.0", "2"}}, {"Compare", []string{"2.5", "2"}}, {"Compare", []string{"0.1", "1"}},
	{"Sign", []string{"-9"}}, {"Sign", []string{"0"}}, {"Sign", []string{"3"}},
	{"FirstAbove", []string{"50"}}, {"FirstAbove", []string{"0"}},
	{"Collatz", []string{"27"}}, {"Collatz", []string{"1"}},
	{"Hypot", []string{"3.0", "4.0"}},
	{"Trailing", []string{"1.5", "2"}},
	{"Named", []string{"7", "3"}},
	{"Ratio", []string{"3", "4"}}, {"Ratio", []string{"0", "4"}},
	{"Even", []string{"7"}}, {"Even", []string{"10"}}, {"Even", []string{"100000"}}, // exceeds the recursion limit
	{"Big", []string{"1"}}, {"Big", []string{"0"}},
	{"Order", []string{"0", "9223372036854775807"}}, {"Order", []string{"3", "9223372036854775807"}},
	{"Order", []string{"3", "4"}},
	{"UntilLoop", []string{"0"}}, {"UntilLoop", []string{"-3"}}, {"UntilLoop", []string{"5"}},
	{"WhileUntil", []string{"0"}}, {"WhileUntil", []string{"5"}}, {"WhileUntil", []string{"20"}},
	{"Hypot", []string{"1e-400", "4.0"}}, {"Hypot", []string{"1e-320", "4.0"}},
	{"fib", []string{"10"}}, {"Specialized", []string{"12"}}, {"ViaUsage", []string{"11"}},
	{"NamedOrder", []string{"0", "9223372036854775807"}}, {"NamedOrder", []string{"3", "4"}},
	{"NamedTwice", []string{"0", "9223372036854775807"}}, {"NamedTwice", []string{"3", "9223372036854775807"}}, {"NamedTwice", []string{"3", "4"}},
	{"OrderArgs", []string{"0", "9223372036854775807"}}, {"OrderArgs", []string{"3", "9223372036854775807"}},
	{"Nat", []string{"5"}}, {"Nat", []string{"0"}}, {"Nat", []string{"-1"}},
	{"Pos", []string{"2"}}, {"Pos", []string{"1"}}, {"Pos", []string{"0"}},
	{"One", []string{"21"}},
	{"Collide", []string{"1"}},
}

// failureClass is the part of a failure both surfaces spell the same way.
func failureClass(msg string) string {
	for _, class := range []string{"arithmetic overflow", "arithmetic domain", "division by zero", "calc recursion limit exceeded", "typed by Natural", "typed by Positive"} {
		if strings.Contains(msg, class) {
			return class
		}
	}
	return "unclassified: " + msg
}

// interpreted answers a case with the interpreter: the value, or the failure.
func interpreted(t *testing.T, s *Session, c compiledCase) (value, failure string) {
	t.Helper()
	v := s.RunCalc("Compiled::" + c.calc + "(" + strings.Join(c.args, ", ") + ")")
	if v.Status == VerdictHolds {
		for _, line := range v.Lines {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "= "); ok {
				return rest, ""
			}
		}
		t.Fatalf("%s%v: no value in %q", c.calc, c.args, v.Lines)
	}
	return "", failureClass(strings.Join(v.Lines, "\n"))
}

// compiledRun answers a case with the executable: the value, or the failure.
func compiledRun(t *testing.T, exe string, c compiledCase) (value, failure string) {
	t.Helper()
	out, err := exec.Command(exe, c.args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err == nil {
		return text, ""
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("%s %v: %v\n%s", exe, c.args, err, text)
	}
	return "", failureClass(text)
}

// loadCompileFixture loads the fixture into a session whose step budget is
// lifted: compiled code has none, so the oracle must run each case to its end.
func loadCompileFixture(t testing.TB) *Session {
	t.Helper()
	data, err := os.ReadFile("testdata/compile_calcs.sysml")
	if err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(string(data)).Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture has errors: %v", errs)
	}
	budgets := runtime.DefaultBudgets()
	budgets.MaxSteps = 1 << 40
	if err := s.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	return s
}

// The compiled program computes what the interpreter computes, prints it as
// the interpreter prints it, and fails where the interpreter fails.
func TestCompiledCalcsAgreeWithInterpreter(t *testing.T) {
	for _, target := range codegen.Targets() {
		t.Run(string(target), func(t *testing.T) {
			if target == codegen.TargetC {
				if _, err := exec.LookPath("cc"); err != nil {
					t.Skip("no C compiler on PATH")
				}
			}
			s := loadCompileFixture(t)
			exes := map[string]string{}
			dir := t.TempDir()
			for _, c := range compiledCases {
				exe, built := exes[c.calc]
				if !built {
					program, err := s.CompileCalc("Compiled::" + c.calc)
					if err != nil {
						t.Fatalf("compile %s: %v", c.calc, err)
					}
					exe = filepath.Join(dir, c.calc)
					if err := codegen.Build(program, target, exe); err != nil {
						t.Fatalf("build %s: %v", c.calc, err)
					}
					exes[c.calc] = exe
				}
				wantValue, wantFailure := interpreted(t, s, c)
				gotValue, gotFailure := compiledRun(t, exe, c)
				if gotValue != wantValue || gotFailure != wantFailure {
					t.Errorf("%s(%s): compiled = (%q, %q), interpreted = (%q, %q)",
						c.calc, strings.Join(c.args, ", "), gotValue, gotFailure, wantValue, wantFailure)
				}
			}
			for _, repeat := range []string{"0", "-1", "x", "2x", ""} {
				out, err := exec.Command(exes["Fib"], "--repeat", repeat, "10").CombinedOutput()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 2 {
					t.Errorf("--repeat %q: got %v %q, want usage and exit status 2", repeat, err, out)
				}
			}
			if out, err := exec.Command(exes["Fib"], "--repeat", "3", "10").Output(); err != nil || strings.TrimSpace(string(out)) != "55" {
				t.Errorf("--repeat 3: got %q, %v", out, err)
			}
		})
	}
}

// A calc outside the subset is refused with the reason, never compiled wrong.
func TestCompileRefusesWhatItCannotCompile(t *testing.T) {
	s := loadCompileFixture(t)
	for _, tc := range []struct{ calc, reason string }{
		{"StringResult", "String"},
		{"Sequence", "library functions"},
		{"Defaulted", "default value"},
		{"OutOnly", "`out`"},
		{"Library", "library functions"},
		{"Refined", "members of its own"},
		{"DynamicIntPow", "non-literal Integer exponent"},
		{"Narrowed", "Real assigned to x"},
		{"Unbound", "attribute x has no value"},
	} {
		_, err := s.CompileCalc("Refused::" + tc.calc)
		if err == nil {
			t.Errorf("%s: compiled, want a refusal mentioning %q", tc.calc, tc.reason)
			continue
		}
		if !errors.Is(err, codegen.ErrUnsupported) {
			t.Errorf("%s: %v is not an ErrUnsupported", tc.calc, err)
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: %v does not mention %q", tc.calc, err, tc.reason)
		}
	}
	if _, err := s.CompileCalc("Compiled::Nowhere"); err == nil {
		t.Error("an unknown calc compiled")
	}
}

// The generated source is deterministic and names the calc it came from.
func TestCompiledSourceNamesTheCalc(t *testing.T) {
	s := loadCompileFixture(t)
	program, err := s.CompileCalc("Compiled::Fib")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range codegen.Targets() {
		src, err := codegen.Source(program, target)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := codegen.Source(program, target)
		if string(src) != string(again) {
			t.Errorf("%s: two renderings differ", target)
		}
		if !strings.Contains(string(src), "Compiled::Fib") {
			t.Errorf("%s: source does not name Compiled::Fib", target)
		}
	}
}
