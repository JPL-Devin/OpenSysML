package main

import (
	"strings"
	"testing"
)

// writtenHolderModel is a part whose features an action writes, one a plain
// integer and one an array whose members are reached through the part.
const writtenHolderModel = `package Demo {
    private import ScalarValues::*;
    private import Collections::*;
    attribute def LabeledGrid :> Array { attribute label : String; }
    attribute grid : LabeledGrid { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); :>> label = "grid"; }
    part def Holder {
        attribute n : Integer;
        attribute cells : LabeledGrid;
    }
    part holder : Holder;
    action Writer {
        action write { assign holder.n := 5; assign holder.cells := grid; }
        first start;
        then write;
        then done;
    }
}
`

// TestPipedSessionReadsWrittenValuesAfterADeclaration drives the prompt as the
// bug report did: an action writes the holder, an unrelated declaration
// re-analyzes the document, and %eval must then answer what %features lists.
func TestPipedSessionReadsWrittenValuesAfterADeclaration(t *testing.T) {
	binary := buildCLI(t)

	stdin := strings.Join([]string{
		"%instantiate Demo::holder",
		"%action Demo::Writer",
		"%continue",
		"part def Widget;",
		"%features Demo::holder",
		"%eval Demo::holder.n",
		"%eval Demo::holder.cells.rank",
		"%quit",
	}, "\n") + "\n"
	got := runPiped(t, binary, stdin, writtenHolderModel)
	if got.status != exitHolds {
		t.Errorf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"n = 5", "cells = Array(2, 2)[1, 2, 3, 4]", "✓ Demo::holder.n\n  = 5", "✓ Demo::holder.cells.rank\n  = 2"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("the session did not report %q:\n%s", want, got.output())
		}
	}
	if strings.Contains(got.stdout, "<unset>") {
		t.Errorf("%%eval answered a fresh object rather than the carried one:\n%s", got.output())
	}
}
