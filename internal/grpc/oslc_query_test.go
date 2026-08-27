package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

func TestOSLCQueryText(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		OslcQuery: `oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Elements) != 3 || resp.Elements[0].Id != "Demo::vehicle" {
		t.Fatalf("elements = %#v", resp.Elements)
	}
	if resp.Elements[0].Properties[QueryPropName] != "vehicle" {
		t.Fatalf("properties = %#v", resp.Elements[0].Properties)
	}
}

func TestOSLCQueryMutuallyExclusive(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{},
		OslcQuery: `sysml:type=PartUsage`,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestOSLCQueryMatchesREPLQuery(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	const text = `oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name,sysml:owner`
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		OslcQuery: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := repl.NewSession()
	if result := session.Submit(queryModel); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	lines, err := session.Query(text)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(resp.Elements))
	for _, element := range resp.Elements {
		line := fmt.Sprintf("%s  %s", element.Id, element.Type)
		for _, property := range []string{QueryPropName, QueryPropOwner} {
			if value, ok := element.Properties[property]; ok {
				line += fmt.Sprintf("  %s=%s", property, value)
			}
		}
		want = append(want, line)
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("REPL lines = %v, gRPC lines = %v", lines, want)
	}
}
