package modules

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Sources are the pinned inputs a generation reads, recorded in every output
// so a consumer can tell which upstream revision a module came from.
type Sources struct {
	OntologyRepo   string
	OntologyCommit string
	XMIRelease     string
}

// Output is one generated file, relative to the output directory.
type Output struct {
	Path    string
	Content []byte
}

// CatalogFile is the OASIS XML catalog OWL tools (Protégé, the OWL API)
// read from an ontology's directory to resolve owl:imports to local files.
const CatalogFile = "catalog-v001.xml"

// Generate turns a partition into the module documents, the layer documents
// that import them, a catalog mapping every term to its module, an XML
// catalog mapping every ontology IRI to its file, and a VERSION file
// recording the sources. Each ontology's owl:versionInfo is
// "<ontology commit>+xmi<release>".
func Generate(p *Partition, baseIRI string, src Sources) ([]Output, error) {
	if !strings.HasSuffix(baseIRI, "/") {
		return nil, fmt.Errorf("base IRI %q must end in /", baseIRI)
	}
	if src.OntologyRepo == "" || src.OntologyCommit == "" || src.XMIRelease == "" {
		return nil, fmt.Errorf("sources must name the ontology repository, its commit and the XMI release")
	}
	versionInfo := src.OntologyCommit + "+xmi" + src.XMIRelease
	provenance := func(path string) []Triple {
		ont := IRI(baseIRI + path)
		return []Triple{
			{ont, IRI(RDFNS + "type"), IRI(OWLNS + "Ontology")},
			{ont, IRI(OWLNS + "versionInfo"), Literal(versionInfo, "")},
			{ont, IRI(DCNS + "source"), Literal(src.OntologyRepo+"@"+src.OntologyCommit, "")},
			{ont, IRI(DCNS + "source"), Literal("OMG KerML/SysML XMI "+src.XMIRelease, "")},
		}
	}
	var outputs []Output
	for _, mod := range p.Modules {
		ont := IRI(baseIRI + mod.Path)
		triples := provenance(mod.Path)
		triples = append(triples, Triple{ont, IRI(DCNS + "title"), Literal(mod.Path, "")})
		if mod.Comment != "" {
			triples = append(triples, Triple{ont, IRI(DCNS + "description"), Literal(mod.Comment, "")})
		}
		for _, imp := range mod.Imports {
			triples = append(triples, Triple{ont, IRI(OWLNS + "imports"), IRI(baseIRI + imp)})
		}
		if mod.ImportsEcore {
			triples = append(triples, Triple{ont, IRI(OWLNS + "imports"), IRI(EcoreOntology)})
		}
		triples = append(triples, mod.Triples...)
		doc, err := WriteTurtle(triples)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", mod.Path, err)
		}
		outputs = append(outputs, Output{Path: mod.Path + ".ttl", Content: doc})
	}
	for _, layer := range p.Layers {
		ont := IRI(baseIRI + layer.Path)
		triples := provenance(layer.Path)
		triples = append(triples, Triple{ont, IRI(DCNS + "title"), Literal(layer.Path, "")})
		if layer.Comment != "" {
			triples = append(triples, Triple{ont, IRI(DCNS + "description"), Literal(layer.Comment, "")})
		}
		for _, child := range layer.Children {
			triples = append(triples, Triple{ont, IRI(OWLNS + "imports"), IRI(baseIRI + child)})
		}
		doc, err := WriteTurtle(triples)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", layer.Path, err)
		}
		outputs = append(outputs, Output{Path: layer.Path + ".ttl", Content: doc})
	}
	iriCatalog, err := xmlCatalog(p, baseIRI)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs,
		Output{Path: "catalog.tsv", Content: catalog(p)},
		Output{Path: CatalogFile, Content: iriCatalog},
		Output{Path: "VERSION", Content: []byte(fmt.Sprintf(
			"version\t%s\nontology-repo\t%s\nontology-commit\t%s\nxmi-release\t%s\nbase-iri\t%s\n",
			versionInfo, src.OntologyRepo, src.OntologyCommit, src.XMIRelease, baseIRI))},
	)
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].Path < outputs[j].Path })
	return outputs, nil
}

