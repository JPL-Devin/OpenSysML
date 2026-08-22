package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// CodeNonstandardNotation marks notation OpenSysML accepts that no production of
// the pinned SysML v2 grammars admits.
const CodeNonstandardNotation = "nonstandard-notation"

// CodeKerMLNotation marks KerML notation used in a SysML file, where the SysML
// grammar has no production for it. Strict mode rejects it too: the pinned
// SysML grammar admits it nowhere.
const CodeKerMLNotation = "kerml-notation"

// CodeSysMLNotation marks SysML notation used in a KerML file, which the pinned
// KerML grammar admits nowhere.
const CodeSysMLNotation = "sysml-notation"

// CodeReservedKeywordName marks a declared name spelled as a reserved keyword,
// which the parser reads and warns about and strict mode rejects.
const CodeReservedKeywordName = "reserved-keyword-name"

// NonstandardNotationPass reports the constructs OpenSysML accepts beyond the
// pinned grammars, classified in docs/reference/grammar/conformance-audit.md.
// They stay parsed in either mode — models already use them — and the mode
// decides what the finding weighs: a warning by default, an error under
// conformance.ModeStrict, which rejects the file but, being notation, gates
// no higher tier.
type NonstandardNotationPass struct{}

// Level reports the syntax level: the written notation is all it reads.
func (NonstandardNotationPass) Level() PassLevel { return LevelSyntax }

// Run walks the document for extension notation and for KerML notation in a
// SysML file.
func (NonstandardNotationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if root == nil {
		return nil
	}
	// A document of no known kind — the REPL and CLI buffer — reads as SysML,
	// the notation its prompt takes; the REPL drops the finding for a snippet it
	// loaded from a .kerml file.
	w := &notationWalker{
		sysml:       ctx.Kind != source.KindKerML,
		kind:        ctx.Kind,
		severity:    notationSeverity(ctx.Options.Conformance),
		parsedClean: !hasParseError(ctx.ParseDiagnostics),
	}
	w.walk(root.Members)
	// Under strict conformance the findings are errors, but they say how the
	// model is written, not what it means, so they gate no higher tier.
	for i := range w.diags {
		w.diags[i].Notation = true
	}
	return w.diags
}

// notationSeverity maps the conformance mode onto what a non-conforming
// construct weighs.
func notationSeverity(mode conformance.Mode) Severity {
	if mode.IsStrict() {
		return SeverityError
	}
	return SeverityWarning
}

// notationWalker accumulates the diagnostics of one document.
type notationWalker struct {
	sysml bool
	// kind is the language the document is read as, which fixes its reserved words.
	kind source.Kind
	// severity is what every finding of this walk carries; see notationSeverity.
	severity          Severity
	diags             []Diagnostic
	inRequirementBody bool
	// inActionBody records that the body being walked admits ActionBodyItem
	// members (SysML.xtext:1367).
	inActionBody bool
	// parsedClean records that the document parsed without an error, which a
	// finding needs when recovery can shape the tree it reads (see initialNode).
	parsedClean bool
}

