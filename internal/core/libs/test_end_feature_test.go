package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Test parsing of "end [name] [mult] feature ..." patterns
func TestEndFeaturePatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errors int
	}{
		{
			name: "simple end feature (anonymous end, defines feature)",
			input: `
				connector {
					end feature source: Anything;
				}
			`,
			errors: 0,
		},
		{
			name: "end name feature (named end, defines feature)",
			input: `
				connector {
					end self2 [1] feature sameThing: Anything;
				}
			`,
			errors: 0, // target after fix
		},
		{
			name: "end with mult feature (anonymous end with mult, defines feature)",
			input: `
				connector {
					end [1] feature transferSource references source;
				}
			`,
			errors: 0, // target after fix
		},
		{
			name: "from variant",
			input: `
				connector :Link
					from [1] shorterOccurrence references thisOccurrence
					to [1] longerOccurrence references thatOccurrence;
			`,
			errors: 0, // already works - no "feature" keyword
		},
		{
			name: "from with feature keyword",
			input: `
				connector {
					from [1] feature transferSource references source;
				}
			`,
			errors: 0, // target after fix (if pattern exists)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) != tt.errors {
				t.Errorf("Expected %d errors, got %d", tt.errors, len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}
		})
	}
}

// Test actual stdlib patterns that fail
func TestEndFeatureStdlib(t *testing.T) {
	tests := []struct {
		name   string
		input  string
	}{
		{
			name: "Links.kerml pattern",
			input: `
				classifier Link {
					end feature thisThing: Anything redefines source;
					end self2 [1] feature sameThing: Anything redefines target;
				}
			`,
		},
		{
			name: "Transfers.kerml pattern",
			input: `
				flow {
					end [1] feature transferSource references source;
					end [payloadNum] feature transferPayload references payload;
				}
			`,
		},
		{
			name: "Occurrences.kerml pattern",
			input: `
				connector :Within {
					from [0..*] smallerOccurrence references elements
					to [1] largerOccurrence references self;
				}
			`,
		},
		{
			name: "CausationConnections pattern",
			input: `
				connector {
					end theCauses [*] occurrence theCause :> causes {
						doc /* comment */
					}
					end theEffects [*] occurrence theEffect :> effects;
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			t.Logf("Parsed with %d diagnostics", len(p.Diagnostics))
			for _, d := range p.Diagnostics {
				t.Logf("  - %s (offset %d)", d.Message, d.Span.Offset)
			}
		})
	}
}
