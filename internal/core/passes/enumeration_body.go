package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// EnumerationBodyPass reports a member an enumeration body cannot own. The
// grammar admits only enumerated values and annotating elements there
// (SysML.xtext `EnumerationBody`: `AnnotatingMember | EnumerationUsageMember`),
// so a nested definition, package, import, alias or the like is a notation error:
// the file still reads, so it gates no higher tier. An owned usage that is not an
// enumerated value is left to the variation-membership constraint, whose
// message names the fix.
type EnumerationBodyPass struct{}

func (EnumerationBodyPass) Level() PassLevel { return LevelSyntax }

func (EnumerationBodyPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if root == nil {
		return nil
	}
	return checkEnumerationBodies(root.Members)
}

// checkEnumerationBodies reports every illegal enumeration member in the
// member subtree.
func checkEnumerationBodies(members []ast.Node) []Diagnostic {
	var diags []Diagnostic
	for _, m := range members {
		switch n := unwrapType(m).(type) {
		case *ast.Package:
			diags = append(diags, checkEnumerationBodies(n.Members)...)
		case *ast.Namespace:
			diags = append(diags, checkEnumerationBodies(n.Members)...)
		case *ast.Definition:
			if n.Kind == ast.DefEnumeration {
				diags = append(diags, checkEnumerationBody(n)...)
			}
			diags = append(diags, checkEnumerationBodies(n.Members)...)
		case *ast.Usage:
			diags = append(diags, checkEnumerationBodies(n.Members)...)
		}
	}
	return diags
}

// checkEnumerationBody reports each member of def that is neither an
// enumerated value nor an annotating element.
func checkEnumerationBody(def *ast.Definition) []Diagnostic {
	var diags []Diagnostic
	for _, m := range def.Members {
		node := unwrapType(m)
		if enumerationBodyAdmits(node) {
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Notation: true,
			Span:     m.Span(),
			Message: fmt.Sprintf("enumeration definition %s cannot own %s: an enumeration body holds only enumerated values and annotations (comments, documentation, textual representations, metadata); move %s out of %s",
				enumerationName(def), enumerationMemberLabel(node), enumerationMemberLabel(node), enumerationName(def)),
			Code:   "enumeration-body-member",
			Source: "syntax",
		})
	}
	return diags
}

// enumerationBodyAdmits reports whether the grammar lets an enumeration body
// own node. A parse error is already reported, so it is admitted here.
func enumerationBodyAdmits(node ast.Node) bool {
	switch node.(type) {
	case *ast.Usage, // a non-enumerated usage is the variation-membership constraint's finding
		*ast.Comment, *ast.Documentation, *ast.TextualRepresentation, *ast.PrefixMetadata, *ast.ErrorNode:
		return true
	}
	return false
}

// enumerationName names def for a diagnostic.
func enumerationName(def *ast.Definition) string {
	if def.Ident.Name != "" {
		return "`" + def.Ident.Name + "`"
	}
	if def.Ident.ShortName != "" {
		return "`" + def.Ident.ShortName + "`"
	}
	return "this enumeration"
}

// enumerationMemberLabel describes an illegal member for a diagnostic.
func enumerationMemberLabel(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Definition:
		return labelNamed(definitionKeyword(n), n.Ident)
	case *ast.Package:
		return labelNamed("package", n.Ident)
	case *ast.Namespace:
		return labelNamed("namespace", n.Ident)
	case *ast.Alias:
		return labelNamed("alias", n.Ident)
	case *ast.Import:
		if n.IsExpose {
			return "this expose"
		}
		return "this import"
	case *ast.Dependency:
		return "this dependency"
	case *ast.FilterMember:
		return "this filter"
	}
	return "this member"
}

// definitionKeyword is the keyword phrase def was declared with.
func definitionKeyword(def *ast.Definition) string {
	kw := def.Keyword
	if kw == "" {
		kw = def.Kind.String()
	}
	if def.HasDefKeyword {
		kw += " def"
	}
	return kw
}

// labelNamed is "<kind> `<name>`", or "this <kind>" when unnamed.
func labelNamed(kind string, id ast.Identification) string {
	if id.Name != "" {
		return kind + " `" + id.Name + "`"
	}
	if id.ShortName != "" {
		return kind + " `" + id.ShortName + "`"
	}
	return "this " + kind
}
