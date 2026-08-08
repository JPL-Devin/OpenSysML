package lower

import "testing"

func TestLowerConnectionsFromActionBody(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			done end;
			then start end;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := PeerPorts(graph.Connections, "outPort"); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("PeerPorts(outPort) = %v, want [inPort]", got)
	}
	// A connection joins its ends without a direction, so routing works either way.
	if got := PeerPorts(graph.Connections, "inPort"); len(got) != 1 || got[0] != "outPort" {
		t.Errorf("PeerPorts(inPort) = %v, want [outPort]", got)
	}
}

// An end that declares its own name attaches to the feature it
// reference-subsets, so that is what routing must join.
func TestLowerConnectionEndsThatDeclareTheirOwnName(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port outPort;
			port inPort;
			connection : Link connect
				source references outPort to
				target references inPort;
			first start;
			done end;
			then start end;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := PeerPorts(graph.Connections, "outPort"); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("PeerPorts(outPort) = %v, want [inPort]", got)
	}
}

func TestLowerSendRecordsViaForm(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p;
			first start;
			action viaSend { send 1 via p; }
			action toSend { send 2 to other; }
			done end;
			then start viaSend;
			then viaSend toSend;
			then toSend end;
		}
	`)
	var sends []Send
	for _, body := range graph.Bodies {
		for _, stmt := range body {
			if send, ok := stmt.(Send); ok {
				sends = append(sends, send)
			}
		}
	}
	if len(sends) != 2 {
		t.Fatalf("lowered %d sends, want 2", len(sends))
	}
	via, addressed := sends[0], sends[1]
	if via.Target != "p" {
		via, addressed = addressed, via
	}
	if !via.IsVia || via.Target != "p" {
		t.Errorf("`send 1 via p` lowered to %+v, want IsVia with Target p", via)
	}
	if addressed.IsVia || addressed.Target != "other" {
		t.Errorf("`send 2 to other` lowered to %+v, want no IsVia with Target other", addressed)
	}
}

func TestLowerAcceptRecordsViaPort(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port inPort;
			first start;
			action r accept msg : Integer via inPort;
			done end;
			then start r;
			then r end;
		}
	`)
	if len(graph.Accepts) != 1 {
		t.Fatalf("Accepts = %v, want one accept", graph.Accepts)
	}
	for _, accept := range graph.Accepts {
		if accept.ViaPort != "inPort" {
			t.Errorf("Accept = %+v, want ViaPort inPort", accept)
		}
	}
}

func TestPeerPortsIgnoresUnconnectedAndSelf(t *testing.T) {
	conns := []Connection{{Ends: []string{"a", "b"}}, {Ends: []string{"a", "a"}}}
	if got := PeerPorts(conns, "lonely"); got != nil {
		t.Errorf("PeerPorts(lonely) = %v, want nothing", got)
	}
	// A port joined to itself reaches nobody but itself, which is not a peer.
	if got := PeerPorts(conns, "a"); len(got) != 1 || got[0] != "b" {
		t.Errorf("PeerPorts(a) = %v, want [b]", got)
	}
	if got := PeerPorts(nil, "a"); got != nil {
		t.Errorf("PeerPorts with no connections = %v, want nothing", got)
	}
}

func TestPeerPortsSpansMultiEndAndMultipleConnections(t *testing.T) {
	conns := []Connection{
		{Ends: []string{"hub", "left", "right"}},
		{Ends: []string{"hub", "extra"}},
		{Ends: []string{"hub", "left"}},
	}
	got := PeerPorts(conns, "hub")
	want := []string{"left", "right", "extra"}
	if len(got) != len(want) {
		t.Fatalf("PeerPorts(hub) = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("PeerPorts(hub) = %v, want %v", got, want)
		}
	}
}
