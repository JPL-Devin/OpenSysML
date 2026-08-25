package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsConfiguredPreflight(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("preflight reached the wrapped handler")
	}), map[string]struct{}{"https://example.test": {}})
	request := httptest.NewRequest(http.MethodOptions, "/sysml.SysMLService/GetServerInfo", nil)
	request.Header.Set("Origin", "https://example.test")
	request.Header.Set("Access-Control-Request-Method", "POST")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://example.test" {
		t.Errorf("allow origin = %q", got)
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Grpc-Timeout") {
		t.Error("preflight omitted Grpc-Timeout")
	}
	if !strings.Contains(response.Header().Get("Access-Control-Expose-Headers"), "Grpc-Status") {
		t.Error("response omitted Grpc-Status from exposed headers")
	}
}

func TestCORSRefusesUnlistedPreflight(t *testing.T) {
	handler := corsMiddleware(http.NotFoundHandler(), map[string]struct{}{"https://example.test": {}})
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://other.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("refused origin received an allow header")
	}
}

func TestCORSRejectsWildcard(t *testing.T) {
	if _, err := parseCORSOrigins("https://example.test, *"); err == nil {
		t.Fatal("wildcard origin was accepted")
	}
}
