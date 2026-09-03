// Command ontology-modules splits the Open-MBEE SysML v2 OWL ontology into
// one Turtle module per package of the normative KerML/SysML metamodel.
//
// It reads the sources staged by scripts/download-ontology-sources.sh and
// writes the modules, a term catalog and a VERSION file; -check instead
// reports which committed files are stale.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology/modules"
)

const (
	ontologyRepo = "https://github.com/Open-MBEE/sysmlv2-rdf-ontology"
	defaultBase  = "https://www.omg.org/spec/SysML/modules/"
)

func main() {
	var (
		sources = flag.String("sources", "build/ontology-sources", "directory staged by scripts/download-ontology-sources.sh")
		out     = flag.String("out", "ontology/sysmlv2", "directory to write the modules to")
		base    = flag.String("base", defaultBase, "base IRI of the module ontologies")
		check   = flag.Bool("check", false, "report stale files under -out instead of writing")
	)
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "ontology-modules takes no arguments")
		os.Exit(2)
	}
	if err := run(*sources, *out, *base, *check); err != nil {
		fmt.Fprintln(os.Stderr, "ontology-modules:", err)
		os.Exit(1)
	}
}

func run(sources, out, base string, check bool) error {
	pin, err := readPin(filepath.Join(sources, ".pin"))
	if err != nil {
		return err
	}
	var triples []modules.Triple
	err = withFile(filepath.Join(sources, "sysmlv2-rdf-ontology", "sysml2", "owl", "www.omg.org", "spec", "SysML.owl"), func(r io.Reader) error {
		triples, err = modules.ReadRDFXML(r)
		return err
	})
	if err != nil {
		return err
	}
	meta := &modules.Metamodel{}
	for _, name := range []string{"KerML.xmi", "SysML.xmi"} {
		if err := withFile(filepath.Join(sources, "omg-xmi", name), meta.ReadXMI); err != nil {
			return err
		}
	}
	partition, err := meta.Partition(triples)
	if err != nil {
		return err
	}
	outputs, err := modules.Generate(partition, base, pin)
	if err != nil {
		return err
	}
	if check {
		stale, err := modules.CheckOutputs(out, outputs)
		if err != nil {
			return err
		}
		if len(stale) > 0 {
			return fmt.Errorf("%s is stale; run make ontology-modules:\n  %s", out, strings.Join(stale, "\n  "))
		}
		fmt.Printf("%s is up to date (%d files)\n", out, len(outputs))
		return nil
	}
	if err := modules.WriteOutputs(out, outputs); err != nil {
		return err
	}
	fmt.Printf("wrote %d files to %s (%d modules, %d layers)\n", len(outputs), out, len(partition.Modules), len(partition.Layers))
	return nil
}

// withFile runs read over the named file and reports Close failures too.
func withFile(path string, read func(io.Reader) error) (err error) {
	f, err := os.Open(path) // #nosec G304 -- the operator names the staged source tree via -sources
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	if err := read(f); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// readPin parses the .pin file the download script writes:
// "<ontology commit> <xmi release> <kerml sha256> <sysml sha256>".
func readPin(path string) (modules.Sources, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the operator names the staged source tree via -sources
	if err != nil {
		return modules.Sources{}, fmt.Errorf("%w (run scripts/download-ontology-sources.sh)", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		return modules.Sources{}, fmt.Errorf("%s: want 4 fields, got %d", path, len(fields))
	}
	return modules.Sources{OntologyRepo: ontologyRepo, OntologyCommit: fields[0], XMIRelease: fields[1]}, nil
}
