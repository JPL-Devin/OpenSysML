package opensysml

import (
	"context"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Client answers SysML v2 questions: parse, look up, evaluate, instantiate.
// New gives the default in-process implementation and Dial a remote one; both
// answer identically, which the conformance suite holds them to.
//
// The interface is sealed: implementations come from this package only, so a
// method can be added without breaking callers.
type Client interface {
	// ServerInfo reports the implementation's version and capabilities.
	ServerInfo(ctx context.Context) (*ServerInfo, error)

	// ParseFile parses the model file at path. Diagnostics, including syntax
	// errors, arrive on the returned Model; only an unreadable path or an
	// invalid request fails the call.
	ParseFile(ctx context.Context, path string, opts ...ParseOption) (*Model, error)

	// ParseSource parses inline model source, SysML unless WithLanguage says
	// otherwise. Diagnostics arrive on the returned Model.
	ParseSource(ctx context.Context, content string, opts ...ParseOption) (*Model, error)

	// Diagnostics reports every diagnostic of the parsed model, parser and
	// semantic passes combined — the same list the Model already carries.
	Diagnostics(ctx context.Context, model *Model) ([]Diagnostic, error)

	// LookupSymbol looks a symbol up by fully qualified name. An unknown name
	// is a FailureError; an unknown model is CodeNotFound.
	LookupSymbol(ctx context.Context, model *Model, symbolID string) (*Symbol, error)

	// Evaluate evaluates a SysML expression against the parsed model. An
	// expression that does not parse or evaluate is a FailureError carrying
	// the diagnostics; an unknown model is CodeNotFound.
	Evaluate(ctx context.Context, model *Model, expression string, opts ...EvaluateOption) (Value, error)

	// Instantiate instantiates the named part or usage, answering the root
	// instance and everything reachable from it. An unknown symbol or a failed
	// instantiation is a FailureError; an unknown model is CodeNotFound.
	Instantiate(ctx context.Context, model *Model, symbolID string) (*Instantiation, error)

	// Close releases what the implementation holds. The Client answers no
	// further calls.
	Close() error

	sealed()
}

// ParseOption configures ParseFile and ParseSource.
type ParseOption func(*parseOptions)

type parseOptions struct {
	language string
	strict   bool
}

// WithLanguage names the notation of inline content: LanguageSysML (the
// default) or LanguageKerML. File parsing infers the notation from the file
// extension instead.
func WithLanguage(language Language) ParseOption {
	return func(o *parseOptions) { o.language = string(language) }
}

// WithStrictConformance asks for the source to be judged as conforming SysML
// v2: notation no pinned production admits is an error rather than a warning.
// Requires the strict_conformance capability.
func WithStrictConformance() ParseOption {
	return func(o *parseOptions) { o.strict = true }
}

// EvaluateOption configures Evaluate.
type EvaluateOption func(*evaluateOptions)

type evaluateOptions struct {
	contextSymbolID string
	subjectSymbolID string
}

// WithContextSymbol names the symbol whose scope the expression's names are
// resolved in.
func WithContextSymbol(symbolID string) EvaluateOption {
	return func(o *evaluateOptions) { o.contextSymbolID = symbolID }
}

// WithSubject names a symbol to instantiate and evaluate against, so a feature
// reads that object's value rather than the declared default. Requires the
// evaluate_subject capability.
func WithSubject(symbolID string) EvaluateOption {
	return func(o *evaluateOptions) { o.subjectSymbolID = symbolID }
}

// caller is the transport behind a client: the in-process service or a remote
// one, speaking the same protobuf request and response types so both run the
// one service implementation.
type caller interface {
	serverInfo(ctx context.Context) (*pb.ServerInfoResponse, error)
	parseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error)
	getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error)
	getDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error)
	evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error)
	instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error)
	close() error
}

// client is the one Client implementation, over either caller.
type client struct {
	caller caller
}

func (c *client) sealed() {}

func (c *client) ServerInfo(ctx context.Context) (*ServerInfo, error) {
	resp, err := c.caller.serverInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &ServerInfo{
		Version:      resp.Version,
		Capabilities: append([]string(nil), resp.Capabilities...),
	}, nil
}

func (c *client) ParseFile(ctx context.Context, path string, opts ...ParseOption) (*Model, error) {
	req := parseRequest(opts)
	req.Source = &pb.ParseFileRequest_FilePath{FilePath: path}
	return c.parse(ctx, req)
}

func (c *client) ParseSource(ctx context.Context, content string, opts ...ParseOption) (*Model, error) {
	req := parseRequest(opts)
	req.Source = &pb.ParseFileRequest_Content{Content: content}
	return c.parse(ctx, req)
}

func parseRequest(opts []ParseOption) *pb.ParseFileRequest {
	var options parseOptions
	for _, opt := range opts {
		opt(&options)
	}
	return &pb.ParseFileRequest{
		Language:          options.language,
		StrictConformance: options.strict,
	}
}

func (c *client) parse(ctx context.Context, req *pb.ParseFileRequest) (*Model, error) {
	resp, err := c.caller.parseFile(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "ParseFile", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	return &Model{
		Hash:        resp.ModelHash,
		Root:        symbolFromProto(resp.Root),
		Diagnostics: diagnosticsFromProto(resp.Diagnostics),
	}, nil
}

func (c *client) Diagnostics(ctx context.Context, model *Model) ([]Diagnostic, error) {
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.getDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: hash})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "Diagnostics", Message: resp.Error}
	}
	return diagnosticsFromProto(resp.Diagnostics), nil
}

func (c *client) LookupSymbol(ctx context.Context, model *Model, symbolID string) (*Symbol, error) {
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.getSymbol(ctx, &pb.GetSymbolRequest{ModelHash: hash, SymbolId: symbolID})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "LookupSymbol", Message: resp.Error}
	}
	return symbolFromProto(resp.Symbol), nil
}

func (c *client) Evaluate(ctx context.Context, model *Model, expression string, opts ...EvaluateOption) (Value, error) {
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	var options evaluateOptions
	for _, opt := range opts {
		opt(&options)
	}
	resp, err := c.caller.evaluate(ctx, &pb.EvaluateRequest{
		ModelHash:       hash,
		Expression:      expression,
		ContextSymbolId: options.contextSymbolID,
		SubjectSymbolId: options.subjectSymbolID,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "Evaluate", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	return valueFromProto(resp.Result), nil
}

func (c *client) Instantiate(ctx context.Context, model *Model, symbolID string) (*Instantiation, error) {
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: symbolID})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "Instantiate", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	instances := make([]*Instance, 0, len(resp.Instances))
	for _, inst := range resp.Instances {
		instances = append(instances, instanceFromProto(inst))
	}
	return &Instantiation{
		Root:        instanceFromProto(resp.Instance),
		Instances:   instances,
		Diagnostics: diagnosticsFromProto(resp.Diagnostics),
	}, nil
}

func (c *client) Close() error {
	return c.caller.close()
}

// modelHash is the hash a call sends for a model, refusing a nil model the way
// the service refuses an unknown one.
func modelHash(model *Model) (string, error) {
	if model == nil {
		return "", &StatusError{Code: CodeInvalidArgument, Message: "model is nil"}
	}
	return model.Hash, nil
}
