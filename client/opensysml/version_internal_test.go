package opensysml

import "testing"

func TestOnlyAStampedVersionIsReported(t *testing.T) {
	for version, want := range map[string]string{
		"v0.4.1":   "v0.4.1",
		"v1.0.0":   "v1.0.0",
		"":         "dev",
		"(devel)":  "dev",
		"v0.0.0-0": "v0.0.0-0",
	} {
		if got := released(version); got != want {
			t.Errorf("released(%q) = %q, want %q", version, got, want)
		}
	}
}
