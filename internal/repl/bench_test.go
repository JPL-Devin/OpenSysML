package repl

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// Benchmarks over synthetic models of a stated size, so that what loading and
// running a large model costs can be read as a rate rather than as one number:
// a cost that grows with the square of the model is visible only across sizes.
//
//	go test ./internal/repl -run '^$' -bench . -benchmem
//	go test ./internal/repl -run '^$' -bench BenchmarkLoadModel -memprofile heap.out
//
// Each benchmark reports two figures beyond the standard ones:
//
//	B/element   memory allocated per model element, which should not grow with
//	            the size of the model
//	live-B/op   memory the loaded model holds once loading is done, measured with
//	            the session still reachable, which is what bounds the model a
//	            machine can hold at once
const benchElementsPerPart = 5 // part def, calc def, action def, state machine, part usage

// modelSizes are the element counts the load benchmarks run at. They double, so
// a super-linear cost shows as a per-element figure that grows with size.
var modelSizes = []int{50, 200, 800}

// emptyModel is a model with no elements, whose load cost is what a session
// costs before it holds anything: the standard library it indexes to resolve
// names against. Reading the figures for a model of n elements against this one
// separates what the model costs from what the session costs.
const emptyModel = "package BenchModel {\n    import ScalarValues::*;\n}\n"

// syntheticModel returns a model of parts times the repeating group of a part
// definition, a calculation, an action, a state machine and a part usage of it.
// Each part refers to the next, so name resolution has work to do that grows
// with the model rather than a model of unrelated declarations.
func syntheticModel(parts int) string {
	var b strings.Builder
	b.WriteString("package BenchModel {\n    import ScalarValues::*;\n")
	for i := 0; i < parts; i++ {
		fmt.Fprintf(&b, `
    part def Comp%[1]d {
        attribute mass : Real;
        attribute power : Real;
        part sub : Comp%[2]d;
        constraint MassOK {
            assert mass > 0.0;
        }
    }
    calc def Calc%[1]d {
        in a : Real;
        in b : Real;
        return : Real = a * b + %[1]d;
    }
    action def Act%[1]d {
        in x : Real;
        out y : Real;
        bind y = x * 2.0;
    }
    state SM%[1]d {
        initial start;
        state idle;
        state running;
        final done;

        start then idle;
        transition idle to running;
        transition running to done;
    }
    part inst%[1]d : Comp%[1]d {
        attribute :>> mass = %[1]d.0;
        attribute :>> power = %[1]d.5;
    }
`, i, (i+1)%parts)
	}
	b.WriteString("}\n")
	return b.String()
}

// loadModel loads src into a fresh session, failing the benchmark if the model
// does not analyse cleanly: a model that errors out skips the later passes, so
// its cost is not the cost of loading a model.
func loadModel(tb testing.TB, src string) *Session {
	tb.Helper()
	sess := NewSession()
	sess.SubmitFiles([]SourceFile{{Name: "bench.sysml", Text: src}})
	if sess.HasErrors() {
		tb.Fatalf("the benchmark model did not analyse cleanly:\n%s", strings.Join(sess.DiagnosticLines(), "\n"))
	}
	return sess
}

// liveHeap returns the reachable heap, collecting first so that what it reports
// is what is held rather than what has not yet been collected.
func liveHeap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// BenchmarkLoadModel measures parsing, name resolution and the validation passes
// over a whole model, which is what `sysml -validate` spends its time in.
func BenchmarkLoadModel(b *testing.B) {
	sizes := append([]int{0}, modelSizes...)
	for _, parts := range sizes {
		src := emptyModel
		if parts > 0 {
			src = syntheticModel(parts)
		}
		elements := parts * benchElementsPerPart
		b.Run(fmt.Sprintf("elements=%d", elements), func(b *testing.B) {
			before := liveHeap()
			sess := loadModel(b, src)
			held := liveHeap() - before
			runtime.KeepAlive(sess)

			var start, end runtime.MemStats
			runtime.ReadMemStats(&start)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				loadModel(b, src)
			}
			b.StopTimer()
			runtime.ReadMemStats(&end)

			allocated := (end.TotalAlloc - start.TotalAlloc) / uint64(b.N)
			b.ReportMetric(float64(held), "live-B/op")
			if elements > 0 {
				b.ReportMetric(float64(allocated)/float64(elements), "B/element")
			}
		})
	}
}

// benchmarkRun measures one run over an already-loaded model, at each model
// size: what a single run costs should be about the behavior it runs, so a
// figure that grows with the size of the surrounding model is a cost the run
// pays for the model rather than for itself.
func benchmarkRun(b *testing.B, run func(b *testing.B, sess *Session)) {
	for _, parts := range modelSizes {
		src := syntheticModel(parts)
		b.Run(fmt.Sprintf("elements=%d", parts*benchElementsPerPart), func(b *testing.B) {
			sess := loadModel(b, src)
			// The session builds its runtime, and loads the standard library into
			// its index, when it is first run: that is a cost of the session, not
			// of a run, so it is paid before the measurement starts.
			run(b, sess)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				run(b, sess)
			}
		})
	}
}

// BenchmarkRunStateMachine measures starting a state machine in a loaded model.
func BenchmarkRunStateMachine(b *testing.B) {
	benchmarkRun(b, func(b *testing.B, sess *Session) {
		if v := sess.RunStateMachine("SM0"); !v.Holds() {
			b.Fatalf("SM0 did not run: %v", v.Lines)
		}
	})
}

// BenchmarkRunCalc measures evaluating a calculation in a loaded model, which is
// the expression evaluator over a model whose scopes it must search.
func BenchmarkRunCalc(b *testing.B) {
	benchmarkRun(b, func(b *testing.B, sess *Session) {
		if v := sess.RunCalc("Calc0(2.0, 3.0)"); !v.Holds() {
			b.Fatalf("Calc0 did not run: %v", v.Lines)
		}
	})
}

// BenchmarkInstantiate measures creating an object of a part definition, which
// is what a check about an object pays before it evaluates anything.
func BenchmarkInstantiate(b *testing.B) {
	benchmarkRun(b, func(b *testing.B, sess *Session) {
		if _, err := sess.InstantiateNamed("Comp0"); err != nil {
			b.Fatal(err)
		}
	})
}
