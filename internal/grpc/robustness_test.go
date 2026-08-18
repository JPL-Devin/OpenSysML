package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestGRPCRobustness exercises failure modes: missing models, invalid symbols, parse errors.
// Each RPC must return typed errors, never panic.
func TestGRPCRobustness(t *testing.T) {
	service := mustNewService(t, 10) // cache size 10

	t.Run("parse_invalid_syntax", func(t *testing.T) {
		req := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test { invalid syntax (((",
			},
		}
		resp, err := service.ParseFile(context.Background(), req)
		if err != nil {
			t.Fatalf("ParseFile RPC failed: %v (should return diagnostics, not RPC error)", err)
		}
		if len(resp.Diagnostics) == 0 {
			t.Error("Expected diagnostics for invalid syntax, got none")
		}
	})

	t.Run("get_symbol_missing_model", func(t *testing.T) {
		req := &pb.GetSymbolRequest{
			ModelHash: "nonexistent_hash",
			SymbolId:  "test::Symbol",
		}
		_, err := service.GetSymbol(context.Background(), req)
		if err == nil {
			t.Error("Expected error for missing model, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})

	t.Run("evaluate_missing_model", func(t *testing.T) {
		req := &pb.EvaluateRequest{
			ModelHash:  "nonexistent_hash",
			Expression: "2 + 2",
		}
		_, err := service.Evaluate(context.Background(), req)
		if err == nil {
			t.Error("Expected error for missing model, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})

	t.Run("evaluate_parse_error", func(t *testing.T) {
		// First parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to evaluate invalid expression
		evalReq := &pb.EvaluateRequest{
			ModelHash:  parseResp.ModelHash,
			Expression: "invalid syntax (((",
		}
		evalResp, err := service.Evaluate(context.Background(), evalReq)
		if err != nil {
			t.Fatalf("Evaluate RPC failed: %v (should return error field, not RPC error)", err)
		}
		if evalResp.Error == "" {
			t.Error("Expected error field for invalid expression, got empty")
		}
	})

	t.Run("instantiate_missing_symbol", func(t *testing.T) {
		// Parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to instantiate non-existent symbol
		instReq := &pb.InstantiateRequest{
			ModelHash: parseResp.ModelHash,
			SymbolId:  "test::NonExistent",
		}
		instResp, err := service.Instantiate(context.Background(), instReq)
		if err != nil {
			t.Fatalf("Instantiate RPC failed: %v (should return error field, not RPC error)", err)
		}
		if instResp.Error == "" {
			t.Error("Expected error field for missing symbol, got empty")
		}
	})

	t.Run("execute_action_missing_symbol", func(t *testing.T) {
		// Parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to execute non-existent action
		execReq := &pb.ExecuteActionRequest{
			ModelHash:      parseResp.ModelHash,
			ActionSymbolId: "test::NonExistent",
			Inputs:         map[string]*pb.Value{},
		}
		execResp, err := service.ExecuteAction(context.Background(), execReq)
		if err != nil {
			t.Fatalf("ExecuteAction RPC failed: %v (should return error field, not RPC error)", err)
		}
		if execResp.Error == "" {
			t.Error("Expected error field for missing action, got empty")
		}
	})

	t.Run("execute_state_missing_symbol", func(t *testing.T) {
		// Parse a model
		parseReq := &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test {}",
			},
		}
		parseResp, err := service.ParseFile(context.Background(), parseReq)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		// Try to execute non-existent state machine
		execReq := &pb.ExecuteStateRequest{
			ModelHash:            parseResp.ModelHash,
			StateMachineSymbolId: "test::NonExistent",
			Events:               []string{},
		}
		execResp, err := service.ExecuteState(context.Background(), execReq)
		if err != nil {
			t.Fatalf("ExecuteState RPC failed: %v (should return error field, not RPC error)", err)
		}
		if execResp.Error == "" {
			t.Error("Expected error field for missing state machine, got empty")
		}
	})

	t.Run("parse_with_unavailable_standard_library", func(t *testing.T) {
		// A library that would not load leaves the index without it. The request
		// must still answer, reporting unresolved names as diagnostics.
		svc := mustNewService(t, 10)
		defer svc.Close()
		svc.libIndexes = newIndexPool(0, symbols.NewIndex)

		resp, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{
				Content: "package test { attribute def A { attribute x : ScalarValues::Real; } }",
			},
		})
		if err != nil {
			t.Fatalf("ParseFile RPC failed: %v (should return diagnostics, not RPC error)", err)
		}
		if len(resp.Diagnostics) == 0 {
			t.Error("Expected diagnostics for library types that did not load, got none")
		}
		if _, ok := svc.cache.Get(resp.ModelHash); !ok {
			t.Error("Expected the model to be cached despite the missing library")
		}
	})
}
