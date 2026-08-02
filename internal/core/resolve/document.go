package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ResolveDocument walks the document's references and resolves each, recording
// diagnostics on the Resolver. name identifies the document in the index.
func (r *Resolver) ResolveDocument(name string, root *ast.RootNamespace) {
	rootScope := r.idx.DocumentRoot(name)
	if rootScope == nil {
		return
	}
	r.walkMembers(rootScope, membersOf(root))
}

// membersOf returns the top-level members of a RootNamespace.
func membersOf(root *ast.RootNamespace) []ast.Node {
	if root == nil {
		return nil
	}
	return root.Members
}

// walkMembers resolves references in each member, descending into child scopes.
func (r *Resolver) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl, _ := unwrapForResolve(m)
		r.resolveDecl(scope, decl)
	}
}

// unwrapForResolve mirrors the builder's unwrapMember: it strips *ast.Membership
// wrappers so we resolve against the inner declaration.
func unwrapForResolve(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// resolveDecl resolves references contributed by a single declaration and
// recurses into declarations that own a child scope.
func (r *Resolver) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	switch d := decl.(type) {
	case *ast.Package:
		r.resolvePrefixes(scope, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	case *ast.Namespace:
		r.resolvePrefixes(scope, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	case *ast.Import:
		r.ResolveQualified(scope, d.Imported)
	case *ast.Alias:
		r.ResolveQualified(scope, d.For)
	case *ast.Dependency:
		r.resolvePrefixes(scope, d.Prefixes)
		for _, c := range d.Clients {
			r.ResolveQualified(scope, c)
		}
		for _, s := range d.Suppliers {
			r.ResolveQualified(scope, s)
		}
	case *ast.Comment:
		for _, a := range d.About {
			r.ResolveQualified(scope, a)
		}
	case *ast.FilterMember:
		r.resolveExpr(scope, d.Condition)
	case *ast.Definition:
		r.resolvePrefixes(scope, d.Prefixes)
		r.resolveRelationships(scope, d.Relationships)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	case *ast.Usage:
		r.resolvePrefixes(scope, d.Prefixes)
		r.resolveRelationships(scope, d.Relationships)
	if d.Multiplicity != nil {
		r.resolveExpr(scope, d.Multiplicity.Lower)
		r.resolveExpr(scope, d.Multiplicity.Upper)
	}
	r.resolveExpr(scope, d.Value)
	for _, end := range d.ConnectorEnds {
		// ConnectorEnd.Target is Node (QualifiedName or Expression)
		if qn, ok := end.Target.(*ast.QualifiedName); ok {
			r.ResolveQualified(scope, qn)
		} else {
			r.resolveExpr(scope, end.Target)
		}
	}
	if d.FlowEnds != nil {
		r.ResolveQualified(scope, d.FlowEnds.From)
			r.ResolveQualified(scope, d.FlowEnds.To)
			r.ResolveQualified(scope, d.FlowEnds.Payload)
		}
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	}
}

// childScope finds the child scope whose node is decl.
func (r *Resolver) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, c := range scope.Children() {
		if c.Node() == decl {
			return c
		}
	}
	return nil
}

func (r *Resolver) resolvePrefixes(scope *symbols.Scope, prefixes []*ast.PrefixMetadata) {
	for _, p := range prefixes {
		if p != nil {
			r.ResolveQualified(scope, p.Type)
		}
	}
}

// resolveRelationships resolves each relationship target as a qualified name.
func (r *Resolver) resolveRelationships(scope *symbols.Scope, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil && rel.Target != nil {
			// Unwrap FeatureReference if needed (relationship targets parsed as expressions)
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}
			if qn, ok := target.(*ast.QualifiedName); ok {
				r.ResolveQualified(scope, qn)
			} else if fc, ok := target.(*ast.FeatureChainExpr); ok {
				r.resolveFeatureChain(scope, fc)
			}
		}
	}
}

