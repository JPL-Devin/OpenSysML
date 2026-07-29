package lsp

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// qnRef is a qualified-name reference paired with the scope it resolves against.
type qnRef struct {
	qn    *ast.QualifiedName
	scope *symbols.Scope
}

// collectRefs walks a document's scope tree and AST, gathering every
// QualifiedName reference with its resolution scope. It mirrors
// resolve/document.go's traversal.
func collectRefs(root *ast.RootNamespace, rootScope *symbols.Scope) []qnRef {
	c := &refCollector{}
	if root != nil && rootScope != nil {
		c.walkMembers(rootScope, root.Members)
	}
	return c.refs
}

type refCollector struct {
	refs []qnRef
}

func (c *refCollector) add(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn != nil {
		c.refs = append(c.refs, qnRef{qn: qn, scope: scope})
	}
}

func (c *refCollector) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, ch := range scope.Children() {
		if ch.Node() == decl {
			return ch
		}
	}
	return nil
}

func (c *refCollector) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl := m
		switch v := m.(type) {
		case *ast.Membership:
			decl = v.Member
		}
		c.resolveDecl(scope, decl)
	}
}

func (c *refCollector) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	switch d := decl.(type) {
	case *ast.Package:
		c.prefixes(scope, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
	case *ast.Namespace:
		c.prefixes(scope, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
	case *ast.Import:
		c.add(scope, d.Imported)
	case *ast.Alias:
		c.add(scope, d.For)
	case *ast.Dependency:
		c.prefixes(scope, d.Prefixes)
		for _, cl := range d.Clients {
			c.add(scope, cl)
		}
		for _, sp := range d.Suppliers {
			c.add(scope, sp)
		}
	case *ast.Comment:
		for _, a := range d.About {
			c.add(scope, a)
		}
	case *ast.FilterMember:
		c.expr(scope, d.Condition)
	}
}

func (c *refCollector) prefixes(scope *symbols.Scope, prefixes []*ast.PrefixMetadata) {
	for _, p := range prefixes {
		if p != nil {
			c.add(scope, p.Type)
		}
	}
}

func (c *refCollector) expr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		c.add(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			c.expr(scope, op)
		}
		c.add(scope, v.TypeRef)
	case *ast.FeatureChainExpr:
		c.expr(scope, v.Operand)
		c.add(scope, v.Member)
	case *ast.IndexExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Index)
	case *ast.InvocationExpr:
		c.expr(scope, v.Operand)
		c.add(scope, v.Type)
		for _, a := range v.Args {
			c.expr(scope, a)
		}
		for _, na := range v.NamedArgs {
			c.add(scope, na.Name)
			c.expr(scope, na.Value)
		}
	case *ast.CollectExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Body)
	case *ast.SelectExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Body)
	case *ast.ConstructorExpr:
		c.add(scope, v.Type)
		for _, a := range v.Args {
			c.expr(scope, a)
		}
	case *ast.BodyExpr:
		c.expr(scope, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			c.expr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		c.add(scope, v.Ref)
	}
}

// refAtOffset returns the qnRef whose qualified-name span contains offset.
func refAtOffset(refs []qnRef, offset int) *qnRef {
	for i := range refs {
		sp := refs[i].qn.Span()
		if offset >= sp.Offset && offset < sp.End() {
			return &refs[i]
		}
	}
	return nil
}
