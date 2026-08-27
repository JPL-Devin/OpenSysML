package grpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CapabilityTypeFacts names the capability of populating SymbolInfo.type_info,
// .multiplicity and .specializations, which typed code generation requires.
const CapabilityTypeFacts = "type_facts"

// CapabilityConvert names the capability of the Convert RPC, which writes a
// model back out as SysML notation or RDF Turtle. The RDF direction is
// experimental, which the response says per conversion.
const CapabilityConvert = "convert"

// CapabilityVerification names the capability of the verification RPCs, which
// answer the questions the REPL's %constraint, %requirement, %satisfy and %calc
// answer.
const CapabilityVerification = "verification"

// CapabilityQuery names the capability of the Query RPC, which evaluates a
// SysML v2 API & Services Query over a parsed model.
const CapabilityQuery = "query"

// CapabilityOSLCQuery names the capability of evaluating OSLC Query text.
const CapabilityOSLCQuery = "oslc_query"

// CapabilityEnumValues names the capability of carrying an enumeration literal
// as Value.enum_literal, rather than reporting it as an unsupported null.
const CapabilityEnumValues = "enum_values"

// CapabilityEvaluateSubject names the capability of evaluating an expression
// against an instantiated subject.
const CapabilityEvaluateSubject = "evaluate_subject"

// CapabilitySymbolAttributes names the capability of populating
// SymbolInfo.attributes, rather than always reporting none.
const CapabilitySymbolAttributes = "symbol_attributes"

// CapabilityUnsetValue names the capability of reporting a valueless feature of
// a value type as Value.unset, rather than as the empty object it materializes.
const CapabilityUnsetValue = "unset_value"

// CapabilityApplyEdits names the capability of the ApplyEdits RPC, which edits
// a parsed model's own source, preserving everything the edit did not touch.
const CapabilityApplyEdits = "apply_edits"

// CapabilityAuthoring names add-member and delete source authoring operations.
const CapabilityAuthoring = "authoring"

// CapabilityInlineLanguage names explicit language selection for inline content.
const CapabilityInlineLanguage = "inline_language"

// CapabilityStrictConformance names ParseFileRequest.strict_conformance, which
// asks whether the source is conforming SysML v2.
const CapabilityStrictConformance = "strict_conformance"

// CapabilityFeatureValues names the capability of populating
// Instance.feature_values, which replaced the pre-0.1.0 Instance.slots.
const CapabilityFeatureValues = "feature_values"

// capabilities is what this build supports, in report order. A capability is
// only ever added: renaming or dropping one breaks clients that require it.
var capabilities = []string{
	CapabilityTypeFacts, CapabilityConvert, CapabilityVerification, CapabilityQuery,
	CapabilityOSLCQuery,
	CapabilityEnumValues, CapabilityEvaluateSubject, CapabilitySymbolAttributes,
	CapabilityUnsetValue, CapabilityFeatureValues, CapabilityApplyEdits,
	CapabilityAuthoring, CapabilityInlineLanguage, CapabilityStrictConformance,
}

type capabilityAvailability struct {
	available map[string]struct{}
}

func newCapabilityAvailability(withheld []string) (capabilityAvailability, error) {
	available := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		available[capability] = struct{}{}
	}
	for _, capability := range withheld {
		if _, ok := available[capability]; !ok {
			return capabilityAvailability{}, fmt.Errorf("unknown capability %q", capability)
		}
		delete(available, capability)
	}
	return capabilityAvailability{available: available}, nil
}

func (a capabilityAvailability) has(capability string) bool {
	_, ok := a.available[capability]
	return ok
}

func (a capabilityAvailability) names() []string {
	names := make([]string, 0, len(a.available))
	for _, capability := range capabilities {
		if a.has(capability) {
			names = append(names, capability)
		}
	}
	return names
}

// Capabilities returns the capability names this build of the service reports.
func Capabilities() []string {
	return append([]string(nil), capabilities...)
}

// Service implements SysMLServiceServer
type Service struct {
	pb.UnimplementedSysMLServiceServer
	cache *Cache
	// libIndexes hands each model an overlay over the one standard library index
	// the service holds: the library does not depend on the model, so no model
	// pays for loading it, and no model keeps a copy of it.
	libIndexes *libraryBase
	// prewarm is whether Prewarm builds the library ahead of the first request.
	prewarm bool
	// budgets bounds every runtime context the service creates, read once from
	// the environment at construction.
	budgets runtime.Budgets
	// version is the build version GetServerInfo reports, informational only.
	version string
	// capabilities decides both what this service reports and what it supplies.
	capabilities capabilityAvailability
}