// resolveExpr walks an expression subtree resolving feature references and
// classification type references.
func (r *Resolver) resolveExpr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		r.ResolveQualified(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			r.resolveExpr(scope, op)
		}
		if v.TypeRef != nil {
			r.ResolveQualified(scope, v.TypeRef)
		}
	case *ast.FeatureChainExpr:
		r.resolveFeatureChain(scope, v)
	case *ast.IndexExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Index)
	case *ast.InvocationExpr:
		r.resolveExpr(scope, v.Operand)
		if v.Type != nil {
			r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
		for _, na := range v.NamedArgs {
			if na.Name != nil {
				r.ResolveQualified(scope, na.Name)
			}
			r.resolveExpr(scope, na.Value)
		}
	case *ast.CollectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.SelectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.ConstructorExpr:
		if v.Type != nil {
			r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
	case *ast.BodyExpr:
		r.resolveExpr(scope, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			r.resolveExpr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		r.ResolveQualified(scope, v.Ref)
	}
	// Literals (LiteralBool/String/Integer/Real/Infinity, NullExpr) have no refs.
}

// resolveFeatureChain resolves a FeatureChainExpr by resolving the operand
// then walking each member part explicitly, assigning symbols to each part.
func (r *Resolver) resolveFeatureChain(scope *symbols.Scope, fc *ast.FeatureChainExpr) {
	// Resolve operand first
	r.resolveExpr(scope, fc.Operand)
	
	// Get symbol of operand to find its member scope
	operandSym := r.getExprSymbol(scope, fc.Operand)
	if operandSym == nil || fc.Member == nil {
		return
	}
	
	// Walk member parts explicitly, assigning symbols to each part
	r.resolveMemberChain(operandSym, fc.Member)
}

// resolveMemberChain walks a qualified name member-by-member in the given scope,
// assigning each part's symbol explicitly (for feature chain member access).
func (r *Resolver) resolveMemberChain(parentSym *symbols.Symbol, qn *ast.QualifiedName) {
	if qn == nil || len(qn.Parts) == 0 {
		return
	}
	
	// Resolve first part using model.LookupMember if available, else scope.LookupLocal
	var cur *symbols.Symbol
	var ok bool
	
	if r.model != nil {
		// Use inheritance-aware lookup via semantics.Model
		type modelLookup interface {
			LookupMember(*symbols.Symbol, string) (*symbols.Symbol, bool)
		}
		if m, hasMethod := r.model.(modelLookup); hasMethod {
			cur, ok = m.LookupMember(parentSym, qn.Parts[0].Text)
		}
	}
	
	if !ok {
		// Fall back to local scope lookup if model unavailable
		if parentSym.Scope == nil {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Span:    qn.Parts[0].Span,
				Message: "no scope for member lookup in " + parentSym.Name,
			})
			return
		}
		cur, ok = parentSym.Scope.LookupLocal(qn.Parts[0].Text)
	}
	
	if !ok {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Span:    qn.Parts[0].Span,
			Message: "unresolved member: " + qn.Parts[0].Text,
		})
		return
	}
	qn.Parts[0].Sym = cur
	
	// Walk remaining parts via member lookup
	for i := 1; i < len(qn.Parts); i++ {
		var next *symbols.Symbol
		var found bool
		
		if r.model != nil {
			type modelLookup interface {
				LookupMember(*symbols.Symbol, string) (*symbols.Symbol, bool)
			}
			if m, hasMethod := r.model.(modelLookup); hasMethod {
				next, found = m.LookupMember(cur, qn.Parts[i].Text)
			}
		}
		
		if !found {
			// Fall back to local scope lookup
			if cur.Scope == nil {
				r.Diagnostics = append(r.Diagnostics, Diagnostic{
					Span:    qn.Parts[i].Span,
					Message: "no members in " + cur.Name,
				})
				return
			}
			next, found = cur.Scope.LookupLocal(qn.Parts[i].Text)
		}
		
		if !found {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Span:    qn.Parts[i].Span,
				Message: "unresolved member: " + qn.Parts[i].Text + " in " + cur.Name,
			})
			return
		}
		
		qn.Parts[i].Sym = next
		cur = next
	}
	
	// Store final resolution in memo
	r.memo[qn] = resolution{cur, true}
}

// getExprSymbol extracts the symbol referenced by an expression, used for
// member access chains. Returns nil if expression doesn't resolve to a symbol.
// For typed usages, returns the type's symbol (following typing relationships).
func (r *Resolver) getExprSymbol(scope *symbols.Scope, e ast.Node) *symbols.Symbol {
	switch v := e.(type) {
	case *ast.FeatureReference:
		if v.Name == nil {
			return nil
		}
		sym, ok := r.ResolveQualified(scope, v.Name)
		if !ok {
			return nil
		}
		// If this is a typed usage, follow the type
		if usage, isUsage := sym.Decl.(*ast.Usage); isUsage {
			typeSym := r.getUsageType(sym.OwnerScope, usage)
			if typeSym != nil {
				return typeSym
			}
		}
		return sym
	case *ast.FeatureChainExpr:
		// For chained access, get the final member's symbol
		if v.Member == nil {
			return nil
		}
		// First resolve the chain
		r.resolveFeatureChain(scope, v)
		// Get the operand's symbol, then lookup member
		operandSym := r.getExprSymbol(scope, v.Operand)
		if operandSym == nil || operandSym.Scope == nil {
			return nil
		}
		memberSym, ok := r.ResolveQualified(operandSym.Scope, v.Member)
		if !ok {
			return nil
		}
		// Follow type if member is usage
		if usage, isUsage := memberSym.Decl.(*ast.Usage); isUsage {
			typeSym := r.getUsageType(memberSym.OwnerScope, usage)
			if typeSym != nil {
				return typeSym
			}
		}
		return memberSym
	default:
		return nil
	}
}

// getUsageType returns the type symbol of a usage by resolving its typing relationship.
func (r *Resolver) getUsageType(scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	for _, rel := range usage.Relationships {
		if rel.Kind == ast.RelTyping && rel.Target != nil {
			// Unwrap FeatureReference if needed
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}
			if qn, ok := target.(*ast.QualifiedName); ok {
				typeSym, _ := r.ResolveQualified(scope, qn)
				return typeSym
			}
		}
	}
	return nil
}
