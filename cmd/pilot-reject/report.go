package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Case is one negative model's adjudication. The error messages are kept only
// for the disagreeing buckets, where they are the evidence.
type Case struct {
	Path        string   `json:"path"`
	Source      string   `json:"source"`
	Rule        string   `json:"rule"`
	Bucket      string   `json:"bucket"`
	OursErrors  int      `json:"oursErrors"`
	PilotErrors int      `json:"pilotErrors"`
	Ours        []string `json:"ours,omitempty"`
	Pilot       []string `json:"pilot,omitempty"`
}

// Totals is one scope's bucket counts. PilotOnlyRejects are the permissiveness
// gaps this harness exists to surface.
type Totals struct {
	Cases            int `json:"cases"`
	BothReject       int `json:"bothReject"`
	PilotOnlyRejects int `json:"pilotOnlyRejects"`
	OursOnlyRejects  int `json:"oursOnlyRejects"`
	BothAccept       int `json:"bothAccept"`
}

// SourceTotals is the bucket counts for one derivation source of the corpus.
type SourceTotals struct {
	Source string `json:"source"`
	Totals
}

// Report is the whole run. It carries no timestamp and no absolute path, so
// two runs over the same pin produce byte-identical output.
type Report struct {
	Pilot     string         `json:"pilot"`
	Validator string         `json:"validator"`
	Corpus    string         `json:"corpus"`
	Totals    Totals         `json:"totals"`
	Sources   []SourceTotals `json:"sources"`
	Cases     []Case         `json:"cases"`
}

func (t *Totals) count(bucket string) {
	t.Cases++
	switch bucket {
	case bucketBothReject:
		t.BothReject++
	case bucketPilotOnly:
		t.PilotOnlyRejects++
	case bucketOursOnly:
		t.OursOnlyRejects++
	case bucketBothAccept:
		t.BothAccept++
	}
}

// summarize aggregates the totals and per-source totals from the cases.
func (r *Report) summarize() {
	bySource := map[string]*SourceTotals{}
	var names []string
	for _, c := range r.Cases {
		r.Totals.count(c.Bucket)
		st := bySource[c.Source]
		if st == nil {
			st = &SourceTotals{Source: c.Source}
			bySource[c.Source] = st
			names = append(names, c.Source)
		}
		st.count(c.Bucket)
	}
	sort.Strings(names)
	for _, name := range names {
		r.Sources = append(r.Sources, *bySource[name])
	}
}

func writeReports(dir string, report *Report) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	jsonPath := filepath.Join(dir, "pilot-reject.json")
	if err := os.WriteFile(jsonPath, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	textPath := filepath.Join(dir, "pilot-reject.txt")
	if err := os.WriteFile(textPath, []byte(renderText(report)), 0o600); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "wrote %s and %s\n", textPath, jsonPath)
	fmt.Fprintf(os.Stderr, "%s\n", headline(report.Totals))
	return nil
}

func headline(t Totals) string {
	return fmt.Sprintf("%d case(s): %d both reject, %d only the pilot rejects, %d only we reject, %d both accept",
		t.Cases, t.BothReject, t.PilotOnlyRejects, t.OursOnlyRejects, t.BothAccept)
}

func renderText(report *Report) string {
	var b strings.Builder
	b.WriteString("OpenSysML vs the OMG pilot implementation over a hand-written negative corpus\n")
	fmt.Fprintf(&b, "pilot pin: %s\nvalidator: %s\ncorpus:    %s\n\n", report.Pilot, report.Validator, report.Corpus)
	fmt.Fprintf(&b, "TOTAL: %s\n", headline(report.Totals))
	fmt.Fprintf(&b, "  %-12s %6s %11s %10s %9s %11s\n", "source", "cases", "both-reject", "pilot-only", "ours-only", "both-accept")
	for _, st := range report.Sources {
		fmt.Fprintf(&b, "  %-12s %6d %11d %10d %9d %11d\n",
			st.Source, st.Cases, st.BothReject, st.PilotOnlyRejects, st.OursOnlyRejects, st.BothAccept)
	}

	for _, c := range report.Cases {
		fmt.Fprintf(&b, "\n%s\n", c.Path)
		fmt.Fprintf(&b, "  rule:   %s\n", c.Rule)
		fmt.Fprintf(&b, "  bucket: %s (ours %d error(s), pilot %d error(s))\n", c.Bucket, c.OursErrors, c.PilotErrors)
		for _, msg := range c.Pilot {
			fmt.Fprintf(&b, "  pilot: %s\n", msg)
		}
		for _, msg := range c.Ours {
			fmt.Fprintf(&b, "  ours:  %s\n", msg)
		}
	}
	return b.String()
}