// NewService creates a gRPC service with specified cache size, reporting
// version as its build version. It returns an error if cacheSize is not
// positive, if a budget variable holds anything but a positive integer, or if
// the prewarm setting is not a non-negative integer. It does not load the
// standard library: call Prewarm to have that happen in the background, ahead of
// the requests that need it.
func NewService(cacheSize int, version string) (*Service, error) {
	return newService(cacheSize, version, nil)
}

// NewServiceWithUnavailableCapabilitiesForTesting creates a service that
// deliberately lacks named capabilities for conformance testing.
func NewServiceWithUnavailableCapabilitiesForTesting(cacheSize int, version string, unavailable []string) (*Service, error) {
	return newService(cacheSize, version, unavailable)
}

func newService(cacheSize int, version string, unavailable []string) (*Service, error) {
	cache, err := NewCache(cacheSize)
	if err != nil {
		return nil, err
	}
	availability, err := newCapabilityAvailability(unavailable)
	if err != nil {
		return nil, err
	}
	budgets, err := runtime.BudgetsFromEnv()
	if err != nil {
		return nil, err
	}
	prewarm, err := indexPrewarmFromEnv()
	if err != nil {
		return nil, err
	}
	return &Service{
		cache:        cache,
		libIndexes:   newLibraryBase(buildLibraryIndex),
		prewarm:      prewarm > 0,
		budgets:      budgets,
		version:      version,
		capabilities: availability,
	}, nil
}

// Prewarm starts building the service's standard library index in the
// background, so the first model to arrive is not the one that pays for the
// library. It returns immediately and is safe to call more than once.
func (s *Service) Prewarm() {
	if !s.prewarm {
		return
	}
	s.libIndexes.prewarm()
}

// Close waits for a background library build in flight and releases the
// service's reference to the shared index. The service still answers
// afterwards, building the library on the request that needs it.
func (s *Service) Close() {
	s.libIndexes.close()
}

// GetServerInfo reports the service's build version and capabilities. A service
// too old to have this RPC fails the call with UNIMPLEMENTED, which is itself
// the answer that it predates every capability.
func (s *Service) GetServerInfo(ctx context.Context, req *pb.ServerInfoRequest) (*pb.ServerInfoResponse, error) {
	return &pb.ServerInfoResponse{
		Version:      s.version,
		Capabilities: s.capabilities.names(),
	}, nil
}

func (s *Service) requireCapability(capability string) error {
	if s.capabilities.has(capability) {
		return nil
	}
	return statusErrorf(connect.CodeUnimplemented, "capability %q is unavailable", capability)
}

// newRuntime returns a runtime context under the service's budgets.
func (s *Service) newRuntime(semModel *semantics.Model, resolver *resolve.Resolver) *runtime.Context {
	ctx := runtime.NewContext(semModel, resolver, s.budgets.MaxSteps)
	if err := ctx.SetBudgets(s.budgets); err != nil {
		// Unreachable: NewService validated these budgets.
		panic(fmt.Sprintf("grpc: invalid service budgets: %v", err))
	}
	return ctx
}