// hasParseError reports whether the parser errored on the document.
func hasParseError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// walk reports the extension notation in a member list and descends into the
// bodies its members carry.
func (w *notationWalker) walk(members []ast.Node) {
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.Namespace:
			w.kermlNamespace(n)
			w.keywordAsName(n.Ident)
			w.walk(n.Members)
		case *ast.Package:
			w.keywordAsName(n.Ident)
			w.walk(n.Members)
		case *ast.Definition:
			w.kermlRelationships(n.Relationships)
			w.sysmlDeclaration(n, n.Keyword)
			w.keywordAsName(n.Ident)
			w.walkDeclaration(n.Members, n)
		case *ast.Usage:
			w.kermlRelationships(n.Relationships)
			w.sysmlDeclaration(n, n.Keyword)
			w.keywordAsName(n.Ident)
			w.binding(n)
			w.requirementConstraint(n)
			w.walkDeclaration(n.Members, n)
		case *ast.Import:
			w.walk(n.Body)
		case *ast.ResultMember:
			if n.Expression != nil && !isReferenceExpression(n.Expression) {
				w.extension(keywordSpan(n, "return"), "`return <expression>;`",
					"the standard spelling is a trailing expression without `return`")
			}
		case *ast.ConstraintMember:
			if n.Keyword != "" && n.Expression != nil && n.Name == "" && len(n.Body) == 0 {
				w.extension(keywordSpan(n, n.Keyword),
					fmt.Sprintf("`%s <expression>;`", n.Keyword),
					"the standard spelling is a keyword-less trailing condition")
			}
			w.walk(n.Body)
		case *ast.AssumeMember:
			if w.inRequirementBody && n.Expression != nil && !isRequirementReferenceExpression(n.Expression) &&
				n.Reference == nil && n.Name == "" && len(n.Body) == 0 && !n.HasBody {
				w.extension(keywordSpan(n, "assume"), "`assume <expression>;`",
					"the standard spelling is a keyword-less trailing condition")
			}
			w.walk(n.Body)
		case *ast.RequireMember:
			if w.inRequirementBody && n.Expression != nil && !isRequirementReferenceExpression(n.Expression) &&
				n.Reference == nil && n.Name == "" && len(n.Body) == 0 && !n.HasBody {
				w.extension(keywordSpan(n, "require"), "`require <expression>;`",
					"the standard spelling is a keyword-less trailing condition")
			}
			w.walk(n.Body)
		case *ast.SuccessionEdge:
			w.successionEdge(n)
		case *ast.StateNode:
			w.stateNode(n)
		case *ast.StateRegion:
			w.extension(keywordSpan(n, "region"), "`region <name> { … }`",
				"the standard orthogonality marker is `parallel` before a state body")
			w.walk(n.States)
		case *ast.PseudostateNode:
			w.pseudostate(n)
		case *ast.DeferMember:
			w.extension(keywordSpan(n, "defer"), "`defer <event>;`",
				"no notation states a deferred event")
		case *ast.InitialNode:
			w.initialNode(n)
			w.walkActionBody(n.Members)
		case *ast.FinalNode:
			switch {
			case n.Keyword == "final":
				w.extension(keywordSpan(n, n.Keyword), "`final` as an action node",
					"the standard spelling of the node is `done`, the library feature")
			case n.Name != "":
				w.extension(keywordSpan(n, n.Keyword), "`done <name>;`",
					"`done` is a library feature a succession names, not a keyword that declares a node")
			}
		case *ast.DecisionNode:
			if n.Keyword == "decision" {
				w.extension(keywordSpan(n, n.Keyword), "`decision` as an action node",
					"the standard spelling of the node is `decide`")
			}
			w.walkActionBody(n.Members)
		case *ast.TransitionMember:
			if n.ToSpan.Len > 0 {
				w.extension(n.ToSpan, "`transition <source> to <target>;`",
					"a transition states its ends with `first` and `then`")
			}
			w.walkActionBody(n.Effect)
			w.walkActionBody(n.Members)
		case *ast.EntryMember:
			w.walkActionBody(n.Actions)
		case *ast.DoMember:
			w.walkActionBody(n.Actions)
		case *ast.ExitMember:
			w.walkActionBody(n.Actions)
		case *ast.IfActionNode:
			for _, branch := range n.Branches() {
				w.walkActionBody(branch.Body)
			}
		case *ast.WhileLoopActionNode:
			w.walkActionBody(n.Body)
		default:
			w.walkActionBody(ast.NodeBodyMembers(n))
		}
	}
}

// walkDeclaration walks the body of a declaration under the body kind that
// declaration opens.
func (w *notationWalker) walkDeclaration(members []ast.Node, declaration ast.Node) {
	requirement, action := w.inRequirementBody, w.inActionBody
	w.inRequirementBody = isRequirementBodyDeclaration(declaration)
	w.inActionBody = admitsActionBodyItems(declaration)
	w.walk(members)
	w.inRequirementBody, w.inActionBody = requirement, action
}

// walkActionBody walks the body of an action node, which is an ActionBody
// wherever the node itself is written.
func (w *notationWalker) walkActionBody(members []ast.Node) {
	if len(members) == 0 {
		return
	}
	action := w.inActionBody
	w.inActionBody = true
	w.walk(members)
	w.inActionBody = action
}

func isRequirementBodyDeclaration(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Definition:
		switch n.Kind {
		case ast.DefRequirement:
			return true
		case ast.DefConcern, ast.DefViewpoint:
			return true
		}
	case *ast.Usage:
		switch n.Kind {
		case ast.UsageRequirement, ast.UsageSatisfy:
			return true
		case ast.UsageConcern, ast.UsageViewpoint, ast.UsageFramedConcern, ast.UsageObjective:
			return true
		}
	}
	return false
}

