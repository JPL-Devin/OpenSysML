// Command pilot-reject checks the rejection direction the differential cannot
// see: it validates a hand-written negative corpus — models each violating one
// named rule — with both this implementation and the OMG SysML v2 Pilot
// Implementation, and buckets every case by who rejects it. A case the pilot
// rejects and we accept is a permissiveness gap.
//
// It is advisory: nothing in the build or the test suite depends on its
// verdicts. Provision the reference validators with
// scripts/download-pilot-sysml-validator.sh and
// scripts/download-pilot-kerml-validator.sh, then run
// `go run ./cmd/pilot-reject`. See docs/project/pilot-rejection.md.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/errata"
)

// bucket names one quadrant of the two validators' verdicts.
const (
	bucketBothReject = "both-reject"
	bucketPilotOnly  = "pilot-only-rejects"
	bucketOursOnly   = "ours-only-rejects"
	bucketBothAccept = "both-accept"
)

// languageBatch is the corpus files in a single language, in comparison order.
type languageBatch struct {
	Kind  source.Kind
	Files []string
}

func main() {
	repo := flag.String("repo", "", "repository root (default: the module root containing this command)")
	validator := flag.String("validator", "", "pilot SysML validator executable (default: <repo>/build/pilot-sysml-validator/validate-sysml-batch)")
	kermlValidator := flag.String("kerml-validator", "", "KerML pilot validator executable (default: <repo>/build/pilot-kerml-validator/validate-kerml)")
	corpus := flag.String("corpus", "", "negative corpus directory (default: <repo>/cmd/pilot-reject/testdata/negative)")
	out := flag.String("out", "", "output directory for the reports (default: <repo>/build/pilot-reject)")
	timeout := flag.Duration("timeout", 0, "per-batch timeout for the pilot validator (0: no limit)")
	policy := flag.String("conformance", policyAuto,
		"conformance mode our verdicts are taken under: auto (extensions/ strictly, the rest by default), default, or strict")
	update := flag.Bool("update", false, "record this run as "+committedBaseline)
	check := flag.Bool("check", false, "fail unless this run reproduces "+committedBaseline)
	flag.Parse()

	if err := run(*repo, *validator, *kermlValidator, *corpus, *out, *policy, *timeout, *update, *check); err != nil {
		fmt.Fprintf(os.Stderr, "pilot-reject: %v\n", err)
		os.Exit(1)
	}
}

func run(repo, validator, kermlValidator, corpus, out, policy string, timeout time.Duration, update, check bool) error {
	policy, err := parsePolicy(policy)
	if err != nil {
		return err
	}
	if repo == "" {
		repo, err = moduleRoot()
		if err != nil {
			return err
		}
	}
	if validator == "" {
		validator = filepath.Join(repo, "build", "pilot-sysml-validator", "validate-sysml-batch")
	}
	if kermlValidator == "" {
		kermlValidator = filepath.Join(repo, "build", "pilot-kerml-validator", "validate-kerml")
	}
	corpusDir := "cmd/pilot-reject/testdata/negative"
	if corpus != "" {
		corpusDir = relativeTo(repo, corpus)
	}
	if out == "" {
		out = filepath.Join(repo, "build", "pilot-reject")
	}
	files, err := collectCases(repo, corpusDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .sysml or .kerml files under %s", corpusDir)
	}

	batches := batchByLanguage(files)
	for _, batch := range batches {
		if _, err := os.Stat(pilotFor(batch.Kind, validator, kermlValidator)); err != nil {
			if batch.Kind == source.KindKerML {
				return fmt.Errorf("KerML pilot validator not found at %s: run ./scripts/download-pilot-kerml-validator.sh", kermlValidator)
			}
			return fmt.Errorf("pilot validator not found at %s: run ./scripts/download-pilot-sysml-validator.sh", validator)
		}
	}

	cases, err := adjudicate(repo, corpusDir, policy, validator, kermlValidator, files, batches, timeout)
	if err != nil {
		return err
	}

	overlay, err := errata.Load()
	if err != nil {
		return err
	}

	report := &Report{
		Validator:   relativeTo(repo, validator),
		Corpus:      corpusDir,
		Conformance: policy,
		Errata:      newErrataReport(overlay),
	}
	if report.Pilot, err = pilotVersion(validator); err != nil {
		return err
	}
	if report.Provenance, err = provenance(repo, report.Pilot, corpusDir, overlay, files); err != nil {
		return err
	}
	// Only a recorded baseline is dated, so two plain runs stay byte-identical.
	if update {
		report.Provenance.Recorded = baseline.Today()
	}
	for _, rel := range files {
		report.Cases = append(report.Cases, *cases[rel])
	}
	report.summarize()

	if err := runErrata(report, overlay, repo, corpusDir, policy, validator, kermlValidator, out, files, batches, timeout); err != nil {
		return err
	}
	fresh, err := writeReports(out, report)
	if err != nil {
		return err
	}
	committed := filepath.Join(repo, filepath.FromSlash(committedBaseline))
	if update {
		return baseline.Write(committed, fresh)
	}
	if check {
		return baseline.Reproduces(committed, fresh)
	}
	return nil
}