// ParseFile parses a SysML file and caches the result
func (s *Service) ParseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	if req.StrictConformance {
		if err := s.requireCapability(CapabilityStrictConformance); err != nil {
			return nil, err
		}
	}
	if _, inline := req.Source.(*pb.ParseFileRequest_Content); inline && req.Language != "" {
		if err := s.requireCapability(CapabilityInlineLanguage); err != nil {
			return nil, err
		}
	}

	// Extract source content and file path
	var content string
	var filePath string
	var kind source.Kind

	switch src := req.Source.(type) {
	case *pb.ParseFileRequest_Content:
		content = src.Content
		filePath = "<content>"
		kind = source.KindSysML
		switch req.Language {
		case "", "sysml":
		case "kerml":
			kind = source.KindKerML
		default:
			return nil, statusErrorf(connect.CodeInvalidArgument,
				"language must be sysml or kerml, got %q", req.Language)
		}
	case *pb.ParseFileRequest_FilePath:
		filePath = src.FilePath
		kind = source.KindOf(filePath)
		// #nosec G304 -- the client names the model file it wants parsed; reading
		// arbitrary paths is the service's purpose, and it runs with the caller's
		// own privileges.
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, statusErrorf(connect.CodeNotFound, "file not found: %v", err)
		}
		content = string(data)
	default:
		return nil, statusError(connect.CodeInvalidArgument, "source must be file_path or content")
	}

	// Keyed by what was read, not by the hash the request carried: a hash
	// disagreeing with its content would serve another model. The file name is
	// part of the key, since a record's diagnostics name the file it came from.
	// The conformance mode is part of the key: the same source asks a different
	// question in each mode, so one mode's diagnostics may not serve the other.
	mode := conformance.ModeOf(req.StrictConformance)
	modelHash := computeHash(filePath + "\x00" + req.Language + "\x00" + mode.String() + "\x00" + content)
	if cached, ok := s.cache.Get(modelHash); ok {
		return s.buildParseResponse(modelHash, cached), nil
	}

	// Parse the file
	srcFile := source.NewWithKind(filePath, []byte(content), kind)
	p := parser.New(srcFile)
	root := p.ParseFile()

	// Get parser diagnostics
	parseDiags := p.Diagnostics

	// Take an index carrying the standard library, which type resolution needs:
	// an overlay over the one library index the service holds. The model's own
	// document goes into the overlay alone, so what it resolves against does not
	// depend on prewarming or on any other model.
	idx := s.libIndexes.get()

	// Add user document
	idx.AddDocumentWithKind(filePath, root, kind)

	// Run semantic passes (name-resolution, type, constraint)
	// Only run if no parse errors (tier gating per AGENTS.md §4)
	var passesDiags []passes.Diagnostic
	if len(parseDiags) == 0 {
		// passes.Analyze expects parser diagnostics converted to passes.Diagnostic
		parseDiagsConverted := make([]passes.Diagnostic, 0) // No parse errors to convert
		passesDiags = passes.AnalyzeWithOptions(filePath, kind, root, parseDiagsConverted, idx,
			passes.Options{Conformance: mode})
	}

	// Create cached model
	model := &CachedModel{
		Root:        root,
		Index:       idx,
		Source:      srcFile,
		ParseDiags:  parseDiags,
		PassesDiags: passesDiags,
	}

	// Cache the model
	s.cache.Put(modelHash, model)

	return s.buildParseResponse(modelHash, model), nil
}

// GetSymbol retrieves symbol information by FQN
func (s *Service) GetSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup symbol by FQN
	syms := lookupNamed(cached.Index, req.SymbolId)
	if len(syms) == 0 {
		return &pb.SymbolResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}

	// Convert first match to proto
	return &pb.SymbolResponse{
		Symbol: s.symbolToProto(syms[0], cached.SymbolContext()),
	}, nil
}

// GetDiagnostics retrieves all diagnostics for a parsed model (parser + semantic passes)
func (s *Service) GetDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}

	// Convert parser diagnostics to proto
	var pbDiags []*pb.Diagnostic
	for _, diag := range cached.ParseDiags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, cached.Source))
	}

	// Convert semantic pass diagnostics to proto
	for _, diag := range cached.PassesDiags {
		pbDiags = append(pbDiags, DiagnosticToProto(diag, cached.Source))
	}

	return &pb.DiagnosticsResponse{
		Diagnostics: pbDiags,
	}, nil
}

