package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CodeEndpointNotANode marks an action endpoint naming something other than an
// action node accepted by the action lowerer.
const CodeEndpointNotANode = "endpoint-not-a-node"

// ActionEndpointPass checks endpoints written in action bodies against the
// nodes accepted by action lowering.
type ActionEndpointPass struct{}

func (ActionEndpointPass) Level() PassLevel { return LevelNameResolution }

func (ActionEndpointPass) ElementScoped() {}

func (ActionEndpointPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return nil
	}
	c := &actionEndpointChecker{ctx: ctx}
	c.walk(scope, root.Members)
	return c.diags
}

type actionEndpointChecker struct {
	ctx   *Context
	diags []Diagnostic
}

func (c *actionEndpointChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		decl := unwrapMembership(member)
		child := bodyScope(scope, decl)
		switch n := decl.(type) {
		case *ast.Package:
			c.walk(child, n.Members)
		case *ast.Namespace:
			c.walk(child, n.Members)
		case *ast.Definition:
			if n.Kind == ast.DefAction {
				c.checkBody(n, child)
			}
			c.walk(child, n.Members)
		case *ast.Usage:
			if n.Kind == ast.UsageAction {
				c.checkBody(n, child)
			}
			c.walk(child, n.Members)
		}
	}
}

func (c *actionEndpointChecker) checkBody(decl ast.Node, scope *symbols.Scope) {
	nodes, hasInitial, err := lower.ActionNodes(decl, scope)
	if err != nil || len(nodes) == 0 {
		return
	}
	for _, member := range declMembers(decl) {
		n := unwrapMembership(member)
		switch v := n.(type) {
		case *ast.InitialNode:
			c.checkEndpoint(scope, nodes, hasInitial, v.Successor, false, v)
		case *ast.SuccessionEdge:
			if v.SourceMember == nil && !v.SourceImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Source, true, v)
			}
			if v.TargetMember == nil && !v.TargetImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Target, false, v)
			}
		case *ast.ControlFlowEdge:
			if v.SourceMember == nil && !v.SourceImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Source, true, v)
			}
			if v.TargetMember == nil && !v.TargetImplied {
				c.checkEndpoint(scope, nodes, hasInitial, v.Target, false, v)
			}
		case *ast.TransitionMember:
			if v.Source != nil {
				c.checkEndpoint(scope, nodes, hasInitial, v.Source, true, v)
			}
			c.checkEndpoint(scope, nodes, hasInitial, v.Target, false, v)
		case *ast.Usage:
			if v.Kind != ast.UsageSuccession || len(v.ConnectorEnds) != 2 {
				continue
			}
			if redefinesUsage(v) {
				continue
			}
			c.checkEndpoint(scope, nodes, hasInitial, connectorEndReference(v.ConnectorEnds[0]), true, v)
			c.checkEndpoint(scope, nodes, hasInitial, connectorEndReference(v.ConnectorEnds[1]), false, v)
		}
	}
}

func (c *actionEndpointChecker) checkEndpoint(
	scope *symbols.Scope,
	nodes []ast.Node,
	hasInitial bool,
	ref ast.Node,
	sourceEnd bool,
	subject ast.Node,
) {
	if ref == nil || lower.ActionEndpointAccepted(nodes, hasInitial, ref, sourceEnd) {
		return
	}
	qn := actionEndpointName(ref)
	if qn == nil {
		return
	}
	sym, ok := c.ctx.Resolver().EndpointSymbol(scope, qn)
	if !ok || sym == nil || c.ctx.DownstreamOfFailure(sym.Decl) ||
		c.ctx.DownstreamOfFailure(subject) {
		return
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     ref.Span(),
		Message:  "succession endpoint " + actionEndpointText(ref) + " is not an action node",
		Code:     CodeEndpointNotANode,
		Source:   "action-endpoint",
	})
}

func redefinesUsage(usage *ast.Usage) bool {
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}

func connectorEndReference(end *ast.ConnectorEnd) ast.Node {
	if end == nil {
		return nil
	}
	return end.AttachedTarget()
}

func actionEndpointName(ref ast.Node) *ast.QualifiedName {
	if qn := ast.AsQualifiedName(ref); qn != nil {
		return qn
	}
	chain, ok := ref.(*ast.FeatureChainExpr)
	if !ok {
		return nil
	}
	return actionEndpointName(chain.Operand)
}

func actionEndpointText(ref ast.Node) string {
	if text := lower.FeaturePath(ref); text != "" {
		return text
	}
	return ast.SimpleName(ref)
}