// admitsActionBodyItems reports whether the body a declaration opens is an
// ActionBody (SysML.xtext:1361) or one that includes ActionBodyItem: a
// CalculationBody (:1947) or a CaseBody (:2183).
func admitsActionBodyItems(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Definition:
		switch n.Kind {
		case ast.DefAction, ast.DefCalc, ast.DefConstraint, ast.DefBehavior, ast.DefPredicate:
			return true
		case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	case *ast.Usage:
		switch n.Kind {
		case ast.UsageAction, ast.UsageStep, ast.UsageCalc, ast.UsageExpr, ast.UsageConstraint:
			return true
		case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		case ast.UsageTransition, ast.UsagePredicate, ast.UsageBehavior:
			return true
		}
	}
	return false
}

func isReferenceExpression(node ast.Node) bool {
	switch node.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	default:
		return false
	}
}

func isRequirementReferenceExpression(node ast.Node) bool {
	switch node.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	default:
		return false
	}
}

// initialNode reports the `initial` spelling of the node, and a one-ended `first
// <node>;` outside an action body: InitialNodeMember is reachable from
// ActionBodyItem alone (SysML.xtext:1376), never from DefinitionBodyItem (:516).
func (w *notationWalker) initialNode(n *ast.InitialNode) {
	switch {
	case n.Keyword == "initial":
		w.extension(keywordSpan(n, n.Keyword), "`initial` as an action node",
			"the standard spelling of the node is `first`")
	// A recovered `first <source> then <target>` reads as a one-ended node, so
	// only a document that parsed cleanly is judged here.
	case !w.inActionBody && n.Successor == nil && w.parsedClean:
		w.extension(keywordSpan(n, n.Keyword), "a one-ended `first <node>;` outside an action body",
			"only an action body admits it; elsewhere a succession names both ends, `first <source> then <target>`")
	}
}

// successionEdge reports `then <source> <target>;`: a succession states two ends
// only as `first … then …` (SuccessionAsUsage, SysML.xtext:1033). An end bound by
// position is spanned outside the edge, which keeps the one-ended form silent.
func (w *notationWalker) successionEdge(n *ast.SuccessionEdge) {
	if !w.writtenEnd(n, n.Source) || !w.writtenEnd(n, n.Target) {
		return
	}
	w.extension(keywordSpan(n, "then"), "`then <source> <target>;`",
		"a succession names both ends as `first <source> then <target>`")
}

// writtenEnd reports whether end was written inside the edge that carries it.
func (w *notationWalker) writtenEnd(edge ast.Node, end *ast.QualifiedName) bool {
	if end == nil || len(end.Parts) == 0 {
		return false
	}
	sp, within := end.Span(), edge.Span()
	return sp.Offset >= within.Offset && sp.Offset < within.End()
}

// binding reports a binding whose right side is an expression: `bind` relates two
// ConnectorEndMembers (SysML.xtext:1020), so `bind a = b;` is legal and
// `bind a = b * 2;` is ours.
func (w *notationWalker) binding(n *ast.Usage) {
	if n.Kind != ast.UsageBinding || n.Value == nil || isReferenceExpression(n.Value) {
		return
	}
	w.extension(n.Value.Span(), "`bind <feature> = <expression>;`",
		"a binding relates two features, so bind the expression's result feature instead")
}

// requirementConstraint reports an `assume`/`require` member outside a requirement
// body, where RequirementConstraintMember alone admits it (SysML.xtext:2039).
func (w *notationWalker) requirementConstraint(n *ast.Usage) {
	if w.inRequirementBody || n.Kind != ast.UsageConstraint {
		return
	}
	keyword := n.PrefixKeyword
	if keyword != "assume" && keyword != "require" {
		keyword = n.Keyword
	}
	if keyword != "assume" && keyword != "require" {
		return
	}
	w.extension(keywordSpan(n, keyword), fmt.Sprintf("`%s` outside a requirement body", keyword),
		"only a requirement, concern, viewpoint or objective body admits it")
}

// stateNode reports the `initial`/`final` state markers, then descends.
func (w *notationWalker) stateNode(n *ast.StateNode) {
	switch {
	case n.IsInitial:
		w.extension(keywordSpan(n, "initial"), "`initial <state>;`",
			"the standard way to mark the state a machine starts in is `entry; then <state>;`")
	case n.IsFinal:
		w.extension(keywordSpan(n, "final"), "`final <state>;`",
			"a final state is reached by a transition, and is written `state <name>;`")
	}
	w.walkActionBody(n.Entry)
	w.walkActionBody(n.Do)
	w.walkActionBody(n.Exit)
	w.walk(n.Defer)
	w.walk(n.Substates)
	for _, region := range n.Regions {
		w.walk([]ast.Node{region})
	}
}

