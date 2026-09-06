package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// subjectDecl is a case's subject as its body declares it: the object the case
// is about, which binds as the case's first input parameter (SysML v2 §7.21).
type subjectDecl struct {
	Name  string
	Value ast.Node
	Node  ast.Node
}

// subjectDeclaration reads a subject declaration, written as a subject member
// (`subject s : T;`, `subject = expr;`) or a subject-kind usage.
func subjectDeclaration(member ast.Node) (subjectDecl, bool) {
	switch m := member.(type) {
	case *ast.SubjectMember:
		name, _ := m.EffectiveName()
		return subjectDecl{Name: name, Value: m.BindingExpr, Node: m}, true
	case *ast.Usage:
		if m.Kind != ast.UsageSubject {
			return subjectDecl{}, false
		}
		name, _ := ast.EffectiveName(m)
		return subjectDecl{Name: name, Value: m.Value, Node: m}, true
	}
	return subjectDecl{}, false
}

// subjectParameter records link's subject among params: refining the subject an
// earlier link declared, else inserting it first, the position a subject binds by.
func (ctx *Context) subjectParameter(
	params []calcParameter, index map[string]int, aliases *map[string]string,
	link *symbols.Symbol, member ast.Node, subject subjectDecl,
) []calcParameter {
	sym := memberSymbol(declScope(link), member)
	param := calcParameter{
		Name: subject.Name, Default: subject.Value, Owner: link, IsSubject: true,
		Decl: ctx.calcMemberDeclOf(link, sym, subject.Name),
	}
	at := -1
	for i := range params {
		if params[i].IsSubject {
			at = i
			break
		}
	}
	if at < 0 {
		if seen, ok := ctx.redeclaredIndex(index, sym, subject.Name); ok {
			at = seen
		}
	}
	if at >= 0 {
		// A redeclaration binding no value keeps the inherited binding; one
		// declaring no name keeps the inherited name.
		if param.Default == nil {
			param.Default, param.Owner = params[at].Default, params[at].Owner
		}
		if param.Name == "" {
			param.Name = params[at].Name
		}
		param.Decl = param.Decl.redeclaring(params[at].Decl)
		if params[at].Name != param.Name {
			*aliases = aliasRedefined(*aliases, params[at].Name, param.Name)
			delete(index, params[at].Name)
		}
		params[at] = param
		index[param.Name] = at
		return params
	}
	if param.Name == "" {
		param.Name = "subject"
	}
	for name, i := range index {
		index[name] = i + 1
	}
	index[param.Name] = 0
	return append([]calcParameter{param}, params...)
}

// subjectParameter is the parameter the case's subject binds to, if it declares one.
func (shape *calcShape) subjectParameter() (*calcParameter, bool) {
	for i := range shape.Params {
		if shape.Params[i].IsSubject {
			return &shape.Params[i], true
		}
	}
	return nil, false
}

// enclosingSubject is the subject of the case whose body declares shape's usage,
// which a nested case binding no subject of its own takes (SysML v2 §7.21.2):
// read from the environment of the evaluation reading the usage.
func (ctx *Context) enclosingSubject(shape *calcShape, enclosing *EvalContext) (Value, bool) {
	if enclosing == nil {
		return Value{}, false
	}
	owner := enclosingBehavior(shape.Sym)
	if owner == nil || !isCalcSymbol(owner) {
		return Value{}, false
	}
	outer, err := ctx.calcShapeOf(owner)
	if err != nil {
		return Value{}, false
	}
	subject, ok := outer.subjectParameter()
	if !ok {
		return Value{}, false
	}
	return enclosing.Lookup(subject.Name)
}

// enclosingBehavior is the behavior whose body, directly or through body-local
// blocks, declares sym; nil for a member of a part or a package.
func enclosingBehavior(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil {
			return owner
		}
		if !scope.BodyLocal() {
			return nil
		}
	}
	return nil
}

// unboundSubject reports a case run with no object as its subject.
func (shape *calcShape) unboundSubject(param *calcParameter) error {
	return &UnboundSubjectError{Kind: shape.Kind, Element: shape.Name, Subject: param.Name}
}

// IsAnalysisSymbol reports whether sym declares an analysis case definition or usage.
func IsAnalysisSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		return isAnalysisDecl(sym.Decl)
	}
	return sym.Kind == symbols.SymbolAnalysisCaseDef || sym.Kind == symbols.SymbolAnalysisCaseUsage
}

// RequireAnalysisCase reports ErrNotAnAnalysis for a symbol that is not an analysis
// case definition or usage, describing what it is instead.
func (ctx *Context) RequireAnalysisCase(sym *symbols.Symbol) error {
	if sym == nil {
		return fmt.Errorf("%w: invalid symbol", ErrNotAnAnalysis)
	}
	if !IsAnalysisSymbol(sym) {
		return fmt.Errorf("%w: %s is %s, not an analysis case definition or usage",
			ErrNotAnAnalysis, ctx.qualifiedSymbolName(sym), describeDecl(sym.Decl))
	}
	return nil
}