// Evaluate evaluates a SysML expression in the context of a parsed model
func (s *Service) Evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error) {
	if req.SubjectSymbolId != "" {
		if err := s.requireCapability(CapabilityEvaluateSubject); err != nil {
			return nil, err
		}
	}

	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}

	// Parse expression
	exprSource := source.New("<expression>", []byte(req.Expression))
	p := parser.New(exprSource)
	exprNode := p.ParseExpression()

	// Check for parse errors
	if len(p.Diagnostics) > 0 {
		var pbDiags []*pb.Diagnostic
		for _, diag := range p.Diagnostics {
			pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, exprSource))
		}
		return &pb.EvaluateResponse{
			Diagnostics: pbDiags,
			Error:       "expression parse failed",
		}, nil
	}

	// A subject is both what the expression is evaluated against and, unless a
	// context is named, the namespace its features are named in — the way the
	// prompt evaluates in the context it pinned.
	var subject *symbols.Symbol
	if req.SubjectSymbolId != "" {
		syms := lookupNamed(cached.Index, req.SubjectSymbolId)
		if len(syms) == 0 {
			return &pb.EvaluateResponse{
				Error: fmt.Sprintf("subject not found: %s", req.SubjectSymbolId),
			}, nil
		}
		subject = syms[0]
	}

	// Determine scope
	var scope *symbols.Scope
	if req.ContextSymbolId != "" {
		// Lookup context symbol
		syms := lookupNamed(cached.Index, req.ContextSymbolId)
		if len(syms) > 0 && syms[0].Scope != nil {
			scope = syms[0].Scope
		}
	}
	if scope == nil && subject != nil {
		scope = evalScope(subject, cached)
	}
	if scope == nil {
		// Use document root as default scope
		scope = cached.Index.DocumentRoot(cached.Source.Name())
	}

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := s.newRuntime(semModel, resolver)

	var self *runtime.Instance
	if subject != nil {
		inst, err := runtimeCtx.Instantiate(subject)
		if err != nil {
			return &pb.EvaluateResponse{
				Error: fmt.Sprintf("instantiation of subject %s failed: %v", req.SubjectSymbolId, err),
			}, nil
		}
		self = inst
	}

	// Create eval context and evaluate
	evalCtx := runtime.NewEvalContextIn(runtimeCtx, scope, self)
	result, err := evalCtx.Eval(exprNode)
	if err != nil {
		return &pb.EvaluateResponse{
			Error: fmt.Sprintf("evaluation failed: %v", err),
		}, nil
	}

	return &pb.EvaluateResponse{
		Result: s.valueToProto(runtimeCtx, result, cached.Index),
	}, nil
}

// evalScope is the namespace a subject's features are named in: its own scope,
// so a member is named unqualified, else the scope it was declared in. Mirrors
// the REPL's contextScope.
func evalScope(sym *symbols.Symbol, cached *CachedModel) *symbols.Scope {
	switch {
	case sym == nil:
		return nil
	case sym.Scope != nil:
		return sym.Scope
	case sym.OwnerScope != nil:
		return sym.OwnerScope
	default:
		return cached.Index.DocumentRoot(cached.Source.Name())
	}
}

// Instantiate creates a runtime instance of a part/usage
func (s *Service) Instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup symbol
	syms := lookupNamed(cached.Index, req.SymbolId)
	if len(syms) == 0 {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}
	sym := syms[0]

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := s.newRuntime(semModel, resolver)

	// Instantiate
	inst, err := runtimeCtx.Instantiate(sym)
	if err != nil {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("instantiation failed: %v", err),
		}, nil
	}

	root, all := s.instanceGraphToProto(runtimeCtx, inst, cached.Index)
	return &pb.InstantiateResponse{
		Instance:  root,
		Instances: all,
	}, nil
}

// ExecuteAction executes an action definition
func (s *Service) ExecuteAction(ctx context.Context, req *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup action symbol
	syms := lookupNamed(cached.Index, req.ActionSymbolId)
	if len(syms) == 0 {
		return &pb.ExecuteActionResponse{
			Error: fmt.Sprintf("action not found: %s", req.ActionSymbolId),
		}, nil
	}
	action := syms[0]

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := s.newRuntime(semModel, resolver)

	// Converted against the model's index, so a quantity input keeps the base
	// units it is commensurable with instead of binding an unusable value.
	var inputs map[string]runtime.Value
	if len(req.Inputs) > 0 {
		inputs = make(map[string]runtime.Value, len(req.Inputs))
		for name, pv := range req.Inputs {
			val, cerr := ProtoToValueIn(pv, cached.Index, semModel)
			if cerr != nil {
				return &pb.ExecuteActionResponse{
					Error: fmt.Sprintf("input %q could not be read: %v", name, cerr),
				}, nil
			}
			inputs[name] = val
		}
	}

	// Execute action with the supplied inputs
	outputs, err := runtimeCtx.ExecuteActionWithInputs(action, inputs)
	if err != nil {
		return &pb.ExecuteActionResponse{
			Error: fmt.Sprintf("action execution failed: %v", err),
		}, nil
	}

	// Convert outputs to protobuf
	pbOutputs := make(map[string]*pb.Value)
	for name, val := range outputs {
		pbOutputs[name] = s.valueToProto(runtimeCtx, val, cached.Index)
	}

	return &pb.ExecuteActionResponse{
		Outputs: pbOutputs,
	}, nil
}