// pseudostate reports the pseudostates that are ours. `fork` and `join` are not
// among them: both are action node literals a state body admits.
func (w *notationWalker) pseudostate(n *ast.PseudostateNode) {
	switch n.Kind {
	case ast.PseudostateFork, ast.PseudostateJoin:
		return
	}
	w.extension(keywordSpan(n, n.Keyword), fmt.Sprintf("`%s <name>;`", n.Keyword),
		"the grammars define no pseudostate notation")
}

// kermlNamespace reports a `namespace` declaration in a SysML file, whose root
// production admits package members only.
func (w *notationWalker) kermlNamespace(n *ast.Namespace) {
	if !w.sysml {
		return
	}
	w.diags = append(w.diags, Diagnostic{
		Severity: w.severity,
		Span:     keywordSpan(n, "namespace"),
		Message: "`namespace` is KerML notation: the SysML v2 grammar has no namespace declaration, " +
			"so write `package` here or move the declaration to a .kerml file",
		Code:   CodeKerMLNotation,
		Source: "syntax",
	})
}

// kermlRelationships reports a `featured by` clause in a SysML file: the
// featuring relationship is KerML.xtext:569 only, absent from SysML.xtext.
func (w *notationWalker) kermlRelationships(rels []*ast.Relationship) {
	if !w.sysml {
		return
	}
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelFeaturedBy {
			continue
		}
		w.diags = append(w.diags, Diagnostic{
			Severity: w.severity,
			Span:     rel.Span(),
			Message: "`featured by` is KerML notation: the SysML v2 grammar has no featuring clause, " +
				"so move the declaration to a .kerml file",
			Code:   CodeKerMLNotation,
			Source: "syntax",
		})
	}
}

// kermlDeclarationKeywords are the definition and usage keywords the pinned
// KerML grammar spells; a kind keyword outside the set is SysML-only.
var kermlDeclarationKeywords = map[string]bool{
	"assoc": true, "behavior": true, "binding": true, "bool": true,
	"class": true, "classifier": true, "connector": true, "datatype": true,
	"dependency": true, "expr": true, "feature": true, "flow": true,
	"function": true, "interaction": true, "inv": true, "metaclass": true,
	"metadata": true, "multiplicity": true, "predicate": true, "step": true,
	"struct": true, "succession": true, "type": true,
}

// sysmlDeclaration reports a kind keyword in a KerML file that no KerML
// production spells, like the `part def` of k02 (KerML.xtext RootNamespace).
func (w *notationWalker) sysmlDeclaration(n ast.Node, keyword string) {
	if w.sysml || keyword == "" || kermlDeclarationKeywords[keyword] {
		return
	}
	w.diags = append(w.diags, Diagnostic{
		Severity: w.severity,
		Span:     keywordSpan(n, keyword),
		Message: fmt.Sprintf("`%s` is SysML notation: the KerML grammar has no such declaration keyword, "+
			"so move the declaration to a .sysml file", keyword),
		Code:   CodeSysMLNotation,
		Source: "syntax",
	})
}

// keywordAsName rejects a declared name spelled as a reserved keyword, which the
// ID terminal excludes. Only strict mode reports it: the parser already warns and
// keeps the recovery an editor needs (g15, docs/project/pilot-rejection.md).
func (w *notationWalker) keywordAsName(id ast.Identification) {
	if w.severity != SeverityError || id.Name == "" || id.NameSpan.Len != len(id.Name) {
		return
	}
	if !lexer.IsKeywordIn(id.Name, w.kind) {
		return
	}
	w.diags = append(w.diags, Diagnostic{
		Severity: w.severity,
		Span:     id.NameSpan,
		Message: fmt.Sprintf("%q is a reserved keyword, not a name the ID terminal admits; "+
			"write '%s' to use it as a name", id.Name, id.Name),
		Code:   CodeReservedKeywordName,
		Source: "syntax",
	})
}

// extension reports one construct as an OpenSysML extension.
func (w *notationWalker) extension(span source.Span, construct, standard string) {
	w.diags = append(w.diags, Diagnostic{
		Severity: w.severity,
		Span:     span,
		Message: fmt.Sprintf("%s is an OpenSysML extension with no SysML v2 production: %s",
			construct, standard),
		Code:   CodeNonstandardNotation,
		Source: "syntax",
	})
}

// keywordSpan spans the notation that opens a node, so the diagnostic points at
// the word rather than the whole declaration.
func keywordSpan(n ast.Node, keyword string) source.Span {
	sp := n.Span()
	if sp.Len > len(keyword) {
		sp.Len = len(keyword)
	}
	return sp
}
