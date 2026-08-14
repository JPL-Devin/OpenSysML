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

// An n-ary connection joins every end it declares, and every end is a peer of
// every other one (SysML v2 §7.13.2): lowering must not truncate to a pair.
func TestLowerNaryConnectionKeepsEveryEnd(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p1;
			port p2;
			port p3;
			port p4;
			connection link connect (p1, p2, p3, p4);
			first start;
			done end;
			then start end;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := graph.Connections[0].Ends; len(got) != 4 {
		t.Fatalf("lowered ends = %v, want four", got)
	}
	for _, port := range []string{"p1", "p2", "p3", "p4"} {
		peers := PeerPorts(graph.Connections, port)
		if len(peers) != 3 {
			t.Errorf("PeerPorts(%s) = %v, want the other three ends", port, peers)
		}
		if contains(peers, port) {
			t.Errorf("PeerPorts(%s) = %v, an end is not its own peer", port, peers)
		}
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

// The anonymous inline form declares no connection name, but its ends must
// still reach routing.
func TestLowerAnonymousNaryConnectionKeepsEveryEnd(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p1;
			port p2;
			port p3;
			connect (p1, p2, p3);
			first start;
			done end;
			then start end;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := graph.Connections[0].Ends; len(got) != 3 {
		t.Fatalf("lowered ends = %v, want three", got)
	}
	for _, port := range []string{"p1", "p2", "p3"} {
		if peers := PeerPorts(graph.Connections, port); len(peers) != 2 {
			t.Errorf("PeerPorts(%s) = %v, want the other two ends", port, peers)
		}
	}
}

// A `variation interface` declares no ends of its own: what joins ends are the
// connections its variants declare, and each is lowered tagged with the
// variation and variant it came from so routing can honor the selection.
func TestLowerVariantConnectionsCarryTheirVariation(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			port p1;
			port p2;
			port p3;
			variation interface link {
				variant interface direct connect p1 to p2;
				variant interface indirect connect p1 to p3;
			}
			first start;
			done end;
			then start end;
		}
	`)
	if len(graph.Connections) != 2 {
		t.Fatalf("Connections = %v, want the two variants' connections", graph.Connections)
	}
	for i, want := range []Connection{
		{Ends: []string{"p1", "p2"}, Variation: "link", Variant: "direct"},
		{Ends: []string{"p1", "p3"}, Variation: "link", Variant: "indirect"},
	} {
		got := graph.Connections[i]
		if got.Variation != want.Variation || got.Variant != want.Variant {
			t.Errorf("connection %d = %+v, want variation %s variant %s",
				i, got, want.Variation, want.Variant)
		}
		if len(got.Ends) != 2 || got.Ends[0] != want.Ends[0] || got.Ends[1] != want.Ends[1] {
			t.Errorf("connection %d ends = %v, want %v", i, got.Ends, want.Ends)
		}
	}
}

// A connection declared outside a variation belongs to no variant, so nothing
// about it is conditional on a selection.
func TestLowerPlainConnectionCarriesNoVariant(t *testing.T) {
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
	if conn := graph.Connections[0]; conn.Variation != "" || conn.Variant != "" {
		t.Errorf("connection = %+v, want no variation and no variant", conn)
	}
}

// An end reached through a chain attaches to the port the chain names, which is
// its last segment: `sensor.out` joins `out`, not `sensor`.
func TestLowerConnectionEndFollowsAFeatureChain(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			part sensor { port out; }
			port inPort;
			connect sensor.out to inPort;
			first start;
			done end;
			then start end;
		}
	`)
	if len(graph.Connections) != 1 {
		t.Fatalf("Connections = %v, want one connection", graph.Connections)
	}
	if got := PeerPorts(graph.Connections, "out"); len(got) != 1 || got[0] != "inPort" {
		t.Errorf("PeerPorts(out) = %v, want [inPort]", got)
	}
}
