package opensysml

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
)

// DialOption configures Dial.
type DialOption func(*dialOptions)

type dialOptions struct {
	httpClient *http.Client
	json       bool
}

// WithHTTPClient names the HTTP client the connection uses, for callers with
// their own transport, TLS or timeout policy. The default is
// http.DefaultClient.
func WithHTTPClient(httpClient *http.Client) DialOption {
	return func(o *dialOptions) { o.httpClient = httpClient }
}

// WithJSONBody sends requests as Connect JSON instead of protobuf. JSON is a
// debugging affordance — legible in curl and proxies — and costs an order of
// magnitude in encoding time on large responses, which is why protobuf is the
// default.
func WithJSONBody() DialOption {
	return func(o *dialOptions) { o.json = true }
}

// Dial is the remote implementation: the same interface as New, answered by a
// service someone else runs. The address is explicit — this package never
// starts a service — and names a sysml-grpc serving the Connect transport
// ("localhost:50051" or "https://sysml.example.com"); an address without a
// scheme is dialed as http. Requests cross as Connect protobuf unless
// WithJSONBody says otherwise. Closing leaves the service running: it was not
// this package's to stop.
func Dial(address string, opts ...DialOption) (Client, error) {
	if strings.TrimSpace(address) == "" {
		return nil, errors.New("opensysml: Dial needs an address")
	}
	var config dialOptions
	for _, opt := range opts {
		opt(&config)
	}
	if config.httpClient == nil {
		config.httpClient = http.DefaultClient
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	var connectOpts []connect.ClientOption
	if config.json {
		connectOpts = append(connectOpts, connect.WithProtoJSON())
	}
	return &client{caller: &remote{
		rpc: protoconnect.NewSysMLServiceClient(config.httpClient, address, connectOpts...),
	}}, nil
}

// remote answers through a Connect client. Errors cross as StatusError with
// the equivalent code, so a refusal reads the same as in process.
type remote struct {
	rpc protoconnect.SysMLServiceClient
}

func (r *remote) serverInfo(ctx context.Context) (*pb.ServerInfoResponse, error) {
	resp, err := r.rpc.GetServerInfo(ctx, connect.NewRequest(&pb.ServerInfoRequest{}))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) parseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	resp, err := r.rpc.ParseFile(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) parseSources(ctx context.Context, req *pb.ParseSourcesRequest) (*pb.ParseSourcesResponse, error) {
	resp, err := r.rpc.ParseSources(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	resp, err := r.rpc.GetSymbol(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) getDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	resp, err := r.rpc.GetDiagnostics(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error) {
	resp, err := r.rpc.Evaluate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error) {
	resp, err := r.rpc.Instantiate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) close() error {
	return nil
}

// connectToError is the documented status mapping for the remote path: a
// Connect error code numbers the same as its gRPC status. A transport failure
// that never reached the service is CodeUnavailable.
func connectToError(err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return &StatusError{Code: Code(connectErr.Code()), Message: connectErr.Message()}
	}
	return &StatusError{Code: CodeUnavailable, Message: err.Error()}
}
