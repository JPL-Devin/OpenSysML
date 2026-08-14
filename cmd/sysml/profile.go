package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// Profiling flags. A profile is about a whole run, so it is started once the
// command line is understood and written when the run ends, whatever it decided.
var (
	cpuProfilePath string
	memProfilePath string
	memStats       bool
)

// startProfiling begins the profiles the command line asked for and returns the
// function that ends them, which writes what they recorded. Nothing was asked
// for is not an error: the returned function is then a no-op, so a run that
// profiles nothing takes the same path as one that does.
func startProfiling() (func(), error) {
	started := time.Now()
	var ends []func()

	// The ends run in reverse of the order they were added, so each profile is
	// written before the one started before it is stopped.
	stop := func() {
		for i := len(ends) - 1; i >= 0; i-- {
			ends[i]()
		}
	}

	if cpuProfilePath != "" {
		// #nosec G304 -- the profile is written where the command line says.
		f, err := os.Create(cpuProfilePath)
		if err != nil {
			return func() {}, fmt.Errorf("-cpuprofile: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return func() {}, fmt.Errorf("-cpuprofile: %w", err)
		}
		ends = append(ends, func() {
			pprof.StopCPUProfile()
			if err := f.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "sysml: -cpuprofile:", err)
			}
		})
	}

	if memProfilePath != "" {
		// #nosec G304 -- the profile is written where the command line says.
		f, err := os.Create(memProfilePath)
		if err != nil {
			stop()
			return func() {}, fmt.Errorf("-memprofile: %w", err)
		}
		ends = append(ends, func() {
			// The profile is written where the run ends, at which point the model it
			// loaded is unreachable: what it records of use is where the run allocated,
			// read with `go tool pprof -sample_index=alloc_space`.
			if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
				fmt.Fprintln(os.Stderr, "sysml: -memprofile:", err)
			}
			if err := f.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "sysml: -memprofile:", err)
			}
		})
	}

	if memStats {
		ends = append(ends, func() { reportMemStats(os.Stderr, time.Since(started)) })
	}

	return stop, nil
}

// reportMemStats reports what the run cost: the time it took, the memory it
// allocated over its whole course, and the memory it took from the operating
// system. Total allocation is the pressure the run put on the collector; what
// the run took from the OS is a floor on its peak resident size, which is what
// bounds the model a machine can load. Neither is the live size of the loaded
// model: by the time a run ends the model is unreachable, so what a model costs
// while held is measured by the benchmarks, which keep it alive.
func reportMemStats(w io.Writer, elapsed time.Duration) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(w, "sysml: %s wall, %s allocated in %d allocations over %d collections, %s taken from the OS\n",
		elapsed.Round(time.Millisecond), humanBytes(m.TotalAlloc), m.Mallocs,
		m.NumGC, humanBytes(m.Sys))
}

// humanBytes writes a byte count the way a reader compares two of them: in the
// largest unit that leaves a number with a whole part.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value)
}
