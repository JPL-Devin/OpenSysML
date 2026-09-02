package flexo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

func triple(subject, predicate string, object rdf.Term) rdf.Triple {
	return rdf.Triple{Subject: rdf.IRI(subject), Predicate: rdf.IRI(predicate), Object: object}
}

func TestCommitRequestSpellsEveryChangeKind(t *testing.T) {
	x := rdf.Element + "X1"
	changes := []reposync.ElementChange{
		{Kind: reposync.KindUpdate, ID: "X1", Content: []rdf.Triple{
			triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"PartDefinition")),
			triple(x, rdf.SysML+"declaredName", rdf.String("renamed")),
			triple(x, rdf.SysML+"isAbstract", rdf.TypedLiteral("true", rdf.XSD+"boolean")),
			triple(x, rdf.SysML+"ownedMember", rdf.IRI(rdf.Element+"Y1")),
			triple(x, rdf.SysML+"ownedMember", rdf.IRI(rdf.Expression+"Y2")),
			triple(x, rdf.SysML+"count", rdf.TypedLiteral("3", rdf.XSD+"integer")),
			triple(x, rdf.SysML+"mass", rdf.TypedLiteral("1.5", rdf.XSD+"decimal")),
			triple(x, rdf.SysML+"owner", rdf.IRI(rdf.RDFNS+"nil")),
		}},
		{Kind: reposync.KindCreate, ID: "N1", Content: []rdf.Triple{
			triple(rdf.Element+"N1", rdf.RDFType, rdf.IRI(rdf.SysML+"Package")),
		}},
		{Kind: reposync.KindDelete, ID: "D1"},
	}
	body, err := commitRequest(changes, "sync")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"@type":       "Commit",
		"description": "sync",
		"change": []any{
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "X1"},
				"payload": map[string]any{
					"@id":          "X1",
					"@type":        "PartDefinition",
					"declaredName": "renamed",
					"isAbstract":   true,
					"ownedMember":  []any{map[string]any{"@id": "Y1"}, map[string]any{"@id": "Y2"}},
					"count":        float64(3),
					"mass":         1.5,
					"owner":        nil,
				},
			},
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "N1"},
				"payload":  map[string]any{"@id": "N1", "@type": "Package"},
			},
			map[string]any{
				"@type":    "DataVersion",
				"identity": map[string]any{"@id": "D1"},
				"payload":  nil,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("commit request:\n got %s\nwant %+v", body, want)
	}
}

func TestCommitRequestRefusesWhatTheServiceCannotHold(t *testing.T) {
	x := rdf.Element + "X1"
	typed := triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"PartDefinition"))
	cases := []struct {
		name    string
		content []rdf.Triple
		want    error
	}{
		{"untyped", []rdf.Triple{triple(x, rdf.SysML+"declaredName", rdf.String("a"))}, ErrUntyped},
		{"two types", []rdf.Triple{typed, triple(x, rdf.RDFType, rdf.IRI(rdf.SysML+"Package"))}, ErrUntyped},
		{"foreign property", []rdf.Triple{typed, triple(x, "urn:opensysml:sysml:memberIndex", rdf.String("0"))}, &UnrepresentableError{}},
		{"language tag", []rdf.Triple{typed, triple(x, rdf.SysML+"declaredName", rdf.Term{Kind: rdf.TermLiteral, Value: "a", Lang: "en"})}, &UnrepresentableError{}},
		{"foreign IRI", []rdf.Triple{typed, triple(x, rdf.SysML+"owner", rdf.IRI("http://example.org/x"))}, &UnrepresentableError{}},
		{"odd datatype", []rdf.Triple{typed, triple(x, rdf.SysML+"when", rdf.TypedLiteral("2020-01-01", rdf.XSD+"date"))}, &UnrepresentableError{}},
		{"non-numeric integer", []rdf.Triple{typed, triple(x, rdf.SysML+"n", rdf.TypedLiteral("+07", rdf.XSD+"integer"))}, &UnrepresentableError{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := commitRequest([]reposync.ElementChange{{Kind: reposync.KindCreate, ID: "X1", Content: tc.content}}, "")
			if err == nil {
				t.Fatal("accepted")
			}
			var unrepresentable *UnrepresentableError
			switch tc.want.(type) {
			case *UnrepresentableError:
				if !errors.As(err, &unrepresentable) {
					t.Errorf("want an UnrepresentableError, got %v", err)
				}
			default:
				if !errors.Is(err, tc.want) {
					t.Errorf("want %v, got %v", tc.want, err)
				}
			}
		})
	}
}

