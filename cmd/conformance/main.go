// Command conformance runs the language-independent conformance suite in
// conformance/ against sysml-grpc, which it builds and starts itself.
//
// It is both the CI gate and the executable specification a client in another
// language ports: the scenarios are data, and this program is one reading of
// them. See conformance/README.md for the comparison rules it implements.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"

	_ "github.com/Open-MBEE/OpenSysML/api/proto" // registers the schema this runner reflects over
	"google.golang.org/protobuf/reflect/protoregistry"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// serviceName is the service every scenario addresses.
const serviceName = "sysml.SysMLService"

func main() {
	var (
		dir       = flag.String("dir", "conformance", "conformance suite directory (scenarios/ and fixtures/)")
		binary    = flag.String("binary", "", "sysml-grpc binary to test; built from ./cmd/sysml-grpc when empty")
		repoRoot  = flag.String("repo", ".", "repository root to build the service from")
		report    = flag.String("report", "", "write the machine-readable summary to this file (- for stdout)")
		run       = flag.String("run", "", "run only the scenarios whose id matches this regular expression")
		verbose   = flag.Bool("v", false, "print each scenario's normalized response")
		allowSkip = flag.Bool("allow-skips", false, "treat a scenario skipped for a missing capability as a pass")
	)
	flag.Parse()

	if err := runSuite(*dir, *binary, *repoRoot, *report, *run, *verbose, *allowSkip); err != nil {
		fmt.Fprintf(os.Stderr, "conformance: %v\n", err)
		os.Exit(1)
	}
}

func runSuite(dir, binary, repoRoot, report, run string, verbose, allowSkip bool) error {
	var filter *regexp.Regexp
	if run != "" {
		compiled, err := regexp.Compile(run)
		if err != nil {
			return fmt.Errorf("bad -run pattern: %w", err)
		}
		filter = compiled
	}

	scenarios, err := loadScenarios(filepath.Join(dir, "scenarios"))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workDir, err := os.MkdirTemp("", "conformance-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	if binary == "" {
		built, err := buildService(repoRoot, workDir)
		if err != nil {
			return err
		}
		binary = built
	}
	svc, err := startService(ctx, binary, filepath.Join(workDir, "service.log"), len(scenarios)+16)
	if err != nil {
		return err
	}
	defer svc.stop()

	runner := &runner{
		service:     svc,
		fixtures:    filepath.Join(dir, "fixtures"),
		models:      map[Model]string{},
		verbose:     verbose,
		out:         os.Stdout,
		scenarioLog: os.Stdout,
	}
	if err := runner.readCapabilities(ctx); err != nil {
		return err
	}

	summary := runner.runAll(ctx, scenarios, filter)
	if err := writeReport(report, summary); err != nil {
		return err
	}
	if summary.Failed > 0 || summary.Errored > 0 {
		return fmt.Errorf("%d of %d scenarios failed", summary.Failed+summary.Errored, summary.Total)
	}
	if summary.Skipped > 0 && !allowSkip {
		return fmt.Errorf("%d scenarios were skipped for missing capabilities; pass -allow-skips to accept that", summary.Skipped)
	}
	return nil
}

// writeReport writes the summary as JSON, to a file or to stdout.
func writeReport(path string, summary *Summary) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- a CI artifact is meant to be readable.
}

// methodByName resolves a method of the service from the compiled schema, so a
// scenario naming an RPC that does not exist is an error rather than a pass.
func methodByName(name string) (protoreflect.MethodDescriptor, error) {
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, fmt.Errorf("the schema for %s is not registered: %w", serviceName, err)
	}
	svc, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, errors.New(serviceName + " is not a service")
	}
	method := svc.Methods().ByName(protoreflect.Name(name))
	if method == nil {
		return nil, fmt.Errorf("%s has no RPC %q", serviceName, name)
	}
	return method, nil
}