// ExecuteState executes a state machine
func (s *Service) ExecuteState(ctx context.Context, req *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup state machine symbol
	syms := lookupNamed(cached.Index, req.StateMachineSymbolId)
	if len(syms) == 0 {
		return &pb.ExecuteStateResponse{
			Error: fmt.Sprintf("state machine not found: %s", req.StateMachineSymbolId),
		}, nil
	}
	stateMachine := syms[0]

	// Create runtime context
	resolver := resolve.New(cached.Index)
	semModel := semantics.NewModel(resolver)
	runtimeCtx := s.newRuntime(semModel, resolver)

	// Execute state machine, injecting the requested events and capturing the
	// real ordered state-visit trace.
	finalContext, statesVisited, err := runtimeCtx.ExecuteStateWithEvents(stateMachine, req.Events)
	if err != nil {
		return &pb.ExecuteStateResponse{
			Error: fmt.Sprintf("state machine execution failed: %v", err),
		}, nil
	}

	// Convert final context to protobuf
	pbContext := make(map[string]*pb.Value)
	for name, val := range finalContext {
		pbContext[name] = s.valueToProto(runtimeCtx, val, cached.Index)
	}

	return &pb.ExecuteStateResponse{
		StatesVisited: statesVisited,
		FinalContext:  pbContext,
	}, nil
}

// buildParseResponse constructs ParseFileResponse from cached model
func (s *Service) buildParseResponse(modelHash string, model *CachedModel) *pb.ParseFileResponse {
	// Convert parser + semantic diagnostics
	var pbDiags []*pb.Diagnostic
	for _, diag := range model.ParseDiags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, model.Source))
	}
	for _, diag := range model.PassesDiags {
		pbDiags = append(pbDiags, DiagnosticToProto(diag, model.Source))
	}

	// Convert root namespace to SymbolInfo
	var rootSymbol *pb.SymbolInfo
	if model.Root != nil {
		// Find root symbol in index
		rootSyms := model.Index.LookupQualified("") // Root has empty name
		if len(rootSyms) == 0 {
			// Fallback: use DocumentRoot
			rootScope := model.Index.DocumentRoot(model.Source.Name())
			if rootScope != nil {
				rootSymbol = &pb.SymbolInfo{
					Id:       "",
					Name:     "",
					Kind:     "RootNamespace",
					Metadata: make(map[string]string),
					ChildIds: collectChildIDs(rootScope, model.Index),
				}
			}
		} else {
			rootSymbol = s.symbolToProto(rootSyms[0], model.SymbolContext())
		}
	}

	return &pb.ParseFileResponse{
		ModelHash:   modelHash,
		Root:        rootSymbol,
		Diagnostics: pbDiags,
	}
}

// collectChildIDs extracts child symbol FQNs from a scope
func collectChildIDs(scope *symbols.Scope, idx *symbols.Index) []string {
	var ids []string
	for _, sym := range scope.AllMembers() {
		ids = append(ids, idx.GetFQN(sym))
	}
	return ids
}

// computeHash generates SHA-256 hash of content
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// lookupNamed resolves a symbol ID written in either spelling: the quoted,
// notation-legal form a model author writes ('My Pkg'::Car), or the unquoted
// spelling the index records (My Pkg::Car), which keeps working as it did.
// Model elements come before library homonyms: an ID naming both denotes the
// model's own element, which is what the client asked about.
func lookupNamed(idx *symbols.Index, id string) []*symbols.Symbol {
	if syms := idx.LookupQualified(id); len(syms) > 0 {
		return modelFirst(idx, syms)
	}
	if plain, ok := unquotedName(id); ok && plain != id {
		return modelFirst(idx, idx.LookupQualified(plain))
	}
	return nil
}

// modelFirst reorders matches so the ones the model declares precede the ones
// standard-library content declares, each group keeping its index order.
func modelFirst(idx *symbols.Index, syms []*symbols.Symbol) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(syms))
	var lib []*symbols.Symbol
	for _, sym := range syms {
		if idx.Library(sym) {
			lib = append(lib, sym)
			continue
		}
		out = append(out, sym)
	}
	return append(out, lib...)
}

// unquotedName is the name a notation-legal qualified name states, with the
// quoting of its unrestricted segments removed, or false for an ID the notation
// does not read as one whole name.
func unquotedName(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	p := parser.New(source.New("<symbol-id>", []byte(id)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 || p.Offset() != len(id) {
		return "", false
	}
	ref, ok := expr.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ref.Name.Global || len(ref.Name.Parts) == 0 {
		return "", false
	}
	segments := make([]string, 0, len(ref.Name.Parts))
	for _, part := range ref.Name.Parts {
		if part.Text == "" {
			return "", false
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::"), true
}
