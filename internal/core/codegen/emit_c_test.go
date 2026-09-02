package codegen

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// TestCPrintRealMatchesFormatReal drives the prelude's sysml_print_real over
// every power of two and its neighbours, where the shortest spelling is
// hardest to pick, plus random bit patterns, and compares with FormatReal.
func TestCPrintRealMatchesFormatReal(t *testing.T) {
	cc := os.Getenv(CCompilerEnvVar)
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skip("no C compiler on PATH")
	}

	var values []float64
	for e := -1074; e <= 1023; e++ {
		p := math.Ldexp(1, e)
		values = append(values, p, -p, math.Nextafter(p, 0), math.Nextafter(p, math.Inf(1)))
	}
	rng := rand.New(rand.NewSource(20260902))
	for len(values) < 20000 {
		f := math.Float64frombits(rng.Uint64())
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			values = append(values, f)
		}
	}
	for _, f := range []float64{0, 0.1, 0.3, 0.1 + 0.2, 1e-4, 1e21, 9.5e20, 123456789012345680, 5e-324, math.MaxFloat64} {
		values = append(values, f, -f)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "print.c")
	program := fmt.Sprintf("#define SYSML_MAX_CALC_DEPTH %d\n", runtime.DefaultMaxCalcDepth) + cPrelude + `
int main(void) {
	char line[64];
	while (fgets(line, sizeof line, stdin)) {
		uint64_t bits = strtoull(line, NULL, 16);
		sysml_real r;
		memcpy(&r, &bits, sizeof r);
		sysml_print_real(r);
	}
	return 0;
}
`
	if err := os.WriteFile(src, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "print")
	build := exec.Command(cc, append(append([]string{}, CFlags...), "-o", bin, src, "-lm")...) // #nosec G204 -- test-owned arguments
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}

	var in bytes.Buffer
	for _, f := range values {
		fmt.Fprintf(&in, "%016x\n", math.Float64bits(f))
	}
	run := exec.Command(bin)
	run.Stdin = &in
	out, err := run.Output()
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	mismatches := 0
	for i, f := range values {
		if !sc.Scan() {
			t.Fatalf("output ended after %d of %d values", i, len(values))
		}
		if got, want := strings.TrimSpace(sc.Text()), runtime.FormatReal(f); got != want {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("%016x: C prints %s, FormatReal %s", math.Float64bits(f), got, want)
			}
		}
	}
	if mismatches > 10 {
		t.Errorf("%d mismatches in total", mismatches)
	}
}
