package passes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// IdentityMetadataPass validates the IdentityMetadata annotations of a
// document: id shape, the enclosing ProjectRef an ElementId resolves against,
// and effective-id uniqueness over the whole generated id space of each
// project scope (element ids, `_om` membership ids, `_p` expression-node ids).
type IdentityMetadataPass struct{}

func (IdentityMetadataPass) Level() PassLevel { return LevelConstraint }

func (IdentityMetadataPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	// A project scope may span workspace documents, so uniqueness is judged
	// over all of them; each document only reports its own elements.
	roots := []*symbols.Scope{rootScope}
	for _, doc := range ctx.Index.WorkspaceDocuments() {
		if doc == name {
			continue
		}
		if r := ctx.Index.DocumentRoot(doc); r != nil {
			roots = append(roots, r)
		}
	}
	table := identity.Build(ctx.Model(), ctx.Resolver(), roots...)
	c := &identityChecker{table: table, docRoot: rootScope}
	c.check()
	return c.diags
}

type identityChecker struct {
	table   *identity.Table
	docRoot *symbols.Scope
	diags   []Diagnostic
}

// inDocument reports whether the info's symbol is declared in the document
// under validation, so each document reports only its own elements.
func (c *identityChecker) inDocument(info *identity.Info) bool {
	for sc := info.Symbol.OwnerScope; sc != nil; sc = sc.Parent() {
		if sc == c.docRoot {
			return true
		}
	}
	return false
}

func (c *identityChecker) check() {
	scopes := make(map[string][]*identity.Info)
	var keys []string
	for _, sym := range c.table.Symbols() {
		info, ok := c.table.Info(sym)
		if !ok {
			continue
		}
		if info.Annotated && c.inDocument(info) {
			c.checkShape(info)
			if info.Scope == nil {
				c.errorf(info.AnnotationSpan, "identity-unscoped-id",
					"ElementId on %s has no enclosing ProjectRef to resolve against", info.FQN)
			}
		}
		key := scopeKey(info)
		if _, seen := scopes[key]; !seen {
			keys = append(keys, key)
		}
		scopes[key] = append(scopes[key], info)
	}
	for _, key := range keys {
		c.checkScope(scopes[key])
	}
}

// scopeKey groups elements by the project their scope names — org plus
// projectId — with the absent scope as a group of its own.
func scopeKey(info *identity.Info) string {
	if info.Scope == nil {
		return ""
	}
	return "bound\x00" + info.Scope.Key()
}

// checkShape reports the first byte of a declared id outside [a-zA-Z0-9_-],
// including the empty id, which has no legal byte at all.
func (c *identityChecker) checkShape(info *identity.Info) {
	if info.Declared && info.DeclaredID == "" {
		c.errorf(info.AnnotationSpan, "identity-id-shape",
			"element id of %s is empty; an id needs at least one byte of [a-zA-Z0-9_-]", info.FQN)
		return
	}
	for i := 0; i < len(info.DeclaredID); i++ {
		b := info.DeclaredID[i]
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-' {
			continue
		}
		c.errorf(info.AnnotationSpan, "identity-id-shape",
			"element id %q of %s: byte 0x%02x (%q) at offset %d is outside [a-zA-Z0-9_-]",
			info.DeclaredID, info.FQN, b, string(rune(b)), i)
		return
	}
}

// checkScope validates the generated id space of one project scope: duplicate
// effective ids, and declared ids that land on another element's membership
// (`…_om`) or expression-node (`…_p…`) id.
func (c *identityChecker) checkScope(infos []*identity.Info) {
	byID := make(map[string][]*identity.Info)
	for _, info := range infos {
		byID[info.EffectiveID] = append(byID[info.EffectiveID], info)
	}
	for _, group := range byID {
		// Distinct qualified names never derive one id, so a group of derived
		// ids is one name seen twice, not an identity conflict.
		if len(group) < 2 || !anyAnnotated(group) {
			continue
		}
		names := make([]string, 0, len(group))
		for _, info := range group {
			names = append(names, info.FQN)
		}
		sort.Strings(names)
		for _, info := range group {
			if !c.inDocument(info) {
				continue
			}
			c.errorf(c.spanOf(info), "identity-duplicate-id",
				"duplicate element id %q in one project scope: %s",
				info.EffectiveID, strings.Join(names, " and "))
		}
	}
	for _, info := range infos {
		if info.DeclaredID == "" {
			continue
		}
		if base, ok := strings.CutSuffix(info.DeclaredID, "_om"); ok {
			c.reportDerivedCollision(info, byID[base], "the owning-membership id")
		}
		for i := strings.Index(info.DeclaredID, "_p"); i >= 0; {
			c.reportDerivedCollision(info, byID[info.DeclaredID[:i]], "an expression-node id")
			next := strings.Index(info.DeclaredID[i+1:], "_p")
			if next < 0 {
				break
			}
			i += 1 + next
		}
	}
}

// anyAnnotated reports whether any element of the group declares its id.
func anyAnnotated(group []*identity.Info) bool {
	for _, info := range group {
		if info.Annotated {
			return true
		}
	}
	return false
}

// reportDerivedCollision errors on both elements when a declared id lands in
// the derived id space another element generates.
func (c *identityChecker) reportDerivedCollision(info *identity.Info, owners []*identity.Info, space string) {
	for _, owner := range owners {
		if owner == info {
			continue
		}
		if c.inDocument(info) {
			c.errorf(info.AnnotationSpan, "identity-duplicate-id",
				"element id %q of %s collides with %s of %s",
				info.DeclaredID, info.FQN, space, owner.FQN)
		}
		if c.inDocument(owner) {
			c.errorf(c.spanOf(owner), "identity-duplicate-id",
				"%s of %s collides with element id %q of %s",
				space, owner.FQN, info.DeclaredID, info.FQN)
		}
	}
}

// spanOf locates an element for a diagnostic: its ElementId annotation when it
// carries one, its declaration otherwise.
func (c *identityChecker) spanOf(info *identity.Info) source.Span {
	if info.Annotated {
		return info.AnnotationSpan
	}
	return info.Symbol.DeclSpan
}

func (c *identityChecker) errorf(span source.Span, code, format string, args ...any) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     code,
		Source:   "constraint",
	})
}
