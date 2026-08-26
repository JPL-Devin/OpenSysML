package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// grpcExtension uses notation of ours: a warning by default, an error when the
// request asks strictly.
const grpcExtension = "package P { state def S { choice c; state a; } }"

func TestServerAdvertisesStrictConformance(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(info.Capabilities, CapabilityStrictConformance) {
		t.Fatalf("capabilities = %v, want %q among them", info.Capabilities, CapabilityStrictConformance)
	}
}

func TestParseFileStrictConformanceEscalatesOurNotation(t *testing.T) {
	srv := mustNewService(t, 10)
	for _, tc := range []struct {
		name   string
		strict bool
		want   string
	}{
		{"default", false, "warning"},
		{"strict", true, "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: grpcExtension},
				StrictConformance: tc.strict,
			})
			if err != nil {
				t.Fatal(err)
			}
			d := notationDiagnostic(t, resp)
			if d.Severity != tc.want {
				t.Fatalf("severity = %q, want %q", d.Severity, tc.want)
			}
		})
	}
}

// The two modes must not share a cache entry, or the second caller is answered
// with the first one's question.
func TestParseFileCachesTheModesSeparately(t *testing.T) {
	srv := mustNewService(t, 10)
	def, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: grpcExtension},
	})
	if err != nil {
		t.Fatal(err)
	}
	strict, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:            &pb.ParseFileRequest_Content{Content: grpcExtension},
		StrictConformance: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if def.ModelHash == strict.ModelHash {
		t.Fatalf("both modes hashed to %q", def.ModelHash)
	}
	if got := notationDiagnostic(t, def).Severity; got != "warning" {
		t.Errorf("default severity = %q, want warning", got)
	}
	if got := notationDiagnostic(t, strict).Severity; got != "error" {
		t.Errorf("strict severity = %q, want error", got)
	}

	// Re-asking the default question after the strict one must still answer it.
	again, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: grpcExtension},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := notationDiagnostic(t, again).Severity; got != "warning" {
		t.Errorf("default severity after the strict call = %q, want warning", got)
	}
}

// notationDiagnostic is the response's single nonstandard-notation finding.
func notationDiagnostic(t *testing.T, resp *pb.ParseFileResponse) *pb.Diagnostic {
	t.Helper()
	var found []*pb.Diagnostic
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "OpenSysML extension") {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d extension diagnostic(s) in %+v, want 1", len(found), resp.Diagnostics)
	}
	return found[0]
}
