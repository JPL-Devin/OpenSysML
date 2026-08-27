package opensysml

import (
	"context"
	"fmt"
	"runtime/debug"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
	"google.golang.org/grpc/status"
)

// defaultCacheSize matches the sysml-grpc default, so the two implementations
// keep the same number of parses warm.
const defaultCacheSize = 100

// Option configures New.
type Option func(*options)

type options struct {
	cacheSize int
}

// WithCacheSize bounds how many parsed models the client keeps, evicting the
// least recently used beyond it. The default is 100; New refuses a size that
// is not positive.
func WithCacheSize(size int) Option {
	return func(o *options) { o.cacheSize = size }
}

// New is the in-process implementation: the engine linked into this binary,
// behind the same interface and semantics as the service — same cache keying,
// same capability list, same in-band failures — with no port, no child process
// and no serialization. Runtime budgets and library prewarming are read from
// the environment, as the service reads them. Close releases the standard
// library index the client holds.
func New(opts ...Option) (Client, error) {
	config := options{cacheSize: defaultCacheSize}
	for _, opt := range opts {
		opt(&config)
	}
	svc, err := sysmlgrpc.NewService(config.cacheSize, buildVersion())
	if err != nil {
		return nil, err
	}
	svc.Prewarm()
	return &client{caller: &inprocess{svc: svc}}, nil
}

// buildVersion is the module version of this binary, informational only.
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

// inprocess calls the service implementation directly. Errors cross as
// StatusError, and a panic is caught at this boundary rather than crossing it.
type inprocess struct {
	svc *sysmlgrpc.Service
}

func (p *inprocess) serverInfo(ctx context.Context) (resp *pb.ServerInfoResponse, err error) {
	defer recoverToError(&err)
	resp, callErr := p.svc.GetServerInfo(ctx, &pb.ServerInfoRequest{})
	return resp, statusToError(callErr)
}

func (p *inprocess) parseFile(ctx context.Context, req *pb.ParseFileRequest) (resp *pb.ParseFileResponse, err error) {
	defer recoverToError(&err)
	resp, callErr := p.svc.ParseFile(ctx, req)
	return resp, statusToError(callErr)
}

func (p *inprocess) getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (resp *pb.SymbolResponse, err error) {
	defer recoverToError(&err)
	resp, callErr := p.svc.GetSymbol(ctx, req)
	return resp, statusToError(callErr)
}

func (p *inprocess) getDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (resp *pb.DiagnosticsResponse, err error) {
	defer recoverToError(&err)
	resp, callErr := p.svc.GetDiagnostics(ctx, req)
	return resp, statusToError(callErr)
}

func (p *inprocess) evaluate(ctx context.Context, req *pb.EvaluateRequest) (resp *pb.EvaluateResponse, err error) {
	defer recoverToError(&err)
	resp, callErr := p.svc.Evaluate(ctx, req)
	return resp, statusToError(callErr)
}

func (p *inprocess) instantiate(ctx context.Context, req *pb.InstantiateRequest) (resp *pb.InstantiateResponse, err error) {
	defer recoverToError(&err)
	resp, callErr := p.svc.Instantiate(ctx, req)
	return resp, statusToError(callErr)
}

func (p *inprocess) close() error {
	p.svc.Close()
	return nil
}

// statusToError is the documented status mapping: the gRPC status a handler
// refuses with becomes a StatusError carrying the same code.
func statusToError(err error) error {
	if err == nil {
		return nil
	}
	st := status.Convert(err)
	return &StatusError{Code: Code(st.Code()), Message: st.Message()}
}

// recoverToError keeps a panic from crossing the public boundary: it becomes
// CodeInternal, the status the wire would answer for a crashed handler.
func recoverToError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = &StatusError{Code: CodeInternal, Message: fmt.Sprintf("panic: %v", recovered)}
	}
}
