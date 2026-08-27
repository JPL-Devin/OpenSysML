package main

import (
	"slices"
	"testing"
)

func TestUnavailableCapabilitiesForTesting(t *testing.T) {
	t.Setenv(testWithholdCapabilitiesEnv, " query, strict_conformance ,,")
	got := unavailableCapabilitiesForTesting()
	want := []string{"query", "strict_conformance"}
	if !slices.Equal(got, want) {
		t.Errorf("capabilities = %v, want %v", got, want)
	}
}
