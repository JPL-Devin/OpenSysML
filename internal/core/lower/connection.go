package lower

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Connection is a lowered connector: the ends it joins, by the name each end
// resolves to. A `connect a to b` has two ends; a multi-end `connect` has more,
// and every end is reachable from every other.
type Connection struct {
	Ends []string
}

// lowerConnections extracts the connectors declared among members, in
// declaration order. Usages that are connector-shaped but declare fewer than
// two ends join nothing and are dropped: the constraint pass reports them, and
// carrying them would only make routing check the same thing again.
func lowerConnections(members []ast.Node) []Connection {
	var out []Connection
	for _, member := range members {
		u, isUsage := unwrapMembership(member).(*ast.Usage)
		if !isUsage || !connectorKind(u.Kind) {
			continue
		}
		ends := make([]string, 0, len(u.ConnectorEnds))
		for _, end := range u.ConnectorEnds {
			if name := endName(end); name != "" {
				ends = append(ends, name)
			}
		}
		if len(ends) < 2 {
			continue
		}
		out = append(out, Connection{Ends: ends})
	}
	return out
}

// connectorKind reports whether a usage kind declares connector ends that route
// messages. Flows are excluded: they carry a payload along a declared direction
// rather than joining ends for message passing, and are lowered as data flows.
func connectorKind(k ast.UsageKind) bool {
	switch k {
	case ast.UsageConnection, ast.UsageConnector, ast.UsageInterface, ast.UsageAllocation:
		return true
	}
	return false
}

// endName names the feature an end attaches to. A chain (`sensor.out`) attaches
// to its last segment, which is the port itself. An end that declares its own
// name (`bead references t.bead`) attaches to what it reference-subsets.
func endName(end *ast.ConnectorEnd) string {
	if end == nil {
		return ""
	}
	target := end.Target
	if _, declaresName := end.DeclaredName(); declaresName {
		target = end.ReferencedTarget()
	}
	if target == nil {
		target = end.Reference
	}
	if chain, isChain := target.(*ast.FeatureChainExpr); isChain {
		return ast.SimpleName(chain.Member)
	}
	return ast.SimpleName(target)
}

// PeerPorts returns the ends connected to port, across every connection it
// participates in, in declaration order and without duplicates. A port is never
// its own peer, so a connection joining a port to itself yields nothing.
func PeerPorts(conns []Connection, port string) []string {
	if port == "" {
		return nil
	}
	var peers []string
	seen := map[string]bool{port: true}
	for _, conn := range conns {
		if !contains(conn.Ends, port) {
			continue
		}
		for _, end := range conn.Ends {
			if seen[end] {
				continue
			}
			seen[end] = true
			peers = append(peers, end)
		}
	}
	return peers
}

func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
