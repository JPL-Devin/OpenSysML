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
	"strings"
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
		protocols = flag.String("protocols", "grpc,connect,connect-json", "Comma-separated protocols to test")
		transport = flag.String("transport", "connect", "server transport (connect or grpc)")
	)
	flag.Parse()

	if err := runSuite(*dir, *binary, *repoRoot, *report, *run, *verbose, *allowSkip, *protocols, *transport); err != nil {
		fmt.Fprintf(os.Stderr, "conformance: %v\n", err)
		os.Exit(1)
	}
}

func runSuite(dir, binary, repoRoot, report, run string, verbose, allowSkip bool, protocolList, transport string) error {
	protocols, err := parseProtocols(protocolList)
	if err != nil {
		return err
	}
	if transport != transportConnect && transport != transportGRPC {
		return fmt.Errorf("unknown -transport %q; want connect or grpc", transport)
	}
	if err := validateProtocols(protocols, transport); err != nil {
		return err
	}
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
	svc, err := startServiceWithTransport(ctx, binary, filepath.Join(workDir, "service.log"), len(scenarios)+16, transport)
	if err != nil {
		return err
	}
	defer svc.stop()

	reportData := &Report{Service: svc.binary}
	for _, protocol := range protocols {
		c, err := svc.client(protocol)
		if err != nil {
			return err
		}
		runner := &runner{
			service: svc, client: c,
			fixtures: filepath.Join(dir, "fixtures"),
			models:   map[Model]string{}, verbose: verbose,
			out: os.Stdout, scenarioLog: os.Stdout,
		}
		if err := runner.readCapabilities(ctx); err != nil {
			return err
		}
		summary := runner.runAll(ctx, scenarios, filter)
		c.close()
		reportData.Protocols = append(reportData.Protocols, summary)
		reportData.Total += summary.Total
		reportData.Passed += summary.Passed
		reportData.Failed += summary.Failed
		reportData.Skipped += summary.Skipped
		reportData.Errored += summary.Errored
		if summary.Failed > 0 || summary.Errored > 0 {
			reportData.failure = true
		}
		if summary.Skipped > 0 {
			reportData.hasSkip = true
		}
	}
	if err := writeReport(report, reportData); err != nil {
		return err
	}
	if reportData.failure {
		return fmt.Errorf("%d of %d scenarios failed", reportData.Failed+reportData.Errored, reportData.Total)
	}
	if reportData.hasSkip && !allowSkip {
		return fmt.Errorf("%d scenarios were skipped for missing capabilities; pass -allow-skips to accept that", reportData.Skipped)
	}
	return nil
}

func validateProtocols(protocols []string, transport string) error {
	if transport == transportGRPC {
		for _, protocol := range protocols {
			if protocol != "grpc" {
				return fmt.Errorf("protocol %q requires -transport connect; grpc transport only supports grpc", protocol)
			}
		}
	}
	return nil
}

const (
	transportConnect = "connect"
	transportGRPC    = "grpc"
)

var knownProtocols = map[string]struct{}{"grpc": {}, "connect": {}, "connect-json": {}}

func parseProtocols(value string) ([]string, error) {
	var protocols []string
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := knownProtocols[item]; !ok {
			return nil, fmt.Errorf("unknown protocol %q; want grpc, connect or connect-json", item)
		}
		if _, ok := seen[item]; ok {
			return nil, fmt.Errorf("protocol %q is listed more than once", item)
		}
		seen[item] = struct{}{}
		protocols = append(protocols, item)
	}
	if len(protocols) == 0 {
		return nil, errors.New("-protocols must name at least one protocol")
	}
	return protocols, nil
}

type Report struct {
	Service   string     `json:"service"`
	Total     int        `json:"total"`
	Passed    int        `json:"passed"`
	Failed    int        `json:"failed"`
	Skipped   int        `json:"skipped"`
	Errored   int        `json:"errored"`
	Protocols []*Summary `json:"protocols"`
	failure   bool
	hasSkip   bool
}

// writeReport writes the summary as JSON, to a file or to stdout.
func writeReport(path string, summary *Report) error {
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
