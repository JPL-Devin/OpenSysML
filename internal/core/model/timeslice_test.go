package model

import (
	"os"
	"testing"
)

func TestTimeSliceExample(t *testing.T) {
	ws := NewWorkspace()
	
	data, err := os.ReadFile("../../../examples/sysml-v2-training/27. Occurrences/Time Slice and Snapshot Example.sysml")
	if err != nil {
		t.Fatal(err)
	}
	
	ws.Open("test.sysml", data, 1)
	diags := ws.Diagnostics("test.sysml")
	
	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %s", d.Message)
	}
}
