package libs

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// TestErrorDetail shows detailed diagnostics for files with specific errors
func TestErrorDetail(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	targetError := "expected '{' or ';' after declaration"
	type diagInfo struct {
		file   string
		offset int
		msg    string
		snip   string
	}

	var diags []diagInfo

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			continue
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		hasTarget := false
		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, targetError) {
				hasTarget = true
				offset := d.Span.Offset
				snip := ""
				if offset >= 0 && offset < len(data) {
					start := offset
					if start > 60 {
						start = offset - 60
					} else {
						start = 0
					}
					end := offset + 60
					if end > len(data) {
						end = len(data)
					}
					snip = string(data[start:end])
					snip = strings.ReplaceAll(snip, "\n", "\\n")
					snip = strings.ReplaceAll(snip, "\t", "\\t")
				}
				diags = append(diags, diagInfo{
					file:   name,
					offset: offset,
					msg:    d.Message,
					snip:   snip,
				})
			}
		}

		if hasTarget {
			t.Logf("File: %s", name)
		}
	}

	// Sort by file, then offset
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].file != diags[j].file {
			return diags[i].file < diags[j].file
		}
		return diags[i].offset < diags[j].offset
	})

	t.Logf("\n=== Detailed Diagnostics for '%s' ===", targetError)
	t.Logf("Total occurrences: %d", len(diags))

	currentFile := ""
	for _, d := range diags {
		if d.file != currentFile {
			currentFile = d.file
			t.Logf("\n--- %s ---", d.file)
		}
		t.Logf("  Offset %d: %s", d.offset, d.msg)
		if d.snip != "" {
			t.Logf("    Context: ...%s...", d.snip)
		}
	}
}

// TestBodyMemberErrorDetail shows detailed diagnostics for body member errors
func TestBodyMemberErrorDetail(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	targetError := "expected a body member"
	type diagInfo struct {
		file   string
		offset int
		msg    string
		snip   string
	}

	var diags []diagInfo

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			continue
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, targetError) {
				offset := d.Span.Offset
				snip := ""
				if offset >= 0 && offset < len(data) {
					start := offset
					if start > 60 {
						start = offset - 60
					} else {
						start = 0
					}
					end := offset + 60
					if end > len(data) {
						end = len(data)
					}
					snip = string(data[start:end])
					snip = strings.ReplaceAll(snip, "\n", "\\n")
					snip = strings.ReplaceAll(snip, "\t", "\\t")
				}
				diags = append(diags, diagInfo{
					file:   name,
					offset: offset,
					msg:    d.Message,
					snip:   snip,
				})
			}
		}
	}

	// Sort by file, then offset
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].file != diags[j].file {
			return diags[i].file < diags[j].file
		}
		return diags[i].offset < diags[j].offset
	})

	t.Logf("\n=== Detailed Diagnostics for '%s' ===", targetError)
	t.Logf("Total occurrences: %d", len(diags))

	// Show first 30
	for i, d := range diags {
		if i >= 30 {
			t.Logf("\n... and %d more", len(diags)-30)
			break
		}
		if i == 0 || diags[i-1].file != d.file {
			t.Logf("\n--- %s ---", d.file)
		}
		t.Logf("  Offset %d: %s", d.offset, d.msg)
		if d.snip != "" {
			// Clean up snippet for readability
			snip := d.snip
			if len(snip) > 100 {
				snip = snip[:100] + "..."
			}
			t.Logf("    Context: ...%s...", snip)
		}
	}

	// Group by pattern in first 20 chars of context
	patterns := make(map[string]int)
	for _, d := range diags {
		pattern := d.snip
		if len(pattern) > 30 {
			pattern = pattern[:30]
		}
		// Extract keyword at start
		parts := strings.Fields(pattern)
		if len(parts) > 0 {
			patterns[parts[0]]++
		}
	}

	t.Logf("\n\n=== Pattern frequency (by first keyword) ===")
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range patterns {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})
	for i, p := range sorted {
		if i >= 15 {
			break
		}
		t.Logf("  %3d: %s", p.v, p.k)
	}
}