func TestRepresentationCarriesWhatTheServiceStores(t *testing.T) {
	x := rdf.Element + "X1"
	rep := Representation{}
	dropped, ok, err := rep.Carry(triple(x, "urn:opensysml:sysml:memberIndex", rdf.String("0")))
	if err != nil || ok {
		t.Errorf("a foreign property was carried: %+v, %v", dropped, err)
	}
	cases := []struct {
		in, want rdf.Term
	}{
		{rdf.TypedLiteral("a", rdf.XSD+"string"), rdf.String("a")},
		{rdf.TypedLiteral("2", rdf.XSD+"decimal"), rdf.TypedLiteral("2", rdf.XSD+"integer")},
		{rdf.TypedLiteral("2.5", rdf.XSD+"decimal"), rdf.TypedLiteral("2.5", rdf.XSD+"decimal")},
		{rdf.TypedLiteral("-4", rdf.XSD+"integer"), rdf.TypedLiteral("-4", rdf.XSD+"integer")},
		{rdf.TypedLiteral("false", rdf.XSD+"boolean"), rdf.TypedLiteral("false", rdf.XSD+"boolean")},
		{rdf.IRI(rdf.Expression + "E"), rdf.IRI(rdf.Expression + "E")},
		{rdf.IRI(rdf.RDFNS + "nil"), rdf.IRI(rdf.RDFNS + "nil")},
	}
	for _, tc := range cases {
		got, ok, err := rep.Carry(triple(x, rdf.SysML+"p", tc.in))
		if err != nil || !ok {
			t.Errorf("%s: not carried: %v", tc.in, err)
			continue
		}
		if got.Object != tc.want {
			t.Errorf("%s: carried as %s, want %s", tc.in, got.Object, tc.want)
		}
	}
	if _, _, err := rep.Carry(triple(x, rdf.SysML+"p", rdf.TypedLiteral("1e3", rdf.XSD+"decimal"))); err == nil {
		t.Error("a non-JSON number was carried")
	}
}

// stack fakes the two services for one repository: the commit path records
// what it was sent, the query path answers with a fixed result set.
func stack(t *testing.T, results string) (*Client, *[][]byte) {
	t.Helper()
	var posted [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/projects/p/commits":
			if r.URL.Query().Get("branchId") != "b" {
				t.Errorf("commit posted to branch %q, want b", r.URL.Query().Get("branchId"))
			}
			posted = append(posted, body)
			_, _ = io.WriteString(w, `{"@id":"c-1","@type":"Commit"}`)
		case r.URL.Path == "/orgs/o/repos/p/branches/b/query", r.URL.Path == "/orgs/o/repos/p/locks/Commit.c-0/query":
			if r.Header.Get("Accept") != "application/sparql-results+json" {
				t.Errorf("query without a JSON results Accept: %q", r.Header.Get("Accept"))
			}
			_, _ = io.WriteString(w, results)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return New(Config{Layer1URL: server.URL, SysMLV2URL: server.URL, Token: "t", Org: "o"}), &posted
}

const sparqlFixture = `{"head":{"vars":["s","p","o"]},"results":{"bindings":[
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"http://www.w3.org/1999/02/22-rdf-syntax-ns#type"},"o":{"type":"uri","value":"https://www.omg.org/spec/SysML#PartDefinition"}},
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"https://www.omg.org/spec/SysML#declaredName"},"o":{"type":"literal","value":"X"}},
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"https://www.omg.org/spec/SysML#count"},"o":{"type":"literal","datatype":"http://www.w3.org/2001/XMLSchema#integer","value":"3"}},
{"s":{"type":"uri","value":"urn:sysmlv2:element:X1"},"p":{"type":"uri","value":"https://www.omg.org/spec/SysML#note"},"o":{"type":"literal","xml:lang":"en","value":"hi"}}
]}}`

func TestRepositoryReadsGraphsAndWritesCommits(t *testing.T) {
	client, posted := stack(t, sparqlFixture)
	repo := client.Repository("p", "b")
	ctx := context.Background()

	graph, err := repo.Graph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	x := rdf.IRI(rdf.Element + "X1")
	if got, _ := graph.Object(x, rdf.SysML+"count"); got != rdf.TypedLiteral("3", rdf.XSD+"integer") {
		t.Errorf("typed literal read back as %s", got)
	}
	if got, _ := graph.Object(x, rdf.SysML+"declaredName"); got != rdf.String("X") {
		t.Errorf("plain literal read back as %s", got)
	}
	if got, _ := graph.Object(x, rdf.SysML+"note"); got.Lang != "en" {
		t.Errorf("language tag lost: %s", got)
	}
	if at, err := repo.GraphAt(ctx, "c-0"); err != nil || len(at.Triples()) != 4 {
		t.Errorf("commit graph: %d triple(s), %v", len(at.Triples()), err)
	}

	commit, err := repo.Commit(ctx, []reposync.ElementChange{{Kind: reposync.KindDelete, ID: "X1"}}, "bye")
	if err != nil {
		t.Fatal(err)
	}
	if commit != "c-1" || len(*posted) != 1 {
		t.Errorf("commit %q from %d post(s), want c-1 from 1", commit, len(*posted))
	}
}

func TestSelectGraphRefusesBlankNodes(t *testing.T) {
	client, _ := stack(t, `{"results":{"bindings":[{"s":{"type":"bnode","value":"b0"},"p":{"type":"uri","value":"p"},"o":{"type":"literal","value":"x"}}]}}`)
	if _, err := client.Repository("p", "b").Graph(context.Background()); !errors.Is(err, ErrBlankNode) {
		t.Errorf("want ErrBlankNode, got %v", err)
	}
}