// adjudicate buckets every case of a corpus directory: ours in the policy's
// mode, ours in the default mode, and the pilot's, per language batch.
func adjudicate(repo, corpusDir, policy, validator, kermlValidator string,
	files []string, batches []languageBatch, timeout time.Duration) (map[string]*Case, error) {
	cases := make(map[string]*Case, len(files))
	for _, rel := range files {
		c, err := readCase(repo, corpusDir, rel)
		if err != nil {
			return nil, err
		}
		c.Mode = modeFor(policy, c.Source).String()
		cases[rel] = c
	}
	modes := make(map[string]conformance.Mode, len(cases))
	// The default mode is evaluated for every case as well, so a case asked
	// strictly reports what the default mode says instead of implying it agreed.
	defaults := make(map[string]conformance.Mode, len(cases))
	for rel, c := range cases {
		modes[rel] = modeFor(policy, c.Source)
		defaults[rel] = conformance.ModeDefault
	}

	for _, batch := range batches {
		fmt.Fprintf(os.Stderr, "negative corpus: %d %s case(s)\n", len(batch.Files), batch.Kind)
		pilot := pilotFor(batch.Kind, validator, kermlValidator)
		ours, err := openSysMLErrors(repo, corpusDir, batch.Files, modes)
		if err != nil {
			return nil, err
		}
		oursDefault, err := openSysMLErrors(repo, corpusDir, batch.Files, defaults)
		if err != nil {
			return nil, err
		}
		theirs, err := pilotErrors(pilot, repo, corpusDir, batch.Files, timeout)
		if err != nil {
			return nil, err
		}
		for _, rel := range batch.Files {
			classify(cases[rel], ours[rel], oursDefault[rel], theirs[rel])
		}
	}
	return cases, nil
}

// pilotFor picks the reference validator for a language batch.
func pilotFor(kind source.Kind, validator, kermlValidator string) string {
	if kind == source.KindKerML {
		return kermlValidator
	}
	return validator
}

// classify fills the case's verdicts. A side rejects when it reports at least
// one error-severity diagnostic; warnings do not count. oursDefault is what the
// default mode said, recorded for a case asked strictly so that a strict
// agreement does not read as a default one.
func classify(c *Case, ours, oursDefault, theirs []string) {
	c.OursErrors = len(ours)
	c.PilotErrors = len(theirs)
	c.Bucket = bucketOf(len(ours), len(theirs))
	switch c.Bucket {
	case bucketPilotOnly:
		c.Pilot = theirs
	case bucketOursOnly:
		c.Ours = ours
	}
	if c.Mode == conformance.ModeStrict.String() {
		c.DefaultErrors = len(oursDefault)
		c.DefaultBucket = bucketOf(len(oursDefault), len(theirs))
	}
}

// bucketOf names the quadrant a pair of error counts falls in.
func bucketOf(ours, theirs int) string {
	switch {
	case theirs > 0 && ours > 0:
		return bucketBothReject
	case theirs > 0:
		return bucketPilotOnly
	case ours > 0:
		return bucketOursOnly
	default:
		return bucketBothAccept
	}
}

// readCase reads one corpus file and its mandatory header: the first line must
// state the violated rule and its citation, so no case is anecdotal.
func readCase(repo, dir, rel string) (*Case, error) {
	// #nosec G304 -- the corpus root to validate is named on the command line.
	content, err := os.ReadFile(filepath.Join(repo, dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	first, _, _ := strings.Cut(string(content), "\n")
	rule, ok := strings.CutPrefix(first, "// Invalid: ")
	if !ok {
		return nil, fmt.Errorf("%s: first line must be `// Invalid: <rule> (<citation>).`", rel)
	}
	src, _, _ := strings.Cut(rel, "/")
	return &Case{Path: rel, Source: src, Rule: strings.TrimSpace(rule)}, nil
}

func relativeTo(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}

// moduleRoot walks up from the working directory to the directory holding go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above the working directory; pass -repo")
		}
		dir = parent
	}
}

// batchByLanguage splits the corpus into one batch per language, SysML first.
// Each language runs against its own reference validator; the cases are
// mutually independent, so batching only amortizes the validator's startup.
func batchByLanguage(files []string) []languageBatch {
	batches := []languageBatch{{Kind: source.KindSysML}, {Kind: source.KindKerML}}
	for _, rel := range files {
		for i := range batches {
			if batches[i].Kind == source.KindOf(rel) {
				batches[i].Files = append(batches[i].Files, rel)
			}
		}
	}
	out := make([]languageBatch, 0, len(batches))
	for _, batch := range batches {
		if len(batch.Files) > 0 {
			out = append(out, batch)
		}
	}
	return out
}

// collectCases returns the corpus files, in either language, as sorted
// slash-separated paths relative to the corpus directory.
func collectCases(repo, dir string) ([]string, error) {
	root := filepath.Join(repo, filepath.FromSlash(dir))
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if source.KindOf(path) == source.KindUnknown {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	sort.Strings(files)
	return files, nil
}
