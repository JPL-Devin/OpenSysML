package ast

import "testing"

func qn(name string) *QualifiedName {
	return &QualifiedName{Parts: []NameSegment{{Text: name}}}
}

func TestDumpConnectorEnds(t *testing.T) {
	u := &Usage{
		Kind:          UsageConnection,
		Ident:         Identification{Name: "c"},
		ConnectorEnds: []*ConnectorEnd{
			{Target: qn("a")},
			{Target: qn("b")},
		},
	}
	got := Dump(u)
	want := `(Usage kind="connection" name="c" ref=false direction="none" composite=false derived=false ordered=false nonunique=false ends="a, b")`
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpFlowEnds(t *testing.T) {
	u := &Usage{
		Kind:  UsageFlow,
		Ident: Identification{Name: "f"},
		FlowEnds: &FlowEnds{
			From:    qn("a"),
			To:      qn("b"),
			Payload: qn("Fuel"),
		},
	}
	got := Dump(u)
	want := "(Usage kind=\"flow\" name=\"f\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (FlowEnds from=\"a\" to=\"b\" payload=\"Fuel\"))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpConjugated(t *testing.T) {
	u := &Usage{
		Kind:         UsagePort,
		Ident:        Identification{Name: "p"},
		IsConjugated: true,
	}
	got := Dump(u)
	want := `(Usage kind="port" name="p" ref=false direction="none" composite=false derived=false ordered=false nonunique=false conjugated=true)`
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
