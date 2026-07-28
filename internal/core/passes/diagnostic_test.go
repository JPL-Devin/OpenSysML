package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   "error",
		SeverityWarning: "warning",
		SeverityInfo:    "info",
		SeverityHint:    "hint",
		Severity(999):   "unknown",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(sev), got, want)
		}
	}
}

func TestDiagnosticFields(t *testing.T) {
	d := Diagnostic{
		Severity: SeverityError,
		Span:     source.Span{Offset: 3, Len: 5},
		Message:  "boom",
		Code:     "unresolved",
		Source:   "name-resolution",
	}
	if d.Severity != SeverityError || d.Span.Offset != 3 || d.Span.Len != 5 {
		t.Fatalf("unexpected diag: %+v", d)
	}
	if d.Message != "boom" || d.Code != "unresolved" || d.Source != "name-resolution" {
		t.Fatalf("unexpected diag: %+v", d)
	}
}
