// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const connectTestModel = "package T { part def P { attribute a = 7; } }"

// connectServer starts the Connect handler over cleartext HTTP, which is how the
// prototype serves a deployment that offers no TLS.
func connectServer(t *testing.T) string {
	t.Helper()

	svc, err := sysmlgrpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	srv := httptest.NewServer(h2c.NewHandler(connectHandler(svc, "test"), &http2.Server{}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// h2cClient dials cleartext HTTP/2, which the gRPC and full Connect protocols
// need and which Go's default transport will not do.
func h2cClient() *http.Client {
	return &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}}
}

// TestConnectServesEveryProtocol establishes the claim the evaluation rests on:
// one handler answers the Connect protocol, gRPC and gRPC-Web.
func TestConnectServesEveryProtocol(t *testing.T) {
	url := connectServer(t)

	tests := []struct {
		name    string
		client  *http.Client
		options []connect.ClientOption
	}{
		{"connect protocol, protobuf body", http.DefaultClient, nil},
		{"connect protocol, JSON body", http.DefaultClient,
			[]connect.ClientOption{connect.WithProtoJSON()}},
		{"grpc", h2cClient(), []connect.ClientOption{connect.WithGRPC()}},
		{"grpc-web", http.DefaultClient, []connect.ClientOption{connect.WithGRPCWeb()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := protoconnect.NewSysMLServiceClient(test.client, url, test.options...)

			parsed, err := client.ParseFile(context.Background(), connect.NewRequest(
				&pb.ParseFileRequest{
					Source: &pb.ParseFileRequest_Content{Content: connectTestModel},
				}))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}

			evaluated, err := client.Evaluate(context.Background(), connect.NewRequest(
				&pb.EvaluateRequest{
					Expression: "1 + 2 * 3",
					ModelHash:  parsed.Msg.ModelHash,
				}))
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got := evaluated.Msg.GetResult().GetIntValue(); got != 7 {
				t.Errorf("Evaluate = %d, want 7", got)
			}
		})
	}
}

// TestConnectAnswersAPlainPOST is the curl case: a client with no generated
// code, no gRPC library and no HTTP/2 posts JSON and reads JSON.
func TestConnectAnswersAPlainPOST(t *testing.T) {
	url := connectServer(t)

	post := func(procedure, body string) map[string]any {
		t.Helper()

		res, err := http.Post(url+"/sysml.SysMLService/"+procedure,
			"application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", procedure, err)
		}
		defer res.Body.Close()

		if res.ProtoMajor != 1 {
			t.Errorf("%s answered over HTTP/%d, want HTTP/1.1", procedure, res.ProtoMajor)
		}
		var answer map[string]any
		if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
			t.Fatalf("decode %s answer: %v", procedure, err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: HTTP %d: %v", procedure, res.StatusCode, answer)
		}
		return answer
	}

	parsed := post("ParseFile", `{"content":"`+connectTestModel+`"}`)
	hash, ok := parsed["modelHash"].(string)
	if !ok || hash == "" {
		t.Fatalf("ParseFile reported no modelHash: %v", parsed)
	}

	evaluated := post("Evaluate", `{"expression":"1 + 2 * 3","modelHash":"`+hash+`"}`)
	// A JSON body spells an int64 as a string, which a client reading the
	// answer by hand has to know.
	result, _ := evaluated["result"].(map[string]any)
	if got := result["intValue"]; got != "7" {
		t.Errorf("Evaluate = %v, want \"7\"", got)
	}
}

// TestConnectServesGRPCClientsUnchanged checks the generated grpc-go stub — the
// same code path the Python client and grpcurl use — against the Connect
// handler, since migration risk turns on that working with no client change.
func TestConnectServesGRPCClientsUnchanged(t *testing.T) {
	url := strings.TrimPrefix(connectServer(t), "http://")

	conn, err := grpc.Dial(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewSysMLServiceClient(conn)

	parsed, err := client.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: connectTestModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	instantiated, err := client.Instantiate(context.Background(), &pb.InstantiateRequest{
		ModelHash: parsed.ModelHash,
		SymbolId:  "T::P",
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := instantiated.GetInstance().GetTypeSymbolId(); got != "T::P" {
		t.Errorf("Instantiate type = %q, want T::P", got)
	}
}

// TestConnectReportsStatusCodes checks a Connect client reads the same codes a
// gRPC client reads, since the clients share their error handling.
func TestConnectReportsStatusCodes(t *testing.T) {
	client := protoconnect.NewSysMLServiceClient(http.DefaultClient, connectServer(t))

	_, err := client.Evaluate(context.Background(), connect.NewRequest(&pb.EvaluateRequest{
		Expression: "1 + 1",
		ModelHash:  "missing",
	}))
	if err == nil {
		t.Fatal("evaluating against an unknown model succeeded")
	}
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Errorf("code = %s, want %s", got, connect.CodeNotFound)
	}
}