// catalog lists every declared term with the module that declares it, one
// per line, sorted by term.
func catalog(p *Partition) []byte {
	var b bytes.Buffer
	b.WriteString("term\tmodule\n")
	type row struct{ term, module string }
	var rows []row
	for _, mod := range p.Modules {
		for _, term := range mod.Terms {
			rows = append(rows, row{strings.TrimPrefix(term, SysMLNS), mod.Path})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].term < rows[j].term })
	for _, r := range rows {
		fmt.Fprintf(&b, "%s\t%s\n", r.term, r.module)
	}
	return b.Bytes()
}

type catalogEntry struct {
	Name string `xml:"name,attr"`
	URI  string `xml:"uri,attr"`
}

type oasisCatalog struct {
	XMLName xml.Name       `xml:"urn:oasis:names:tc:entity:xmlns:xml:catalog catalog"`
	Prefer  string         `xml:"prefer,attr"`
	URIs    []catalogEntry `xml:"uri"`
}

// xmlCatalog maps each module and layer IRI to its .ttl, so a tool opening
// any module from disk resolves the whole import closure without network.
func xmlCatalog(p *Partition, baseIRI string) ([]byte, error) {
	cat := oasisCatalog{Prefer: "public"}
	for _, mod := range p.Modules {
		cat.URIs = append(cat.URIs, catalogEntry{baseIRI + mod.Path, mod.Path + ".ttl"})
	}
	for _, layer := range p.Layers {
		cat.URIs = append(cat.URIs, catalogEntry{baseIRI + layer.Path, layer.Path + ".ttl"})
	}
	sort.Slice(cat.URIs, func(i, j int) bool { return cat.URIs[i].Name < cat.URIs[j].Name })
	body, err := xml.MarshalIndent(cat, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(append([]byte(xml.Header), body...), '\n'), nil
}

// WriteOutputs replaces the generated files under dir with outputs, staging
// them in a sibling directory first so a failed write leaves dir untouched.
func WriteOutputs(dir string, outputs []Output) (err error) {
	existing, err := listOutputs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+"-staging-")
	if err != nil {
		return err
	}
	defer func() {
		if rerr := os.RemoveAll(staging); err == nil {
			err = rerr
		}
	}()
	for _, o := range outputs {
		full := filepath.Join(staging, filepath.FromSlash(o.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return err
		}
		err := os.WriteFile(full, o.Content, 0o644) // #nosec G306 -- committed source artifacts are meant to be readable
		if err != nil {
			return err
		}
	}
	for _, path := range existing {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
			return err
		}
	}
	for _, o := range outputs {
		rel := filepath.FromSlash(o.Path)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o750); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(staging, rel), filepath.Join(dir, rel)); err != nil {
			return err
		}
	}
	return nil
}

// CheckOutputs compares outputs with the files under dir and returns the
// paths that differ, are missing, or are present but not generated.
func CheckOutputs(dir string, outputs []Output) ([]string, error) {
	existing, err := listOutputs(dir)
	if err != nil {
		return nil, err
	}
	want := make(map[string]Output, len(outputs))
	for _, o := range outputs {
		want[o.Path] = o
	}
	var stale []string
	for _, path := range existing {
		o, ok := want[path]
		if !ok {
			stale = append(stale, path+" (not generated)")
			continue
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path))) // #nosec G304 -- path came from listing dir
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(got, o.Content) {
			stale = append(stale, path+" (differs)")
		}
		delete(want, path)
	}
	for _, path := range sortedKeys(want) {
		stale = append(stale, path+" (missing)")
	}
	sort.Strings(stale)
	return stale, nil
}

func listOutputs(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == dir {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, ".ttl") || rel == "catalog.tsv" || rel == CatalogFile || rel == "VERSION" {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
