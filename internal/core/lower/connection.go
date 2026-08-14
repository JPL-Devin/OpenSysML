package lower

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Connection is a lowered connector: the ends it joins, by the name each end
// resolves to. A `connect a to b` has two ends; a multi-end `connect` has more,
// and every end is reachable from every other.
//
// A connection a `variant interface` declares joins its ends only where that
// variant is the one selected, so it carries the variation it belongs to and
// its own name: routing must ask which variant was selected before delivering
// through it (SysML v2 §7.20).
type Connection struct {
	Ends      []string
	Variation string // variation point the connection is a variant of, empty when it is not
	Variant   string // name of the variant declaring it, empty when it is not one
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
		// A variation point declares no ends of its own: the connections are the
		// ones its variants declare, of which a selection realizes one.
		if u.IsVariation {
			out = append(out, lowerVariantConnections(u)...)
			continue
		}
		if conn, ok := lowerConnection(u); ok {
			out = append(out, conn)
		}
	}
	return out
}

// lowerConnection extracts the ends of one connector usage. A usage joining
// fewer than two ends joins nothing and is dropped.
func lowerConnection(u *ast.Usage) (Connection, bool) {
	ends := make([]string, 0, len(u.ConnectorEnds))
	for _, end := range u.ConnectorEnds {
		if name := endName(end); name != "" {
			ends = append(ends, name)
		}
	}
	if len(ends) < 2 {
		return Connection{}, false
	}
	return Connection{Ends: ends}, true
}

// lowerVariantConnections extracts the connections the variants of a variation
// connector declare, each tagged with the variation and variant it came from so
// routing can honor the selection.
func lowerVariantConnections(variation *ast.Usage) []Connection {
	name := variation.Ident.Name
	var out []Connection
	for _, member := range variation.Members {
		u, isUsage := unwrapMembership(member).(*ast.Usage)
		if !isUsage || !u.IsVariant || !connectorKind(u.Kind) {
			continue
		}
		conn, ok := lowerConnection(u)
		if !ok {
			continue
		}
		conn.Variation, conn.Variant = name, u.Ident.Name
		out = append(out, conn)
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
	target := end.AttachedTarget()
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
