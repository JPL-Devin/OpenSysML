package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

// baseClient stubs all 12 protocol.Client methods; only PublishDiagnostics is overridden by fakeClient.
type baseClient struct{}

func (baseClient) Progress(ctx context.Context, params *protocol.ProgressParams) error {
	return nil
}
func (baseClient) WorkDoneProgressCreate(ctx context.Context, params *protocol.WorkDoneProgressCreateParams) error {
	return nil
}
func (baseClient) LogMessage(ctx context.Context, params *protocol.LogMessageParams) error {
	return nil
}
func (baseClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	return nil
}
func (baseClient) ShowMessage(ctx context.Context, params *protocol.ShowMessageParams) error {
	return nil
}
func (baseClient) ShowMessageRequest(ctx context.Context, params *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, nil
}
func (baseClient) Telemetry(ctx context.Context, params interface{}) error {
	return nil
}
func (baseClient) RegisterCapability(ctx context.Context, params *protocol.RegistrationParams) error {
	return nil
}
func (baseClient) UnregisterCapability(ctx context.Context, params *protocol.UnregistrationParams) error {
	return nil
}
func (baseClient) ApplyEdit(ctx context.Context, params *protocol.ApplyWorkspaceEditParams) (bool, error) {
	return false, nil
}
func (baseClient) Configuration(ctx context.Context, params *protocol.ConfigurationParams) ([]interface{}, error) {
	return nil, nil
}
func (baseClient) WorkspaceFolders(ctx context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, nil
}

type fakeClient struct {
	baseClient
	published []*protocol.PublishDiagnosticsParams
}

func (c *fakeClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	c.published = append(c.published, params)
	return nil
}

func TestPublishDiagnosticsReportsSyntaxError(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	// Missing closing brace -> syntax diagnostic.
	name := "bad.sysml"
	ws.Open(name, []byte("package P {"), 1)
	s.publishDiagnostics(context.Background(), name)

	if len(fc.published) != 1 {
		t.Fatalf("published count = %d, want 1", len(fc.published))
	}
	got := fc.published[0]
	if got.URI != uri.File(name) {
		t.Errorf("URI = %q, want %q", got.URI, uri.File(name))
	}
	if len(got.Diagnostics) == 0 {
		t.Fatalf("expected at least one diagnostic")
	}
	// passes.SeverityError (0) -> LSP severity 1.
	if got.Diagnostics[0].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("severity = %v, want %v", got.Diagnostics[0].Severity, protocol.DiagnosticSeverityError)
	}
	// Source/Code are copied verbatim from the syntax pass.
	if got.Diagnostics[0].Source != "syntax" {
		t.Errorf("source = %q, want %q", got.Diagnostics[0].Source, "syntax")
	}
	if got.Diagnostics[0].Code != "syntax" {
		t.Errorf("code = %v, want %q", got.Diagnostics[0].Code, "syntax")
	}
}

func TestPublishDiagnosticsClearsWhenClean(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	// A valid document produces no diagnostics but must still publish an empty
	// list so the editor clears any stale squiggles (LSP push-clear semantics).
	ws.Open("ok.sysml", []byte("package P;"), 1)
	s.publishDiagnostics(context.Background(), "ok.sysml")

	if len(fc.published) != 1 {
		t.Fatalf("published count = %d, want 1", len(fc.published))
	}
	if len(fc.published[0].Diagnostics) != 0 {
		t.Fatalf("diagnostics = %d, want 0 on clean doc", len(fc.published[0].Diagnostics))
	}
}

func TestPublishDiagnosticsNilClientNoPanic(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open("ok.sysml", []byte("package P;"), 1)
	// s.client is nil; must not panic.
	s.publishDiagnostics(context.Background(), "ok.sysml")
}
