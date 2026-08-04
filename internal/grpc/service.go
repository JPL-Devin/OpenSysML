package grpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service implements SysMLServiceServer
type Service struct {
	pb.UnimplementedSysMLServiceServer
	cache *Cache
}

// NewService creates a gRPC service with specified cache size
func NewService(cacheSize int) *Service {
	return &Service{
		cache: NewCache(cacheSize),
	}
}

// ParseFile parses a SysML file and caches the result
func (s *Service) ParseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	// Extract source content and file path
	var content string
	var filePath string

	switch src := req.Source.(type) {
	case *pb.ParseFileRequest_Content:
		content = src.Content
		filePath = "<content>"
	case *pb.ParseFileRequest_FilePath:
		filePath = src.FilePath
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "file not found: %v", err)
		}
		content = string(data)
	default:
		return nil, status.Error(codes.InvalidArgument, "source must be file_path or content")
	}

	// Check cache using content hash
	if req.ContentHash != "" {
		if cached, ok := s.cache.Get(req.ContentHash); ok {
			// Cache hit - return cached model
			return s.buildParseResponse(req.ContentHash, cached), nil
		}
	}

	// Parse the file
	srcFile := source.New(filePath, []byte(content))
	p := parser.New(srcFile)
	root := p.ParseFile()

	// Get parser diagnostics
	diags := p.Diagnostics

	// Build symbol index for lookups
	idx := symbols.NewIndex()
	idx.AddDocument(filePath, root)

	// Create cached model
	model := &CachedModel{
		Root:   root,
		Index:  idx,
		Source: srcFile,
		Diags:  diags,
	}

	// Compute model hash
	modelHash := req.ContentHash
	if modelHash == "" {
		modelHash = computeHash(content)
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
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Lookup symbol by FQN
	syms := cached.Index.LookupQualified(req.SymbolId)
	if len(syms) == 0 {
		return &pb.SymbolResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}

	// Convert first match to proto
	return &pb.SymbolResponse{
		Symbol: SymbolToProto(syms[0], cached.Index),
	}, nil
}

// GetDiagnostics retrieves all diagnostics for a parsed model
func (s *Service) GetDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "model not found: %s", req.ModelHash)
	}

	// Convert diagnostics to proto
	var pbDiags []*pb.Diagnostic
	for _, diag := range cached.Diags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, cached.Source))
	}

	return &pb.DiagnosticsResponse{
		Diagnostics: pbDiags,
	}, nil
}

// buildParseResponse constructs ParseFileResponse from cached model
func (s *Service) buildParseResponse(modelHash string, model *CachedModel) *pb.ParseFileResponse {
	// Convert diagnostics
	var pbDiags []*pb.Diagnostic
	for _, diag := range model.Diags {
		pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, model.Source))
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
			rootSymbol = SymbolToProto(rootSyms[0], model.Index)
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
