package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"testing"
)

func TestTask68Simple(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantClean bool
	}{
		{
			name:      "succession_mult_name_first",
			input:     `package Test { succession [1] mySuccession first [1] a then [1] b; }`,
			wantClean: true,
		},
		{
			name:      "succession_mult_first_anonymous",
			input:     `package Test { succession [1] first [1] a then [1] b; }`,
			wantClean: true,
		},
		{
			name:      "connector_mult_name",
			input:     `package Test { connector [1] myConn from [1] a to [1] b; }`,
			wantClean: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.input)))
			_ = p.ParseFile()

			clean := len(p.Diagnostics) == 0
			if clean != tt.wantClean {
				t.Errorf("wantClean=%v, got=%v", tt.wantClean, clean)
				for _, d := range p.Diagnostics {
					t.Logf("  offset %d: %s", d.Span.Offset, d.Message)
				}
			}
		})
	}
}
