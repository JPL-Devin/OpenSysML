package libs

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestErrorPatternAnalysis provides detailed analysis of remaining parse errors
func TestErrorPatternAnalysis(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	type ErrorInfo struct {
		File    string
		Offset  int
		Message string
		Context string
	}

	var allErrors []ErrorInfo

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			continue
		}

		sf := source.New(name, data)
		p := parser.New(sf)
		_ = p.ParseFile()

		if len(p.Diagnostics) == 0 {
			continue
		}

		for _, d := range p.Diagnostics {
			start := d.Span.Offset
			end := start + 120
			if end > len(data) {
				end = len(data)
			}
			ctx := string(data[start:end])
			ctx = strings.ReplaceAll(ctx, "\n", " ")
			ctx = strings.ReplaceAll(ctx, "\t", " ")

			allErrors = append(allErrors, ErrorInfo{
				File:    name,
				Offset:  start,
				Message: d.Message,
				Context: ctx,
			})
		}
	}

	// Group by message
	msgMap := make(map[string][]ErrorInfo)
	for _, e := range allErrors {
		msgMap[e.Message] = append(msgMap[e.Message], e)
	}

	// Sort by frequency
	type MsgCount struct {
		Msg   string
		Count int
	}
	var msgCounts []MsgCount
	for msg, errs := range msgMap {
		msgCounts = append(msgCounts, MsgCount{msg, len(errs)})
	}
	sort.Slice(msgCounts, func(i, j int) bool {
		return msgCounts[i].Count > msgCounts[j].Count
	})

	t.Logf("\n=== Top Error Messages with Context ===\n")
	for i, mc := range msgCounts {
		if i >= 8 {
			break
		}
		t.Logf("\n%d: %s", mc.Count, mc.Msg)
		examples := msgMap[mc.Msg]
		showCount := 5
		if len(examples) < showCount {
			showCount = len(examples)
		}
		for j := 0; j < showCount; j++ {
			shortFile := examples[j].File
			if idx := strings.LastIndex(shortFile, "/"); idx >= 0 {
				shortFile = shortFile[idx+1:]
			}
			t.Logf("  [%s:%d]", shortFile, examples[j].Offset)
			t.Logf("    %s", examples[j].Context)
		}
	}

	// Analyze "expected body member" specifically
	t.Logf("\n\n=== Detailed Analysis: 'expected a body member' ===\n")
	bodyMemberErrors := msgMap["expected a body member"]

	// Pattern categorization
	patterns := make(map[string][]ErrorInfo)
	for _, e := range bodyMemberErrors {
		ctx := e.Context

		// Categorize by first keyword/token
		firstToken := ""
		parts := strings.Fields(ctx)
		if len(parts) > 0 {
			firstToken = parts[0]
		}

		category := "other"
		if strings.HasPrefix(ctx, "subset ") || strings.HasPrefix(ctx, "disjoint ") {
			category = "constraint_statement"
		} else if strings.HasPrefix(ctx, "redefines ") {
			category = "redefines_statement"
		} else if strings.HasPrefix(ctx, "from ") || strings.HasPrefix(ctx, "to ") {
			category = "connector_end"
		} else if strings.Contains(ctx, "occurrence ") && (strings.HasPrefix(ctx, "end ") || strings.HasPrefix(ctx, "from ") || strings.HasPrefix(ctx, "to ")) {
			category = "end_occurrence"
		} else {
			category = fmt.Sprintf("token_%s", firstToken)
		}

		patterns[category] = append(patterns[category], e)
	}

	// Sort categories by count
	type CatCount struct {
		Cat   string
		Count int
	}
	var catCounts []CatCount
	for cat, errs := range patterns {
		catCounts = append(catCounts, CatCount{cat, len(errs)})
	}
	sort.Slice(catCounts, func(i, j int) bool {
		return catCounts[i].Count > catCounts[j].Count
	})

	for _, cc := range catCounts {
		t.Logf("\n  Category: %s (%d occurrences)", cc.Cat, cc.Count)
		examples := patterns[cc.Cat]
		showCount := 3
		if len(examples) < showCount {
			showCount = len(examples)
		}
		for i := 0; i < showCount; i++ {
			shortFile := examples[i].File
			if idx := strings.LastIndex(shortFile, "/"); idx >= 0 {
				shortFile = shortFile[idx+1:]
			}
			t.Logf("    [%s] %s", shortFile, examples[i].Context)
		}
	}
}
