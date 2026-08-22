package parser

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// definitionKindKeywords maps a single kind keyword to its DefinitionKind.
// The two-word `use case` is handled separately in parseDefUsage.
var definitionKindKeywords = map[string]ast.DefinitionKind{
	"part":      ast.DefPart,
	"attribute": ast.DefAttribute,
	"datatype":  ast.DefAttribute,
	"feature":   ast.DefAttribute,
	// Tier A.
	"item":       ast.DefItem,
	"occurrence": ast.DefOccurrence,
	"individual": ast.DefIndividual,
	"metaclass":  ast.DefMetaclass,
	"metadata":   ast.DefMetadata,
	"enum":       ast.DefEnumeration,
	"view":       ast.DefView,
	"viewpoint":  ast.DefViewpoint,
	"rendering":  ast.DefRendering,
	"concern":    ast.DefConcern,
	// Tier B.
	"connection": ast.DefConnection,
	"flow":       ast.DefFlow,
	"message":    ast.DefFlow, // message is synonym for flow
	"port":       ast.DefPort,
	"interface":  ast.DefInterface,
	"allocation": ast.DefAllocation,
	"allocate":   ast.DefAllocation, // Short form for allocation def
	"binding":    ast.DefBinding,
	// Tier C.
	"action":       ast.DefAction,
	"state":        ast.DefState,
	"calc":         ast.DefCalc,
	"function":     ast.DefCalc, // synonym for calc
	"constraint":   ast.DefConstraint,
	"requirement":  ast.DefRequirement,
	"case":         ast.DefCase,
	"analysis":     ast.DefAnalysisCase,
	"verification": ast.DefVerificationCase,
	// KerML structural.
	"behavior":      ast.DefBehavior,
	"assoc":         ast.DefAssoc,
	"struct":        ast.DefStruct,
	"class":         ast.DefClass,
	"classifier":    ast.DefClass, // synonym for class
	"subclassifier": ast.DefClass, // subtyping context synonym for classifier
	"predicate":     ast.DefPredicate,
	"bool":          ast.DefBool,
}

// compoundDefKinds are the two-keyword kinds, where the second keyword names
// the kind rather than the declaration. Every other pair of kind keywords is a
// kind followed by a name. The two-word `use case` is handled separately in
// parseDefUsage.
var compoundDefKinds = map[[2]string]bool{
	{"assoc", "struct"}:      true,
	{"analysis", "case"}:     true,
	{"verification", "case"}: true,
}

// kindPrefixKeywords are keywords that qualify a following kind keyword rather
// than naming the declaration, even when no name follows the kind:
// `assert constraint { ... }` is an asserted anonymous constraint, not a
// declaration named `constraint`.
var kindPrefixKeywords = map[string]bool{
	"assert":  true,
	"assume":  true,
	"require": true,
	"var":     true,
}

// notKindPrefixKeywords are keywords that are never a prefix of a following
// kind keyword, because they are a kind of their own with dedicated parsing
// (`satisfy requirement r by x`) or a direction that the kind carries
// (`in item x`). A following kind keyword belongs to their own declaration.
var notKindPrefixKeywords = map[string]bool{
	"subject": true, "objective": true, "succession": true, "inv": true,
	"connector": true, "satisfy": true, "verify": true, "step": true,
	"expr": true, "interaction": true, "stakeholder": true, "frame": true,
	"actor": true, "expose": true, "render": true, "perform": true,
	"include": true, "exhibit": true, "variant": true, "event": true,
	"timeslice": true, "snapshot": true, "transition": true, "bind": true,
	"binding": true, "member": true,
	// `not satisfy r by p` negates the satisfaction; the prefix path would drop
	// the `not` (SysML.xtext:2118, SatisfyRequirementUsage isNegated).
	"not": true,
	// `individual part p` keeps the modifier; the prefix path would drop it.
	"individual": true,
	"in":         true, "out": true, "inout": true,
}

// usageKindKeywords maps a single kind keyword to its UsageKind.
var usageKindKeywords = map[string]ast.UsageKind{
	"part":      ast.UsagePart,
	"attribute": ast.UsageAttribute,
	"datatype":  ast.UsageAttribute,
	"feature":   ast.UsageAttribute,
	// Tier A.
	"item":       ast.UsageItem,
	"occurrence": ast.UsageOccurrence,
	"event":      ast.UsageOccurrence, // event creates occurrence usage (event-driven)
	"individual": ast.UsageIndividual,
	"snapshot":   ast.UsageOccurrence, // snapshot occurrence usage
	"timeslice":  ast.UsageOccurrence, // timeslice occurrence usage (temporal slice)
	"metadata":   ast.UsageMetadata,
	"enum":       ast.UsageEnumeration,
	"view":       ast.UsageView,
	"viewpoint":  ast.UsageViewpoint,
	"rendering":  ast.UsageRendering,
	"concern":    ast.UsageConcern,
	// Tier B.
	"connection":  ast.UsageConnection,
	"connector":   ast.UsageConnector,
	"succession":  ast.UsageSuccession,
	"flow":        ast.UsageFlow,
	"message":     ast.UsageFlow, // message is synonym for flow
	"port":        ast.UsagePort,
	"interface":   ast.UsageInterface,
	"interaction": ast.UsageInteraction,
	"allocation":  ast.UsageAllocation,
	"allocate":    ast.UsageAllocation, // Short form for allocation usage
	"binding":     ast.UsageBinding,
	"actor":       ast.UsageActor,         // actor of a requirement, use case or viewpoint
	"render":      ast.UsageViewRendering, // rendering a view body names
	"bind":        ast.UsageBinding,       // shorthand for binding
	// Tier C.
	"action":       ast.UsageAction,
	"perform":      ast.UsageAction, // perform keyword creates action usage
	"state":        ast.UsageState,
	"exhibit":      ast.UsageState, // exhibit references state usage (state exhibition)
	"transition":   ast.UsageTransition,
	"step":         ast.UsageStep,
	"calc":         ast.UsageCalc,
	"expr":         ast.UsageExpr, // expression parameter (lambda/closure)
	"function":     ast.UsageCalc, // synonym for calc
	"constraint":   ast.UsageConstraint,
	"inv":          ast.UsageConstraint, // synonym for constraint (invariant)
	"require":      ast.UsageConstraint, // synonym for constraint (required condition)
	"assert":       ast.UsageConstraint, // assert creates constraint usage (assertion)
	"assume":       ast.UsageConstraint, // assume creates constraint usage (assumption)
	"requirement":  ast.UsageRequirement,
	"satisfy":      ast.UsageSatisfy,
	"verify":       ast.UsageSatisfy, // verify is alias for satisfy
	"include":      ast.UsageUseCase, // include creates use case usage with includes relationship
	"subject":      ast.UsageSubject,
	"objective":    ast.UsageObjective,
	"stakeholder":  ast.UsageStakeholder,
	"frame":        ast.UsageFramedConcern,
	"case":         ast.UsageCase,
	"analysis":     ast.UsageAnalysisCase,
	"verification": ast.UsageVerificationCase,
	"variant":      ast.UsagePart, // variant keyword creates variant membership
	// KerML structural.
	"behavior":  ast.UsageBehavior,
	"assoc":     ast.UsageAssoc,
	"struct":    ast.UsageStruct,
	"class":     ast.UsageClass,
	"predicate": ast.UsagePredicate,
	"bool":      ast.UsageBool,
}

var featureModifierKeywords = map[string]bool{
	"abstract":   true,
	"variation":  true,
	"ref":        true,
	"end":        true,
	"constant":   true,
	"const":      true, // KerML spelling (KerML.xtext BasicFeaturePrefix)
	"event":      true, // event-driven occurrence modifier
	"individual": true, // individual occurrence/part modifier
	"snapshot":   true, // snapshot occurrence/part modifier
	"in":         true,
	"out":        true,
	"inout":      true,
	"composite":  true,
	"portion":    true,
	"derived":    true,
	"ordered":    true,
	"nonunique":  true,
	"public":     true,
	"protected":  true,
	"private":    true,
	"readonly":   true,
}

// relationshipKeywords maps a spelled-out relationship keyword to its kind.
var relationshipKeywords = map[string]ast.RelationshipKind{
	"specializes": ast.RelSpecializes,
	"subsets":     ast.RelSubsets,
	"redefines":   ast.RelRedefines,
	"references":  ast.RelReferences,
	"crosses":     ast.RelCrosses,
	"intersects":  ast.RelIntersects,
	"disjoint":    ast.RelDisjoint, // followed by 'from' keyword
	"unions":      ast.RelUnions,
	"differences": ast.RelDifferences,
	"chains":      ast.RelChains,
}

type featureMods struct {
	isAbstract        bool
	isVariation       bool
	isVariant         bool
	isReference       bool
	isVariable        bool
	isEnd             bool
	isChain           bool
	isConstant        bool
	isEvent           bool            // event modifier for occurrences
	isIndividual      bool            // individual modifier for individuals/snapshots
	portion           ast.PortionKind // 'snapshot' / 'timeslice' portion prefix
	isNegated         bool            // `not` of `assert not <kind>`: the conditions are asserted to be false
	prefixKeyword     string          // keyword qualifying the kind that follows it: the `assert` of `assert constraint c`
	visibility        ast.Visibility
	direction         ast.FeatureDirection
	isComposite       bool
	isDerived         bool
	isReadonly        bool
	isOrdered         bool
	isNonunique       bool
	earlyMultiplicity *ast.Multiplicity // for "end [mult] ref ..." syntax
}

// applyFeatureMods transfers modifiers that were consumed before a declaration's
// kind keyword onto the declaration itself, as in `ref part a : V`. Only
// modifiers that were present are applied, so a flag the declaration parsed for
// itself is never cleared.
func applyFeatureMods(decl ast.Node, mods featureMods) {
	switch d := decl.(type) {
	case *ast.Usage:
		if mods.isAbstract {
			d.IsAbstract = true
		}
		if mods.isVariation {
			d.IsVariation = true
		}
		if mods.isVariant {
			d.IsVariant = true
		}
		if mods.isReference {
			d.IsReference = true
		}
		if mods.isVariable {
			d.IsVariable = true
		}
		if mods.isEnd {
			d.IsEnd = true
		}
		if mods.isChain {
			d.IsChain = true
		}
		if mods.isConstant {
			d.IsConstant = true
		}
		if mods.isEvent {
			d.IsEvent = true
		}
		if mods.isIndividual {
			d.IsIndividual = true
		}
		if mods.portion != ast.PortionNone {
			d.Portion = mods.portion
		}
		if mods.isNegated {
			d.IsNegated = true
		}
		if mods.isComposite {
			d.IsComposite = true
		}
		if mods.isDerived {
			d.IsDerived = true
		}
		if mods.isOrdered {
			d.IsOrdered = true
		}
		if mods.isNonunique {
			d.IsNonunique = true
		}
		if mods.direction != ast.DirNone {
			d.Direction = mods.direction
		}
		if mods.visibility != ast.VisibilityDefault {
			d.Visibility = mods.visibility
		}
		if mods.earlyMultiplicity != nil && d.Multiplicity == nil {
			d.Multiplicity = mods.earlyMultiplicity
		}
		if mods.prefixKeyword != "" && d.PrefixKeyword == "" {
			d.PrefixKeyword = mods.prefixKeyword
		}
	case *ast.Definition:
		if mods.isAbstract {
			d.IsAbstract = true
		}
		if mods.isVariation {
			d.IsVariation = true
		}
		if mods.isConstant {
			d.IsConstant = true
		}
		if mods.isEvent {
			d.IsEvent = true
		}
		if mods.visibility != ast.VisibilityDefault {
			d.Visibility = mods.visibility
		}
	}
}

// varPrefixWord marks a variable feature (KerML.xtext BasicFeaturePrefix,
// `isVariable ?= 'var'`). That is the only position where it is a keyword, so it
// is matched contextually like `point` and names a feature everywhere else.
const varPrefixWord = "var"

// atVarWord reports whether the cursor is at the word `var`.
func (p *Parser) atVarWord() bool {
	t := p.peek()
	return t.Kind == lexer.Identifier && p.src.Text(t.Span) == varPrefixWord
}

// atVarPrefix reports whether the cursor is at the `var` prefix of a declaration
// rather than at a feature named `var`.
func (p *Parser) atVarPrefix() bool {
	if !p.atVarWord() {
		return false
	}
	next := p.peekN(1)
	return p.isKindKeyword(next) ||
		(next.Kind == lexer.Keyword && featureModifierKeywords[next.KeywordID])
}

// atVarDeclaration reports whether the cursor is at a `var`-prefixed
// declaration, whose kind keyword may be left out (`var x : Integer;`). A
// following name cannot continue an expression, so `var` there is the prefix.
func (p *Parser) atVarDeclaration() bool {
	if !p.atVarWord() {
		return false
	}
	next := p.peekN(1).Kind
	return p.isKindKeyword(p.peekN(1)) || next == lexer.Identifier || next == lexer.UnrestrictedName
}

// kindPrefixWord returns the word at the cursor that may qualify a following
// kind keyword, or "" when the cursor is at no such word.
func (p *Parser) kindPrefixWord() string {
	if p.at(lexer.Keyword) {
		return p.peek().KeywordID
	}
	if p.atVarPrefix() {
		return varPrefixWord
	}
	return ""
}

// atKindPrefix reports whether the current keyword qualifies the kind keyword
// after it instead of being the kind itself, as in `var feature x` or
// `item part Shape`. When it does not, the second keyword names the declaration
// (`action flow { ... }` is an action named `flow`).
func (p *Parser) atKindPrefix() bool {
	kw := p.kindPrefixWord()
	if kw == "" || notKindPrefixKeywords[kw] {
		return false
	}
	// A feature modifier qualifies the declaration itself (`variation part v`),
	// so it is parsed as a modifier rather than dropped as a kind prefix.
	if featureModifierKeywords[kw] {
		return false
	}
	if kw == varPrefixWord && p.peekN(1).Kind == lexer.Keyword &&
		featureModifierKeywords[p.peekN(1).KeywordID] {
		return true
	}
	if !p.isKindKeyword(p.peekN(1)) {
		return false
	}
	return kindPrefixKeywords[kw] || !namesDeclaration(p.peekN(2))
}

// atSecondaryKind reports whether the current kind keyword belongs to the kind
// of the declaration whose first kind keyword was already consumed, rather than
// being that declaration's name.
func (p *Parser) atSecondaryKind(firstKeyword string) bool {
	if notKindPrefixKeywords[firstKeyword] || !p.isKindKeyword(p.peek()) {
		return false
	}
	if compoundDefKinds[[2]string{firstKeyword, p.peek().KeywordID}] || kindPrefixKeywords[firstKeyword] {
		return true
	}
	return !namesDeclaration(p.peekN(1))
}

// atPortionedKind reports whether the current token is the kind keyword of the
// usage a portion prefix portions (`timeslice item : Cargo`). A kind keyword
// there is always the kind, never the portion's name, since the portion keyword
// itself is the only kind a bare portion usage declares.
func (p *Parser) atPortionedKind() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	_, ok := p.usageKind(p.peek().KeywordID)
	return ok
}

// isKindKeyword reports whether the token is a def or usage kind keyword.
func (p *Parser) isKindKeyword(t lexer.Token) bool {
	if t.Kind != lexer.Keyword {
		return false
	}
	_, isDef := p.definitionKind(t.KeywordID)
	_, isUsage := p.usageKind(t.KeywordID)
	return isDef || isUsage
}

// declarationKindKeyword reports whether parseDefUsage reads this token as the kind
// of the declaration rather than as its name: `frame` and `render` name the
// declaration unless they introduce their member form.
func (p *Parser) declarationKindKeyword(t lexer.Token) bool {
	if !p.isKindKeyword(t) {
		return false
	}
	if kw := t.KeywordID; kw == "frame" || kw == "render" {
		return p.atMemberKeywordUsedAsKeyword(kw)
	}
	return true
}

// namesDeclaration reports whether a kind keyword followed by this token is the
// name of the declaration rather than its kind. A declaration that ends there
// (`;`), opens a body (`{`) or is typed (`:`) has nothing else to take its name
// from, so the kind keyword before it is the name; anything else — a name, a
// `def`, a redefinition — belongs to a declaration of that kind.
func namesDeclaration(t lexer.Token) bool {
	switch t.Kind {
	case lexer.Semicolon, lexer.LBrace, lexer.Colon:
		return true
	}
	return false
}

// atAssertedReference reports whether an `assert` names an existing constraint,
// as against stating a condition of its own: the name is the whole declaration,
// so only a body or a terminator may follow it.
func (p *Parser) atAssertedReference() bool {
	if p.isKindKeyword(p.peek()) || !p.namesReference(0) {
		return false
	}
	for i := 1; ; i += 2 {
		switch sep := p.peekN(i).Kind; sep {
		case lexer.Dot, lexer.ColonColon:
			if next := p.peekN(i + 1); next.Kind != lexer.Identifier &&
				next.Kind != lexer.UnrestrictedName && next.Kind != lexer.Keyword {
				return false
			}
		case lexer.Semicolon, lexer.LBrace, lexer.LBracket:
			return true
		default:
			return false
		}
	}
}

// namesReference reports whether the token at n can name a referenced usage —
// `assert c;`, `assert not c;` — rather than beginning an expression.
func (p *Parser) namesReference(n int) bool {
	t := p.peekN(n)
	switch t.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return true
	case lexer.Keyword:
		return !p.reservedWord(t.KeywordID)
	}
	return false
}

// atFeatureSpecialization reports whether the current token begins a feature
// specialization — typing, subsetting, reference, crossing or redefinition
// (SysML.xtext FeatureSpecialization). Both spellings of each clause answer
// here (`:`/`defined by`, `:>`/`subsets`, `::>`/`references`, `=>`/`crosses`,
// `:>>`/`redefines`), so neither can be read differently from the other.
// `specializes` is excluded: it relates two types (SubclassificationPart).
func (p *Parser) atFeatureSpecialization() bool {
	t := p.peek()
	switch t.Kind {
	case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt:
		return true
	case lexer.Keyword:
		switch t.KeywordID {
		case "subsets", "references", "crosses", "redefines":
			return true
		case "defined", "typed":
			n := p.peekN(1)
			return n.Kind == lexer.Keyword && n.KeywordID == "by"
		}
	}
	return false
}

// beginsDeclarationTail reports whether the token after a name continues a
// keyword-less usage declaration (SysML.xtext DefaultReferenceUsage): a
// specialization or a feature value, e.g. `T1 = 10.0;`, `x :> y = e;`.
func beginsDeclarationTail(t, t2 lexer.Token) bool {
	switch t.Kind {
	case lexer.Eq, lexer.ColonEq, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt:
		return true
	case lexer.Colon:
		// A typing (`kpl : DerivedUnit = km / L;`) names its type next.
		return t2.Kind == lexer.Identifier || t2.Kind == lexer.UnrestrictedName
	case lexer.Keyword:
		switch t.KeywordID {
		case "subsets", "references", "crosses", "redefines":
			return true
		case "defined", "typed":
			return t2.Kind == lexer.Keyword && t2.KeywordID == "by"
		}
	}
	return false
}

// keywordlessFeatureAt reports whether the tokens from offset off declare a
// feature with no kind keyword (KerML.xtext Feature over BasicFeaturePrefix
// FeatureDeclaration): a name followed by a specialization, a multiplicity, a
// body, or nothing at all.
func (p *Parser) keywordlessFeatureAt(off int) bool {
	t := p.peekN(off)
	if t.Kind != lexer.Identifier && t.Kind != lexer.UnrestrictedName {
		return false
	}
	// KerML reserves `var`, so it prefixes a declaration and never names one.
	if t.Kind == lexer.Identifier && p.src.Kind() == source.KindKerML &&
		p.src.Text(t.Span) == varPrefixWord {
		return false
	}
	switch next := p.peekN(off + 1); next.Kind {
	case lexer.LBracket, lexer.Semicolon, lexer.LBrace:
		return true
	default:
		return beginsDeclarationTail(next, p.peekN(off+2))
	}
}

// atKindlessFeatureTyping reports whether a name is declared with no kind
// keyword and states a specialization or multiplicity (`mass : MassValue;`),
// the unambiguous half of a keyword-less declaration.
func (p *Parser) atKindlessFeatureTyping() bool {
	if t := p.peek(); t.Kind != lexer.Identifier && t.Kind != lexer.UnrestrictedName {
		return false
	}
	next := p.peekN(1)
	if next.Kind == lexer.LBracket {
		return true
	}
	if next.Kind == lexer.Eq || next.Kind == lexer.ColonEq || next.Kind == lexer.EqGt {
		return false
	}
	return beginsDeclarationTail(next, p.peekN(2))
}

// atKeywordlessFeature reports whether the cursor is at such a feature, either
// directly or behind a `var` prefix (`var p : Real;`).
func (p *Parser) atKeywordlessFeature() bool {
	if p.atVarPrefixedFeature() {
		return true
	}
	return p.keywordlessFeatureAt(0)
}

// atVarPrefixedFeature reports whether `var` prefixes a keyword-less feature
// rather than naming one. `var` is a prefix only in KerML (KerML.xtext:520).
func (p *Parser) atVarPrefixedFeature() bool {
	return p.src.Kind() == source.KindKerML && p.atVarWord() && p.keywordlessFeatureAt(1)
}

// atTextualRepresentationStart matches the KerML TextualRepresentation head
// (`rep` Name? `language` String), including its language-only spelling.
func (p *Parser) atTextualRepresentationStart() bool {
	if p.atKeyword("rep") {
		next := p.peekN(1)
		return next.Kind == lexer.Identifier ||
			next.Kind == lexer.UnrestrictedName ||
			(next.Kind == lexer.Keyword && next.KeywordID == "language")
	}
	return p.atKeyword("language") && p.peekN(1).Kind == lexer.String
}

// atDefUsageStart reports whether the current token begins a def/usage
// declaration: a feature specialization stated in place of a name, a
// conjugation, a feature-modifier keyword or a kind keyword.
func (p *Parser) atDefUsageStart() bool {
	t := p.peek()
	if p.atFeatureSpecialization() || t.Kind == lexer.Tilde {
		return true
	}
	if p.atTextualRepresentationStart() {
		return true
	}
	if p.atVarPrefixedFeature() {
		return true
	}
	if t.Kind != lexer.Keyword {
		return false
	}
	if featureModifierKeywords[t.KeywordID] {
		return true
	}
	if t.KeywordID == "use" {
		return p.atUseCase()
	}
	// `connect` starts an anonymous connection usage without being a kind
	// keyword: `connection c connect a to b` states it after the kind.
	if t.KeywordID == "connect" {
		return true
	}
	if t.KeywordID == "not" {
		return p.atNegatedSatisfy()
	}
	_, isDef := p.definitionKind(t.KeywordID)
	_, isUsage := p.usageKind(t.KeywordID)
	return isDef || isUsage
}

// atNegatedSatisfy reports whether the cursor is at `not satisfy`, the
// satisfaction negated without an `assert` before it. `verify` has no negated
// form (SysML.xtext:2118 is the only production spelling `isNegated`).
func (p *Parser) atNegatedSatisfy() bool {
	if !p.atKeyword("not") {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "satisfy"
}

// atUseCase reports whether the current token is `use` immediately followed by
// `case` (the two-word use-case kind keyword).
func (p *Parser) atUseCase() bool {
	if !p.atKeyword("use") {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "case"
}

// isModifierOrKindKeyword checks if keyword is a modifier or def/usage kind keyword
func (p *Parser) isModifierOrKindKeyword(kw string) bool {
	_, isMod := featureModifierKeywords[kw]
	_, isDef := p.definitionKind(kw)
	_, isUsage := p.usageKind(kw)
	return isMod || isDef || isUsage
}

// atDirectionKeyword reports whether the cursor is at a feature direction.
func (p *Parser) atDirectionKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	switch p.peek().KeywordID {
	case "in", "out", "inout":
		return true
	}
	return false
}

func (p *Parser) parseFeatureModifiers() featureMods {
	var m featureMods
	for {
		t := p.peek()
		// Handle identifier "chain" as contextual modifier ONLY if followed by name/keyword
		if t.Kind == lexer.Identifier && p.src.Text(t.Span) == "chain" {
			next := p.peekN(1)
			// "chain" is modifier if next token is identifier, keyword, or :: (qualified name)
			isModifier := next.Kind == lexer.Identifier || next.Kind == lexer.Keyword || next.Kind == lexer.ColonColon
			if isModifier {
				m.isChain = true
				p.advance()
				continue
			}
			// Otherwise "chain" is the declaration name itself - stop parsing modifiers
			return m
		}
		if t.Kind == lexer.Identifier && p.src.Text(t.Span) == varPrefixWord {
			next := p.peekN(1)
			isModifier := p.isKindKeyword(next) ||
				(next.Kind == lexer.Keyword && featureModifierKeywords[next.KeywordID]) ||
				p.atVarPrefixedFeature()
			if isModifier {
				m.isVariable = true
				m.prefixKeyword = varPrefixWord
				p.advance()
				continue
			}
			return m
		}
		if t.Kind != lexer.Keyword {
			return m
		}
		switch t.KeywordID {
		case "abstract":
			m.isAbstract = true
		case "variation":
			m.isVariation = true
		case "ref":
			m.isReference = true
		case "end":
			m.isEnd = true
			// Check for early multiplicity: end [mult] ref ...
			// Peek ahead without advancing to see if next token is '['
			p.advance() // consume "end"
			if p.at(lexer.LBracket) {
				m.earlyMultiplicity = p.parseMultiplicity()
			}
			continue
		case "constant", "const":
			m.isConstant = true
		case "event":
			// Check if standalone usage: event <name>; (no typing/body)
			// If followed by identifier/qualified name (not keyword), it's usage keyword
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Identifier || (nextTok.Kind == lexer.Keyword && !p.isModifierOrKindKeyword(nextTok.KeywordID)) {
				// Treat as usage keyword, stop consuming modifiers
				return m
			}
			m.isEvent = true
		case "individual":
			// `individual` is a modifier orthogonal to the kind keyword (SysML v2
			// §8.3.9.11), except before `def` or a typing/specialization token,
			// where it is the declaration's own keyword.
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Colon || nextTok.Kind == lexer.ColonGt || nextTok.Kind == lexer.ColonGtGt {
				// individual : Type → anonymous usage
				return m
			}
			if nextTok.Kind == lexer.Keyword && nextTok.KeywordID == "def" {
				// individual def → DefIndividual keyword
				return m
			}
			m.isIndividual = true
		case "snapshot":
			// The portion loop in parseDefUsage reads a portion prefix the same way
			// for either keyword; only another modifier after it is handled here.
			nextTok := p.peekN(1)
			if nextTok.Kind != lexer.Keyword || !featureModifierKeywords[nextTok.KeywordID] {
				return m
			}
			m.portion = ast.PortionSnapshot
		case "public":
			m.visibility = ast.VisibilityPublic
		case "protected":
			m.visibility = ast.VisibilityProtected
		case "private":
			m.visibility = ast.VisibilityPrivate
		case "in":
			m.direction = ast.DirIn
		case "out":
			m.direction = ast.DirOut
		case "inout":
			m.direction = ast.DirInOut
		case "composite", "portion":
			m.isComposite = true
		case "readonly":
			m.isReadonly = true
		case "derived":
			m.isDerived = true
		case "ordered":
			m.isOrdered = true
		case "nonunique":
			m.isNonunique = true
		default:
			return m
		}
		p.advance()
	}
}

// parsePostModifiers parses feature modifiers that appear after typing/multiplicity.
// Currently only 'ordered' and 'nonunique' are allowed in this position.
// isPostModifierKeyword checks if token is a post-multiplicity modifier keyword
func isPostModifierKeyword(tok lexer.Token) bool {
	if tok.Kind != lexer.Keyword {
		return false
	}
	return tok.KeywordID == "ordered" || tok.KeywordID == "nonunique"
}

func (p *Parser) parsePostModifiers() featureMods {
	var m featureMods
	for {
		t := p.peek()
		if t.Kind != lexer.Keyword {
			return m
		}
		switch t.KeywordID {
		case "ordered":
			m.isOrdered = true
			p.advance()
		case "nonunique":
			m.isNonunique = true
			p.advance()
		case "terminate":
			// Consume terminate keyword - marks terminal action node
			// For now just consume it (no AST field, behavioral semantics)
			p.advance()
		default:
			return m
		}
	}
}

// modifierImpliedKind gives the kind an occurrence modifier declares with no kind
// keyword after it (SysML.xtext IndividualUsage, PortionUsage, EventOccurrenceUsage).
func modifierImpliedKind(mods featureMods) (ast.UsageKind, string) {
	switch {
	case mods.isIndividual:
		return ast.UsageIndividual, "individual"
	case mods.portion == ast.PortionSnapshot:
		return ast.UsageOccurrence, "snapshot"
	case mods.portion == ast.PortionTimeslice:
		return ast.UsageOccurrence, "timeslice"
	case mods.isEvent:
		return ast.UsageOccurrence, "event"
	}
	return ast.UsageAttribute, ""
}

// warnAmbiguousModifierKind reports `individual part : Vehicle` and its kin, where
// the kind keyword sits where a name would and so leaves the usage unnamed. isKind
// reports which keywords the caller reads as a kind, since a keyword it reads as a
// name is not ambiguous at all.
func (p *Parser) warnAmbiguousModifierKind(mods featureMods, isKind func(lexer.Token) bool) {
	if !mods.isIndividual && !mods.isReference && !mods.isEvent && mods.portion == ast.PortionNone {
		return
	}
	t := p.peek()
	if !isKind(t) || !namesDeclaration(p.peekN(1)) {
		return
	}
	p.warn(t.Span, "'"+t.KeywordID+"' is read as the kind of this usage, which is therefore unnamed; "+
		"name the usage after the keyword, or quote the keyword to use it as the name", codeAmbiguousModifierKind)
}

// parseDefUsage parses a definition or usage declaration. The caller has
// already established (via atDefUsageStart) that a def/usage begins here.
func (p *Parser) parseDefUsage(start int) ast.Node {
	// Parse optional `#MetadataType` prefixes (user-defined keywords)
	var prefixes []*ast.PrefixMetadata
	for p.at(lexer.Hash) {
		p.advance() // consume #
		// Allow keywords as metadata type names (e.g., #scenario, #cause)
		metaName := p.parseQualifiedNameRelaxed()
		if metaName != nil {
			prefixes = append(prefixes, &ast.PrefixMetadata{
				Type: metaName,
			})
		}
	}

	// Helper to apply prefixes to result node
	applyPrefixes := func(node ast.Node) ast.Node {
		if len(prefixes) == 0 {
			return node
		}
		if u, ok := node.(*ast.Usage); ok {
			u.Prefixes = append(prefixes, u.Prefixes...)
		} else if d, ok := node.(*ast.Definition); ok {
			d.Prefixes = append(prefixes, d.Prefixes...)
		}
		return node
	}

	// A specialization where the name would go declares an unnamed usage of the
	// default kind (`:>> x`, `redefines x`), whichever spelling it was written in.
	if p.atFeatureSpecialization() {
		u := p.parseUsage(start, ast.UsageAttribute, "", featureMods{}, false)
		return applyPrefixes(u)
	}

	mods := p.parseFeatureModifiers()

	// A satisfaction is negated with no `assert` before it: `not satisfy r by p;`
	// (SysML.xtext:2118, SatisfyRequirementUsage `'assert'? isNegated ?= 'not'?`).
	if p.atNegatedSatisfy() {
		mods.isNegated = true
		p.advance() // 'not'
	}

	// UsagePrefix ends in UsageExtensionKeyword* (SysML.xtext:582), so prefix
	// metadata may follow the modifiers: `abstract #Classified z;`.
	for p.at(lexer.Hash) {
		p.advance()
		if metaName := p.parseQualifiedNameRelaxed(); metaName != nil {
			prefixes = append(prefixes, &ast.PrefixMetadata{Type: metaName})
		}
	}

	// `snapshot` and `timeslice` portion the occurrence usage they prefix:
	// `timeslice item i;` as well as the bare `timeslice t;`
	// (SysML v2 8.3.9.11, PortionUsage).
	for p.atKeyword("snapshot") || p.atKeyword("timeslice") {
		tok := p.advance()
		portion := ast.PortionSnapshot
		if tok.KeywordID == "timeslice" {
			portion = ast.PortionTimeslice
		}
		if mods.portion != ast.PortionNone {
			p.error(tok.Span, "a usage declares at most one portion kind ('snapshot' or 'timeslice')")
		}
		mods.portion = portion
		// Without a kind keyword the portion itself declares an occurrence usage.
		if !p.atPortionedKind() {
			isAll := p.acceptSufficientAll()
			return applyPrefixes(p.parseUsage(start, ast.UsageOccurrence, tok.KeywordID, mods, isAll))
		}
	}

	p.warnAmbiguousModifierKind(mods, p.declarationKindKeyword)

	// Two-word `use case` kind keyword.
	if p.atUseCase() {
		p.advance() // 'use'
		p.advance() // 'case'
		if p.atKeyword("def") {
			p.advance() // 'def'
			return applyPrefixes(p.parseDefinition(start, ast.DefUseCase, "use case", mods, false, true))
		}
		return applyPrefixes(p.parseUsage(start, ast.UsageUseCase, "use case", mods, false))
	}

	t := p.peek()
	kw := ""
	if t.Kind == lexer.Keyword {
		kw = t.KeywordID
	}

	// Special case: perform action <name> (declaration form)
	// Pattern: perform action generateTorque: GenerateTorque;
	// Skip "perform" and parse as regular "action" usage
	if kw == "perform" && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "action" {
		p.advance() // consume 'perform'
		// The prefix says the action is performed by whatever declares it, which
		// the kind keyword alone would lose (PerformActionUsage, SysML v2 §7.17.6).
		mods.prefixKeyword = "perform"
		kw = "action" // treat as regular action keyword
		// Continue to dual-keyword path (don't enter usage-only block)
	} else if kw == "subject" || kw == "objective" || kw == "succession" || kw == "inv" || kw == "connect" || kw == "connector" || kw == "bind" || kw == "satisfy" || kw == "verify" || kw == "include" || kw == "step" || kw == "expr" || kw == "interaction" || kw == "require" || kw == "transition" || kw == "perform" || kw == "exhibit" || kw == "variant" || kw == "assert" || kw == "assume" || kw == "event" || kw == "stakeholder" || kw == "frame" || kw == "actor" || kw == "expose" || kw == "render" {
		// Check for usage-only keywords that never have def forms

		// Special case: perform <ref>; (shorthand without action keyword)
		// Must check BEFORE consuming keyword token
		if kw == "perform" && !p.atKeyword("action") {
			p.advance() // consume 'perform'
			return applyPrefixes(p.parsePerformedActionReference(start, mods, "perform"))
		}

		// A member keyword can also be an ordinary name: KerML has no `frame` or
		// `render` keyword, and the Kernel Semantic Library writes `in frame :
		// SpatialFrame[1]`. Read the keyword as the declaration's own name
		// unless what follows can begin the member form.
		if (kw == "frame" || kw == "render") && !p.atMemberKeywordUsedAsKeyword(kw) {
			return applyPrefixes(p.parseUsage(start, ast.UsageAttribute, "", mods, false))
		}

		// `connect a to b { … }` is a connection usage whose ends the keyword
		// introduces; it declares nothing of its own (SysML.xtext ConnectionUsage).
		if kw == "connect" {
			p.advance() // consume 'connect'
			return applyPrefixes(p.parseUsage(start, ast.UsageConnection, "connect", mods, false))
		}

		p.advance() // consume the kind keyword
		// `variant x` declares a variant of the variation that owns it
		// (VariantMembership, SysML v2 §7.20).
		if kw == "variant" {
			mods.isVariant = true
		}
		isAll := p.acceptSufficientAll()
		if kw == "exhibit" && !p.atKeyword("state") && p.atFeatureChainTarget() {
			return applyPrefixes(p.parseReferenceMemberUsage(
				start, ast.UsageState, kw, "state", mods, p.parseStateBody, false))
		}
		if kw == "bind" {
			u := p.parseUsage(start, p.usageKindOf(kw), kw, mods, isAll)
			return applyPrefixes(normalizeAnonymousBindingEnd(u))
		}

		// `render` names the rendering a view uses (ViewRenderingMember) and
		// `frame` the concern a requirement frames (FramedConcernMember). Each
		// owns a usage that either references an existing element —
		// `render asTreeDiagram;`, `frame 'system breakdown';` — or declares one
		// after the kind keyword the notation spells out, `render rendering r`
		// and `frame concern c` (SysML.xtext ViewRenderingUsage,
		// FramedConcernUsage; SysML v2 §8.3.20, §8.3.26). Only the declaration
		// form states a name.
		if kw == "render" || kw == "frame" {
			declKeyword, noun, body := "rendering", "rendering", p.parseDefUsageBodyMembers
			if kw == "frame" {
				declKeyword, noun, body = "concern", "concern", p.parseRequirementBody
			}
			if p.acceptKeyword(declKeyword) {
				return applyPrefixes(p.parseUsage(start, p.usageKindOf(kw), kw, mods, isAll))
			}
			return applyPrefixes(p.parseReferenceMemberUsage(start, p.usageKindOf(kw), kw, noun, mods, body, false))
		}

		// `exhibit state modes { … }` and `exhibit state modes : Modes;` state
		// the exhibited state after the state keyword, where the reference form
		// `exhibit modes;` names an existing one (SysML.xtext ExhibitStateUsage:
		// `'exhibit' ( OwnedReferenceSubsetting … | StateUsageKeyword
		// UsageDeclaration? ) ValuePart? StateUsageBody`; SysML v2 §8.3.17).
		if kw == "exhibit" && p.atKeyword("state") {
			p.advance() // consume 'state'
			return applyPrefixes(p.parseUsage(start, ast.UsageState, "exhibit state", mods, isAll))
		}

		// Special case: include use case <name> (full form)
		// If include is followed by "use case", consume them and parse as use case with includes relationship
		if kw == "include" && p.atUseCase() {
			p.advance() // consume 'use'
			p.advance() // consume 'case'
			u := p.parseUsage(start, ast.UsageUseCase, "use case", mods, isAll)
			if u != nil {
				// Add includes relationship to first typing target
				// Actually, include use case <name> : Type means: create use case usage <name> typed by Type, with includes semantics
				// The includes relationship is implicit in the 'include' keyword context
				// For now, we'll add an includes relationship with nil target (self-referential)
				// Or use a special flag. But spec may expect includes to target the typing.
				// Simplest: add includes relationship AFTER typing parsed
				if len(u.Relationships) > 0 && u.Relationships[0].Kind == ast.RelTyping {
					// Insert includes relationship pointing to typing target
					typing := u.Relationships[0].Target
					u.Relationships = append([]*ast.Relationship{
						{Kind: ast.RelIncludes, Target: typing},
					}, u.Relationships...)
				}
			}
			return applyPrefixes(u)
		}

		// A guard between the ends makes the succession a transition
		// (SysML.xtext:1719 GuardedSuccession returns TransitionUsage):
		// `succession S first a if g then b;`.
		if kw == "succession" && p.atGuardedSuccession() {
			return applyPrefixes(p.parseTransitionMember(start))
		}

		// Special case: succession flow from X to Y
		// If succession is followed by flow keyword, parse as flow usage with succession typing
		if kw == "succession" && p.atKeyword("flow") {
			p.advance() // consume 'flow'
			u := p.parseUsage(start, ast.UsageFlow, "flow", mods, isAll)
			// Add implicit succession typing - succession concept applies to this flow
			// Use typing relationship to indicate this flow has succession semantics
			if u != nil {
				// Add succession as typing (could also use specialization)
				// For now, treat as semantic annotation - flow inherits succession characteristics
				// Implementation note: May need dedicated AST flag or relationship for this hybrid
			}
			return applyPrefixes(u)
		}

		// `event m.start;` names an existing occurrence rather than declaring
		// one, and takes a value part like any usage (SysML.xtext
		// EventOccurrenceUsage: `'event' ( OwnedReferenceSubsetting
		// FeatureSpecializationPart? | OccurrenceUsageKeyword UsageDeclaration? )
		// UsageCompletion`).
		if kw == "event" && !p.atKeyword("occurrence") && !p.at(lexer.Colon) {
			u := p.parseReferenceMemberUsage(
				start, ast.UsageOccurrence, kw, "occurrence", mods, p.parseDefUsageBodyMembers, true)
			u.IsEvent = true
			return applyPrefixes(u)
		}

		// Special case: include <ref>; (shorthand for use case with includes relationship)
		// If include is NOT followed by "use case", parse as use case usage with includes
		// Pattern: include <ref>[mult] { body };
		if kw == "include" && !p.atUseCase() {
			u := &ast.Usage{
				Kind:        ast.UsageUseCase,
				IsAbstract:  mods.isAbstract,
				IsReference: mods.isReference,
				IsVariable:  mods.isVariable,
				IsEnd:       mods.isEnd,
				Visibility:  mods.visibility,
				Direction:   mods.direction,
				IsComposite: mods.isComposite,
			}
			u.NodeBase.NodeSpan = p.spanFrom(start)

			// Parse reference target
			target := p.parseRelationshipTarget()
			if target != nil {
				// Add as includes relationship
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelIncludes,
					Target: target,
				})
			} else {
				// The reference form of `include` subsets an existing use case, so it
				// names one (SysML.xtext:2300 IncludeUseCaseUsage).
				p.error(p.peek().Span, "expected a use case reference after 'include'")
			}

			// Optional multiplicity after reference
			if p.at(lexer.LBracket) {
				u.Multiplicity = p.parseMultiplicity()
			}

			// Expect semicolon or body
			if p.accept2(lexer.Semicolon) {
				u.HasBody = false
			} else if p.at(lexer.LBrace) {
				p.advance() // consume '{'
				// Parse body members
				var members []ast.Node
				for !p.at(lexer.RBrace) && !p.atEOF() {
					m := p.parseBodyMember()
					if m != nil {
						members = append(members, m)
					}
				}
				p.expect(lexer.RBrace, "expected '}' to close body")
				u.Members = members
				u.HasBody = true
			}

			u.NodeSpan = p.spanFrom(start)
			return applyPrefixes(u)
		}

		// `assert constraint { ... }` (likewise `assume`/`require`) spells the
		// kind after the prefix: the second keyword is the kind, so the
		// declaration is an anonymous constraint rather than one named
		// `constraint`.
		// `variant` likewise prefixes a kind when a name follows it
		// (`variant attribute diameterSmall = 70[mm];`); with no name, the
		// second keyword is the variant's own name.
		// `assert not constraint { … }` and `assert not satisfy … by …` negate the
		// declaration the prefix qualifies (Invariant::isNegated), so the `not`
		// belongs to it rather than to an expression. `assert not c { … }` negates
		// the reference form the same way, where `assert not (x > 1);` does negate
		// an expression (SysML.xtext:2008, AssertConstraintUsage).
		if kindPrefixKeywords[kw] && p.atKeyword("not") &&
			(p.isKindKeyword(p.peekN(1)) || p.namesReference(1)) {
			mods.isNegated = true
			p.advance()
		}

		// A `not` with nothing after it negates neither a declaration nor an
		// expression, so the assertion states no condition at all.
		if kindPrefixKeywords[kw] && p.atKeyword("not") {
			switch p.peekN(1).Kind {
			case lexer.Semicolon, lexer.RBrace, lexer.EOF:
				p.advance() // 'not'
				return p.errorNodeSkip(start, "expected a condition after 'not'")
			}
		}

		// `assert c;` and `assert not c { … }` name an existing constraint rather
		// than declaring one named `c` (SysML.xtext:2009, AssertConstraintUsage's
		// OwnedReferenceSubsetting).
		if kw == "assert" && p.atAssertedReference() {
			u := p.parseReferenceMemberUsage(
				start, ast.UsageConstraint, kw, "constraint", mods, p.parseConstraintBody, true)
			u.IsNegated = mods.isNegated
			return applyPrefixes(u)
		}

		// A prefix keyword qualifies a two-word kind as well as a one-word one:
		// `variant use case uc11;` is a use case usage (SysML.xtext:700,
		// VariantUsageElement).
		if (kindPrefixKeywords[kw] || kw == "variant") && p.atUseCase() {
			p.advance() // 'use'
			p.advance() // 'case'
			if kindPrefixKeywords[kw] {
				mods.prefixKeyword = kw
			}
			return applyPrefixes(p.parseUsage(start, ast.UsageUseCase, "use case", mods, isAll))
		}

		kindKeyword := kw
		if p.isKindKeyword(p.peek()) &&
			(kindPrefixKeywords[kw] || (kw == "variant" && !namesDeclaration(p.peekN(1)))) {
			kindKeyword = p.peek().KeywordID
			// `variant` is a modifier of the declaration it prefixes, recorded
			// as isVariant; a prefix keyword says what the declaration is for.
			if kindPrefixKeywords[kw] {
				mods.prefixKeyword = kw
			}
			p.advance()
		}
		return applyPrefixes(p.parseUsage(start, p.usageKindOf(kindKeyword), kindKeyword, mods, isAll))
	}

	// Special case: if current token is 'def' (after prefixes/modifiers), parse as generic definition
	// This handles patterns like `#scenario def X` where prefix acts as semantic annotation
	if p.atKeyword("def") {
		p.advance() // consume 'def'
		// Use generic definition kind (or could extract from prefix)
		return applyPrefixes(p.parseDefinition(start, ast.DefClass, "", mods, false, true))
	}

	defKind, ok := p.definitionKind(kw)
	if !ok {
		// Fallback: if we have modifiers but no kind keyword, assume it's a generic usage (e.g., "in x: Integer;")
		// This is common for parameters in calc/action bodies.
		// Also check if name + multiplicity/modifiers follow (e.g., "in seq[1..*] ordered;")
		// An `end` whose declaration is omitted entirely is only an interface
		// default end or an `end ref` (SysML v2 8.2.2.14.1).
		if mods.isEnd && (p.at(lexer.Semicolon) || p.at(lexer.LBrace)) {
			return applyPrefixes(p.parseAnonymousEndUsage(start, mods))
		}

		hasModifiers := mods.direction != ast.DirNone || mods.isReference || mods.isEnd ||
			mods.isComposite || mods.isDerived || mods.isIndividual || mods.isEvent ||
			mods.portion != ast.PortionNone
		hasNameWithMultOrMods := p.atNameOrKeyword() && (p.peekN(1).Kind == lexer.LBracket || p.peekN(1).Kind == lexer.Colon || isPostModifierKeyword(p.peekN(1)))
		// A DefaultReferenceUsage needs no keyword at all (SysML.xtext:632):
		// `T1 = 10.0;`, `distancePerVolume :> scalarQuantities = d / v;`.
		keywordlessDecl := p.atKeywordlessFeature()
		// SysML v2 §7.27.4: a user-defined keyword may declare a usage without
		// any language-defined keyword (`#cause 'battery old' { ... }`). The
		// kind of such a usage comes from the metadata, not the syntax.
		keywordOnlyUsage := len(prefixes) > 0 &&
			(p.at(lexer.Identifier) || p.at(lexer.UnrestrictedName) || p.atFeatureSpecialization())
		if hasModifiers || hasNameWithMultOrMods || keywordOnlyUsage || keywordlessDecl {
			kind, keyword := modifierImpliedKind(mods)
			return applyPrefixes(p.parseUsage(start, kind, keyword, mods, false))
		}
		return applyPrefixes(nil)
	}
	p.advance() // consume the kind keyword

	// `individual : T` is an anonymous individual occurrence usage; `snapshot` is
	// handled with the usage-only keywords above.
	if kw == "individual" {
		mods.isIndividual = true
	}

	// Parse 'all' modifier if present (appears after keyword, before name)
	isAll := p.acceptSufficientAll()

	// Parse 'chain' modifier if present (identifier, not keyword)
	t2 := p.peek()
	if t2.Kind == lexer.Identifier && p.src.Text(t2.Span) == "chain" {
		mods.isChain = true
		p.advance()
	}

	// Parse a secondary kind keyword, which happens either in a compound kind
	// (`assoc struct`) or when the first keyword prefixes the second
	// (`item part Shape`). A kind keyword that is not one of those is the
	// declaration's name (`attribute item : Integer` names the attribute
	// `item`), and consuming it here would silently discard that name.
	kindKeyword := kw
	if p.atSecondaryKind(kw) {
		kindKeyword = p.peek().KeywordID
		defKind = p.definitionKindOf(kindKeyword)
		p.advance() // consume secondary keyword
	}

	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return applyPrefixes(p.parseDefinition(start, defKind, kindKeyword, mods, isAll, true))
	}

	// Check if this is a definition-only keyword (not in usageKindKeywords)
	// Examples: metaclass, struct, class, predicate, bool
	// These keywords don't require "def" suffix and can't be used as usages
	_, hasUsageForm := p.usageKind(kw)
	if !hasUsageForm {
		// Definition-only keyword - parse as definition directly
		return applyPrefixes(p.parseDefinition(start, defKind, kindKeyword, mods, isAll, false))
	}

	// Note: 'datatype' is treated uniformly as a usage keyword by the parser.
	// Semantic classification (def vs usage) is deferred to the symbol builder
	// and semantics passes, which have full context (relationships, body structure).
	// This follows Phase 4 principle: parse syntax uniformly, classify semantically.

	// A secondary keyword refines a definition's kind (`individual item def`),
	// but a usage keeps the kind of the first keyword (`item part Shape` is an
	// item usage), so the keyword recorded here is that first one.
	return applyPrefixes(p.parseUsage(start, p.usageKindOf(kw), kw, mods, isAll))
}

// parseDefinition parses a definition. keyword is the kind keyword as consumed
// from the token stream, kept so a synonym spelling can be told from the
// canonical one. defKeywordConsumed distinguishes SysML <kind> def from
// KerML classifier declarations, which share DefinitionKind values.
func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, keyword string, mods featureMods, isAll bool, defKeywordConsumed bool) *ast.Definition {
	def := &ast.Definition{
		Kind:        kind,
		Keyword:     keyword,
		IsAbstract:  mods.isAbstract,
		IsVariation: mods.isVariation,
		IsAll:       isAll,
		IsConstant:  mods.isConstant,
		IsEvent:     mods.isEvent,
		Visibility:  mods.visibility,
		Ident:       p.parseIdentification(),
	}
	if !defKeywordConsumed && isKerMLClassifierDefinitionKeyword(keyword) && p.at(lexer.LBracket) {
		def.Multiplicity = p.parseMultiplicity()
	}
	def.Relationships = p.parseRelationships(false)

	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	defer p.pushBodyContext(defBodyContext(kind))()
	switch kind {
	case ast.DefAction, ast.DefOccurrence:
		// Action/occurrence def bodies: mixed (declarations + behavioral statements)
		// Occurrence defs support temporal ordering of messages/events (interactions)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseActionBodyMixed()
			hasBody = true
		}
	case ast.DefCalc:
		// Calculation def bodies: mixed (parameters + return statements)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCalcBody()
			hasBody = true
		}
	case ast.DefConstraint:
		// Constraint def bodies: always use parseConstraintBody (handles assert/assume/bare expressions)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseConstraintBody()
			hasBody = true
		}
	case ast.DefRequirement, ast.DefConcern, ast.DefViewpoint:
		// Requirement def bodies: a requirement member may appear anywhere in the
		// body, not only first, so the whole body is parsed as a requirement body
		// (which falls back to general body members). A concern definition is a
		// requirement definition and a viewpoint definition a concern definition
		// (SysML v2 §7.19), so all three carry require/assume/subject/actor.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseRequirementBody()
			hasBody = true
		}
	case ast.DefState:
		// State def bodies are state bodies, like state usage bodies: what the
		// first member happens to be does not change what the rest may be, and
		// the generic body member parser knows nothing of regions or transitions.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseStateBody()
			hasBody = true
		}
	case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
		// Case bodies may end in a ResultExpressionMember (SysML.xtext
		// CalculationBodyPart): `vehicle.mass` as the last member.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCaseBody()
			hasBody = true
		}
	case ast.DefEnumeration:
		// Enumeration bodies hold EnumeratedValues, whose keyword and
		// declaration are both optional (SysML.xtext EnumeratedValue): `= 60.0;`.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseEnumBody()
			hasBody = true
		}
	default:
		members, hasBody = p.parseDefUsageBody()
	}

	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}

// isKerMLClassifierDefinitionKeyword identifies keyword forms using KerML's
// ClassifierDeclaration (and the shared TypeDeclaration) multiplicity slot.
func isKerMLClassifierDefinitionKeyword(keyword string) bool {
	switch keyword {
	case "type", "classifier", "class", "datatype", "struct", "assoc", "behavior",
		"function", "predicate", "interaction", "metaclass":
		return true
	default:
		return false
	}
}

// defBodyContext returns the body notation a definition of the given kind
// declares its members in.
func defBodyContext(kind ast.DefinitionKind) bodyContext {
	switch kind {
	case ast.DefInterface:
		return bodyInterface
	}
	return bodyOther
}

// usageBodyContext returns the body notation a usage of the given kind declares
// its members in.
func usageBodyContext(kind ast.UsageKind) bodyContext {
	switch kind {
	case ast.UsageInterface:
		return bodyInterface
	}
	return bodyOther
}

// isBehavioralKeyword checks if next token is a behavioral keyword
func (p *Parser) isBehavioralKeyword() bool {
	if !p.at(lexer.Keyword) {
		// `done;` and the other unreserved node words, in the node shape only.
		_, ok := p.atActionNodeWord()
		return ok
	}
	kw := p.peek().KeywordID
	switch kw {
	case "first", "fork", "join", "merge", "decide", "action", "then",
		"assign", "perform", "while", "loop", "if", "send", "terminate", "for",
		// `else <target>;` is a DefaultTargetSuccession member (SysML.xtext).
		"else":
		return true
	}
	return false
}

// isResultKeyword checks if next token is 'return'
func (p *Parser) isResultKeyword() bool {
	return p.at(lexer.Keyword) && p.peek().KeywordID == "return"
}

// parseUsageIdentification parses identification for usage declarations, with special handling
// for step usage to allow "do" keyword as identifier name (since "do" is a valid step name like entry/exit).
func (p *Parser) parseUsageIdentification(kind ast.UsageKind) ast.Identification {
	// Special case: step usage allows "do" as identifier
	if kind == ast.UsageStep && p.atKeyword("do") {
		tok := p.advance()
		return ast.Identification{
			Name: tok.KeywordID,
		}
	}
	// Default: use standard identification parsing
	return p.parseIdentification()
}

// atGuardedSuccession reports whether the succession being read states a guard
// between its ends (`succession [name] first a if g then b`), which is the
// GuardedSuccession production and so a transition, not a connector.
func (p *Parser) atGuardedSuccession() bool {
	i := 0
	if p.peek().Kind == lexer.Identifier || p.peek().Kind == lexer.UnrestrictedName {
		i = 1
	}
	if !p.peekIsKeyword(i, "first") {
		return false
	}
	for depth := 0; i < 60; i++ {
		tok := p.peekN(i)
		switch tok.Kind {
		case lexer.EOF, lexer.Semicolon, lexer.LBrace, lexer.RBrace:
			return false
		case lexer.LParen, lexer.LBracket:
			depth++
		case lexer.RParen, lexer.RBracket:
			depth--
		case lexer.Keyword:
			if depth == 0 && tok.KeywordID == "then" {
				return false
			}
			if depth == 0 && tok.KeywordID == "if" {
				return true
			}
		}
	}
	return false
}

// isAnonymousSuccession checks if we're at the start of anonymous succession ends (no name).
// Anonymous succession patterns:
// - `succession [mult] first [mult] x then y` - mult + "first" keyword (NO name between)
// - `succession first [mult] x then y` - starts with "first" keyword
// - `succession x then y` - identifier followed by "then" (not name, but first connector end)
// - `succession x.y then z` - feature chain followed by "then" (not name, but first connector end)
// Named succession patterns (NOT anonymous):
// - `succession [mult] name first [mult] x then y` - mult + identifier + "first" (identifier is NAME)
// - `succession name first [mult] x then y` - identifier + "first"
func (p *Parser) isAnonymousSuccession() bool {
	if p.at(lexer.LBracket) {
		// Starts with multiplicity - lookahead past it to check what follows
		i := 1
		// Skip multiplicity tokens: [, expressions (identifiers, numbers, operators), .., *, ]
		depth := 1 // track bracket nesting for complex expressions
		for i < 30 && depth > 0 {
			tok := p.peekN(i)
			if tok.Kind == lexer.RBracket {
				depth--
				if depth == 0 {
					// Found closing bracket, check next token
					i++
					break
				}
			}
			if tok.Kind == lexer.LBracket {
				depth++
			}
			// Allow any token inside multiplicity (expressions can be complex)
			// Just skip to matching closing bracket
			i++
		}
		// After closing bracket, check next token
		nextTok := p.peekN(i)
		if nextTok.Kind == lexer.Keyword && nextTok.KeywordID == "first" {
			// Pattern: `succession [mult] first ...` - anonymous
			return true
		}
		// Pattern: `succession [mult] identifier ...` - could be named or anonymous
		// Check if identifier followed by "first" keyword (named) or "then" keyword (anonymous)
		if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.UnrestrictedName || nextTok.Kind == lexer.Keyword {
			i++
			// Skip feature chain (dots, identifiers)
			for i < 30 {
				tok := p.peekN(i)
				if tok.Kind == lexer.Keyword && tok.KeywordID == "first" {
					// Pattern: `succession [mult] name first ...` - NAMED succession
					return false
				}
				if tok.Kind == lexer.Keyword && tok.KeywordID == "then" {
					// Pattern: `succession [mult] x.y then ...` - anonymous (x.y is connector end)
					return true
				}
				if tok.Kind == lexer.LBracket || tok.Kind == lexer.RBracket || tok.Kind == lexer.Decimal || tok.Kind == lexer.DotDot || tok.Kind == lexer.Star {
					i++
					continue // skip multiplicity
				}
				if tok.Kind == lexer.Dot || tok.Kind == lexer.ColonColon || tok.Kind == lexer.Identifier || tok.Kind == lexer.Keyword {
					i++
					continue // skip feature chain parts
				}
				// Unknown token, assume named
				return false
			}
		}
		// Couldn't determine, assume named
		return false
	}
	if p.atKeyword("first") {
		return true // starts with "first" keyword
	}
	// Check for pattern: identifier/feature chain + "then" (means identifier is connector end, not name)
	if p.atName() || p.atNameOrKeyword() || p.at(lexer.Keyword) {
		// Special case: if identifier immediately followed by "first", it's a NAMED succession
		// Pattern: succession name first [mult] x then y
		// Also check: succession name[mult] first x then y
		nextIdx := 1
		nextTok := p.peekN(nextIdx)

		// Skip multiplicity if present: [...]
		if nextTok.Kind == lexer.LBracket {
			depth := 1
			nextIdx++
			for nextIdx < 30 && depth > 0 {
				tok := p.peekN(nextIdx)
				if tok.Kind == lexer.LBracket {
					depth++
				} else if tok.Kind == lexer.RBracket {
					depth--
				}
				nextIdx++
			}
			nextTok = p.peekN(nextIdx)
		}

		// Check if "first" follows (after optional multiplicity)
		if nextTok.Kind == lexer.Keyword && nextTok.KeywordID == "first" {
			return false // NAMED succession
		}

		// Count identifiers before "then" to distinguish:
		// - succession name end1 then end2 (2 identifiers) - NAMED
		// - succession end1 then end2 (1 identifier) - ANONYMOUS
		identCount := 1 // current identifier (at position 0)
		for i := 1; i < 30; i++ {
			tok := p.peekN(i)
			if tok.Kind == lexer.EOF {
				return false
			}
			if tok.Kind == lexer.Keyword && tok.KeywordID == "then" {
				// Found "then" - check identifier count
				// If 1 identifier before "then", it's anonymous (identifier is connector end)
				// If 2+ identifiers, first is name, second is connector end - NAMED
				return identCount == 1
			}
			// Count identifiers (simple names, not part of feature chains)
			// Only count as separate identifier if preceded by whitespace/nothing, not dot/::
			if tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName {
				prevTok := p.peekN(i - 1)
				if prevTok.Kind != lexer.Dot && prevTok.Kind != lexer.ColonColon {
					identCount++
				}
			}
			// Skip over multiplicity syntax, dots, :: for feature chains
			if tok.Kind == lexer.LBracket || tok.Kind == lexer.RBracket ||
				tok.Kind == lexer.Decimal || tok.Kind == lexer.DotDot || tok.Kind == lexer.Star ||
				tok.Kind == lexer.Dot || tok.Kind == lexer.ColonColon || tok.Kind == lexer.Whitespace {
				continue
			}
			// If not identifier/keyword and not "then", stop searching
			if tok.Kind != lexer.Identifier && tok.Kind != lexer.UnrestrictedName && tok.Kind != lexer.Keyword {
				return false
			}
		}
	}
	return false
}

// parseUsage parses a usage. keyword is the kind keyword as consumed from the
// token stream, kept for the same reason as in parseDefinition.
func (p *Parser) parseUsage(start int, kind ast.UsageKind, keyword string, mods featureMods, isAll bool) *ast.Usage {
	u := &ast.Usage{
		Kind:          kind,
		Keyword:       keyword,
		PrefixKeyword: mods.prefixKeyword,
		IsNegated:     mods.isNegated,
		IsAbstract:    mods.isAbstract,
		IsVariation:   mods.isVariation,
		IsVariant:     mods.isVariant,
		IsReference:   mods.isReference,
		IsVariable:    mods.isVariable,
		IsAll:         isAll,
		IsEnd:         mods.isEnd,
		IsChain:       mods.isChain,
		IsConstant:    mods.isConstant,
		IsEvent:       mods.isEvent,
		IsIndividual:  mods.isIndividual,
		Portion:       mods.portion,
		Visibility:    mods.visibility,
		Direction:     mods.direction,
		IsComposite:   mods.isComposite,
		IsDerived:     mods.isDerived,
		IsOrdered:     mods.isOrdered,
		IsNonunique:   mods.isNonunique,
	}

	// Apply early multiplicity if present (for "end [mult] ref ..." syntax)
	if mods.earlyMultiplicity != nil {
		u.Multiplicity = mods.earlyMultiplicity
	}

	// Handle UsageSatisfy special syntax:
	// Full form: satisfy [requirement] <name> by <name> { body }
	// Short form: satisfy/verify <name>;
	if kind == ast.UsageSatisfy {
		// The declaration form is a full UsageDeclaration, so it may state a
		// type as well as a name (`satisfy requirement r : Req1 by v;`), while
		// the reference form names an existing requirement usage and declares
		// nothing.
		if p.acceptKeyword("requirement") {
			// The UsageDeclaration is optional, and `by` introduces the subject
			// rather than naming the satisfaction (`satisfy requirement by v;`).
			if !p.atKeyword("by") {
				u.Ident = p.parseUsageIdentification(kind)
			}
			declRels := p.parseRelationships(true)
			u.Relationships = append(u.Relationships, declRels...)
		} else if reqName := p.parseQualifiedName(); reqName != nil {
			// SysML.xtext:2119 owns a ReferenceSubsetting here; recorded as a
			// plain subsetting until its passes/runtime readers migrate.
			u.Relationships = append(u.Relationships, &ast.Relationship{
				Kind:   ast.RelSubsets,
				Target: reqName,
			})
			// The reference form takes specializations of its own:
			// `verify r :>> massRequirement;` (SysML.xtext:2272,
			// RequirementVerificationUsage's `FeatureSpecialization*`).
			u.Relationships = append(u.Relationships, p.parseRelationships(true)...)
		}

		// ValuePart? — a satisfy usage may bind a value like any usage.
		if p.acceptValueOperator() {
			u.Value = p.ParseExpression()
		}

		// Check for optional "by" clause. Per SatisfyRequirementUsage the `by`
		// operand names the subject of the satisfaction, never the usage itself,
		// so it is always recorded as a subject relationship.
		if p.acceptKeyword("by") {
			if subjTarget := p.parseRelationshipTarget(); subjTarget != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelSubject,
					Target: subjTarget,
				})
			}
		}

		// A satisfy usage is a requirement usage (SysML v2 §7.20), so its body
		// carries requirement members.
		if p.accept2(lexer.Semicolon) {
			u.HasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			leave := p.pushBodyContext(usageBodyContext(kind))
			u.Members = p.parseRequirementBody()
			leave()
			u.HasBody = true
		}
		u.NodeSpan = p.spanFrom(start)
		return u
	}

	// Handle UsageBinding special syntax: binding [mult] name = [mult] target; OR binding name[mult] of [mult] target = [mult] value;
	if kind == ast.UsageBinding {
		// Check for multiplicity before name: binding [mult] name ...
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
		}

		// The UsageDeclaration before `bind` may specialize without naming the
		// connector: `binding : AB bind a = b;` (SysML.xtext BindingConnectorAsUsage).
		u.Relationships = append(u.Relationships, p.parseRelationships(true)...)

		// `binding { … }` states its ends as body members and so names nothing
		// at all (KerML.xtext BindingConnectorDeclaration).
		if p.at(lexer.LBrace) || p.at(lexer.Semicolon) {
			// no declaration and no ends before the body
		} else if p.atKeyword("bind") {
			// `binding [mult] bind [mult] src = [mult] tgt` states the connector's
			// ends after the `bind` keyword instead of naming the connector, so the
			// keyword is consumed rather than read as the name.
			p.advance()
			if p.at(lexer.LBracket) {
				p.parseMultiplicity() // end multiplicity, not the connector's
			}
			if source := p.parseRelationshipTarget(); source != nil {
				u.Relationships = append(u.Relationships, bindingEnd(source))
			}
		} else if p.atNameOrKeyword() && p.peekN(1).Kind != lexer.Dot && p.peekN(1).Kind != lexer.ColonColon && p.peekN(1).Kind != lexer.LBracket {
			// Parse source (name or feature chain like x.field)
			// Check if simple name or feature chain
			// Simple name - use as identification
			u.Ident = p.parseIdentification()
		} else if p.atNameOrKeyword() && p.peekN(1).Kind == lexer.LBracket {
			// Name with multiplicity after it: name[mult]
			// Parse as identification first
			u.Ident = p.parseIdentification()
			// Don't parse multiplicity yet, handle after checking for "of"
		} else {
			// A qualified name or feature chain here states the binding's first
			// end, not the connector's name.
			source := p.parseRelationshipTarget()
			if source != nil {
				u.Relationships = append(u.Relationships, bindingEnd(source))
			}
		}

		// A named declaration specializes either side of its multiplicity
		// (FeatureSpecializationPart): `binding ab1 : AB bind a = b;`.
		u.Relationships = append(u.Relationships, p.parseRelationships(true)...)

		// Check for multiplicity after name (before "of"): name[mult] of ...
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
			u.Relationships = append(u.Relationships, p.parseRelationships(true)...)
		}

		// `binding name bind src = tgt` both names the connector and states its
		// ends, so the keyword follows the declaration instead of replacing it.
		if p.atKeyword("bind") {
			p.advance()
			if p.at(lexer.LBracket) {
				p.parseMultiplicity() // end multiplicity, not the connector's
			}
			if source := p.parseRelationshipTarget(); source != nil {
				u.Relationships = append(u.Relationships, bindingEnd(source))
			}
		}

		// Check for source expression: binding [mult] name[mult2] source = target
		// If we have name[mult] and next token is NOT "of" or "=", parse source expression
		if u.Ident.Name != "" && !p.atKeyword("of") && !p.at(lexer.Eq) && (p.atName() || p.atNameOrKeyword()) {
			// Parse source as relationship target
			source := p.parseRelationshipTarget()
			if source != nil {
				u.Relationships = append(u.Relationships, bindingEnd(source))
			}
		}

		// Check for "of" keyword (binding name of [mult] target = value)
		// In KerML, `of x = y` states both ends (KerML.xtext:875); SysML spells
		// that form `bind` and keeps `=` as a value.
		sawOf := p.atKeyword("of") && p.src.Kind() == source.KindKerML
		if p.acceptKeyword("of") {
			// Parse source multiplicity and target
			if p.at(lexer.LBracket) {
				// Store source multiplicity somewhere - for now skip or use relationships
				p.parseMultiplicity() // consume but don't store separately
			}
			// `of` names the feature the binding binds, not its type.
			target := p.parseRelationshipTarget()
			if target != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelReferences,
					Target: target,
				})
			}
		}

		// A KerML binding has no feature value, so `x = y` states its two ends and
		// what looked like the name is the first (KerML.xtext:879).
		endsOnly := sawOf || (p.src.Kind() == source.KindKerML && p.at(lexer.Eq))
		if endsOnly && u.Ident.Name != "" && !hasBindingEnd(u) {
			u = normalizeAnonymousBindingEnd(u)
		}

		// Parse value: = [mult] expr
		if p.accept2(lexer.Eq) {
			// Optional multiplicity before value expression
			if p.at(lexer.LBracket) {
				p.parseMultiplicity() // consume multiplicity prefix in value
			}
			if endsOnly {
				if target := p.parseRelationshipTarget(); target != nil {
					u.Relationships = append(u.Relationships, bindingEnd(target))
				}
			} else {
				u.Value = p.ParseExpression()
			}
		}

		// Parse body or semicolon
		leave := p.pushBodyContext(usageBodyContext(kind))
		members, hasBody := p.parseDefUsageBody()
		leave()
		u.Members = members
		u.HasBody = hasBody
		u.NodeSpan = p.spanFrom(start)
		return u
	}

	// Handle succession/connector/flow with multiplicity before name/first keyword
	// Pattern: `succession [mult] name first [mult] x then [mult] y`
	// Pattern: `succession [mult] first [mult] x then [mult] y` (anonymous)
	// Pattern: `connector [mult] name from [mult] x to [mult] y`
	// Check for anonymous succession BEFORE consuming multiplicity
	var earlyMultiplicity *ast.Multiplicity
	var isAnonymous bool
	if kind == ast.UsageSuccession {
		isAnonymous = p.isAnonymousSuccession()
	}
	if (kind == ast.UsageSuccession || kind == ast.UsageConnector || kind == ast.UsageFlow) && p.at(lexer.LBracket) {
		earlyMultiplicity = p.parseMultiplicity()
	}

	// A declaration stating a specialization before its name has no name to state
	// (SysML.xtext FeatureDeclaration): `part redefines wheel` is the same unnamed
	// usage as `part :>> wheel`. The name it answers to is its redefinition's, and
	// the symbol layer derives that (KerML 7.3.4.5, symbols.effectiveIdent).
	preRels := p.parsePreNameRelationships(true)
	// A bare flow shorthand `flow x to y` and anonymous succession `succession x then y` have no declaration name
	// Anonymous connector starts with 'from' keyword (e.g., `connector : X from y to z`)
	// A connection or interface stating ends where its name would go declares
	// nothing of its own either: `interface b1.p to b2.p`.
	skipIdentification := (kind == ast.UsageFlow && (p.atFlowShorthand() || p.atKeyword("from"))) ||
		(kind == ast.UsageSuccession && isAnonymous) ||
		(kind == ast.UsageAllocation && p.atAllocateShorthand()) ||
		(kind == ast.UsageConnector && (p.atKeyword("from") || p.atConnectorChainFirstEnd())) ||
		((kind == ast.UsageConnection || kind == ast.UsageInterface) &&
			(keyword == "connect" || p.atConnectorShorthandEnds()))
	if !skipIdentification {
		u.Ident = p.parseUsageIdentification(kind)
	}

	// Parse post-identification relationships (e.g., : Type)
	postIdRels := p.parseRelationships(true)
	u.Relationships = append(preRels, postIdRels...)

	// For anonymous succession/flow, skip multiplicity parsing - it belongs to connector ends
	// UNLESS earlyMultiplicity was already parsed (e.g., `succession [mult] first ...`)
	// In the shorthand forms a multiplicity here is the first end's.
	// A succession stating a specialization instead of a name still has a
	// declaration of its own, so the multiplicity is the declaration's
	// (KerML.xtext:891, SuccessionDeclaration's FeatureDeclaration alternative).
	declared := u.Ident.Name != "" || u.Ident.ShortName != "" || len(u.Relationships) > 0
	skipMultiplicity := ((kind == ast.UsageSuccession || kind == ast.UsageFlow) && !declared && earlyMultiplicity == nil) ||
		((kind == ast.UsageConnection || kind == ast.UsageInterface) && skipIdentification)
	if !skipMultiplicity {
		if earlyMultiplicity != nil {
			u.Multiplicity = earlyMultiplicity
		} else {
			u.Multiplicity = p.parseMultiplicity()
		}
	}

	// Parse post-multiplicity modifiers (ordered/nonunique)
	postMods := p.parsePostModifiers()
	if postMods.isOrdered {
		u.IsOrdered = true
	}
	if postMods.isNonunique {
		u.IsNonunique = true
	}

	// DEBUG: trace token after post-modifiers
	// fmt.Printf("DEBUG parseUsage after postMods: tok=%v keyword=%q offset=%d\n",
	//     p.peek().Kind, p.peek().KeywordID, p.peek().Span.Offset)

	// Parse additional relationships after modifiers (e.g., :> target)
	postRels := p.parseRelationships(true)
	u.Relationships = append(u.Relationships, postRels...)
	p.checkTypeDeclarationSpecialization(u, keyword)

	if p.acceptValueOperator() {
		u.Value = p.ParseExpression()
	}
	p.parseTierBEnds(u, kind)

	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	defer p.pushBodyContext(usageBodyContext(kind))()
	switch kind {
	case ast.UsageAction:
		// Action usage bodies: mixed (declarations + behavioral statements)
		// Support THREE forms:
		// 1. action name; (no body)
		// 2. action name { in item x; action nested {...}; first ...; } (braced mixed body)
		// 3. action name \n statements (inline behavioral body without braces)
		if p.atTransitionEffectStatement(start) && p.atEffectEnd() {
			// 4. action written as a transition's effect: no body of its own, and
			// the transition owns the ';' (`do action alarm : Alarm;`).
			hasBody = false
		} else if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.at(lexer.LBrace) {
			// Braced body - use mixed parser (handles declarations + behavioral)
			_, ok := p.expect(lexer.LBrace, "expected '{'")
			if ok {
				members = p.parseActionBodyMixed()
				hasBody = true
			}
		} else if p.isBehavioralKeyword() {
			// Inline behavioral body without braces: action name\n assign ...;
			// The body is a single statement plus any 'then'-chained continuations;
			// a following statement that is not chained belongs to the enclosing body.
			// EXCEPT: a 'then' chaining to a declaration is namespace-level
			// succession, not behavioral succession - stop parsing body
			//
			// An unbraced body written as a transition's effect ends where the
			// transition does, so the transition owns its statement's ';'.
			inEffect := p.atTransitionEffectStatement(start)
			savedEffectStmtStart := p.effectStmtStart
			for !p.atEOF() && !p.atNamespaceSuccession() {
				if inEffect {
					p.effectStmtStart = p.peek().Span.Offset
				}
				members = append(members, p.parseActionMember())
				// Only an inline statement continues the body; `then a b;` names
				// members of the enclosing body.
				if !p.atKeyword("then") || !startsInlineSuccessionStatement(p.peekN(1)) {
					break
				}
			}
			p.effectStmtStart = savedEffectStmtStart
			hasBody = true
		} else {
			// Expected ';' or '{' or behavioral keyword
			p.error(p.peek().Span, "expected '{' or ';' after action declaration")
		}
	case ast.UsageCalc:
		// Calculation usage bodies: mixed (parameters + return statements)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCalcBody()
			hasBody = true
		}
	case ast.UsageExpr:
		// An expression's body is members ending in the expression it computes
		// (KerML.xtext Expression): `expr e1 {v > 3}`.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCaseBody()
			hasBody = true
		}
	case ast.UsageBool:
		// Bool usage bodies: can be calc-style (with return) OR constraint-style (single expression)
		// Lookahead: if body starts with 'in' or 'return' → calcBody, otherwise → constraint-style expression
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			// Peek at first token in body
			firstTok := p.peek()
			if firstTok.Kind == lexer.Keyword && (firstTok.KeywordID == "in" || firstTok.KeywordID == "return") {
				// Structured calc body with parameters/return
				members = p.parseCalcBody()
			} else {
				// Single expression body (constraint-style)
				members = p.parseConstraintBody()
			}
			hasBody = true
		}
	case ast.UsageConstraint, ast.UsagePredicate:
		// Constraint bodies: { assert/assume expr; ... }
		// Bool and predicate usages also use constraint-style bodies with expressions
		// Special case: if body starts with 'in' or 'return' keyword, parse as calc body (structured parameters)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if p.at(lexer.LBrace) {
			// Check if this is a typed predicate with input/return parameters
			// Peek ahead past the '{' and any 'doc' keywords to see if body has 'in' or 'return'
			hasCalcBody := false
			for i := 1; i < 10; i++ { // Look ahead up to 10 tokens
				tok := p.peekN(i)
				if tok.KeywordID == "doc" {
					continue // Skip doc keywords
				}
				if tok.KeywordID == "in" || tok.KeywordID == "return" {
					hasCalcBody = true
				}
				break // Stop at first non-doc keyword
			}

			p.advance() // {
			if hasCalcBody {
				// Parse as calc body with structured parameters
				members = p.parseCalcBody()
			} else {
				members = p.parseConstraintBody()
			}
			hasBody = true
		} else {
			p.expect(lexer.LBrace, "expected '{' or ';'")
		}
	case ast.UsageRequirement, ast.UsageConcern, ast.UsageViewpoint, ast.UsageFramedConcern, ast.UsageObjective:
		// Requirement bodies: { subject/assume/require/actor ... }. A concern
		// usage is a requirement usage and a viewpoint usage a concern usage
		// (SysML v2 §7.19), so they carry the same members; a framed concern is
		// a concern usage and its declaration form ends in a RequirementBody
		// (SysML.xtext FramedConcernUsage). An objective is a requirement usage
		// too (SysML.xtext ObjectiveRequirementUsage).
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseRequirementBody()
			hasBody = true
		}
	case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		// Case bodies may end in a ResultExpressionMember (SysML.xtext
		// CalculationBodyPart): `vehicle.mass` as the last member.
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCaseBody()
			hasBody = true
		}
	case ast.UsageState:
		// State usage bodies: always use parseStateBody (it handles both state-specific and generic members)
		// Optional: parallel or exclusive keyword before body
		if p.atKeyword("parallel") || p.atKeyword("exclusive") {
			// Consume keyword (could store in AST if needed)
			p.advance()
		}
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseStateBody()
			hasBody = true
		}
	default:
		members, hasBody = p.parseDefUsageBody()
	}

	// A flow's `of name : Type` clause contributes a member before the body is
	// parsed, so the body members are appended rather than replacing it.
	u.Members = append(u.Members, members...)
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseSuccessionAsUsage parses a succession stated without the `succession`
// keyword (SysML v2 8.2.2.13.3): `first a then b;`.
func (p *Parser) parseSuccessionAsUsage(start int) ast.Node {
	u := &ast.Usage{Kind: ast.UsageSuccession}
	p.parseConnectorEnds(u, "")
	u.Members, u.HasBody = p.parseDefUsageBody()
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseDefUsageBody parses a definition/usage body: `;` (no body) or
// `{ member* }`. Body members may be nested def/usage declarations or ordinary
// namespace members, each carrying optional visibility.
func (p *Parser) parseDefUsageBody() (members []ast.Node, hasBody bool) {
	if p.accept2(lexer.Semicolon) {
		return nil, false
	}
	if _, ok := p.expect(lexer.LBrace, "expected '{' or ';' after declaration"); !ok {
		return nil, false
	}
	return p.parseDefUsageBodyMembers(), true
}

// parseDefUsageBodyMembers parses the members of a definition/usage body up to
// and including its closing brace, with the opening brace already consumed.
func (p *Parser) parseDefUsageBodyMembers() []ast.Node {
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		// A member-attached `then` sequences the members either side of it, so
		// the keyword is taken here and the member it prefixes read next time
		// round (see succession.go).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		// `then a b;` is a succession member naming two members of this body,
		// which is the form a member-attached `then` desugars to and so the
		// form a converted model is written back as.
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance()))
			continue
		}
		body.add(p.parseBodyMember())
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return body.finish()
}

// parseCaseBody parses a case body's members, which may end in a result
// expression member (SysML.xtext CalculationBodyPart): `vehicle.mass`.
func (p *Parser) parseCaseBody() []ast.Node {
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance()))
			continue
		}
		if p.atResultExpression() {
			body.add(p.ParseExpression())
			continue
		}
		body.add(p.parseBodyMember())
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return body.finish()
}

// atResultExpression reports whether the current token begins a bare result
// expression rather than a member declaration. Keyword-led members belong to
// the member parser, and a name whose next token continues a declaration is a
// declaration.
func (p *Parser) atResultExpression() bool {
	t := p.peek()
	if (t.Kind == lexer.Keyword && !exprStartKeywords[t.KeywordID]) || t.Kind == lexer.LBrace {
		return false
	}
	if !p.atExprStart() || p.atVarDeclaration() {
		return false
	}
	if !p.atName() {
		return true
	}
	next := p.peekN(1)
	if next.Kind == lexer.Keyword && wordBinaryOpKeywords[next.KeywordID] {
		return true
	}
	isDecl := next.Kind == lexer.Colon || next.Kind == lexer.Semicolon ||
		next.Kind == lexer.Keyword || next.Kind == lexer.LBracket ||
		next.Kind == lexer.LBrace ||
		beginsDeclarationTail(next, p.peekN(2))
	return !isDecl
}

// wordBinaryOpKeywords are the keyword binary operators (binaryOpFor); a name
// followed by one continues an expression, not a declaration.
var wordBinaryOpKeywords = map[string]bool{
	"implies": true,
	"or":      true,
	"xor":     true,
	"and":     true,
	"hastype": true,
	"istype":  true,
	"as":      true,
	"meta":    true,
}

// parseEnumBody parses an enumeration body, whose enumerated values may be
// anonymous (SysML.xtext EnumeratedValue): `= 60.0;`.
func (p *Parser) parseEnumBody() []ast.Node {
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		if p.at(lexer.Eq) || p.at(lexer.ColonEq) {
			start := p.peek().Span.Offset
			trivia := p.takeTrivia()
			u := &ast.Usage{Kind: ast.UsageEnumeration}
			p.acceptValueOperator()
			u.Value = p.ParseExpression()
			p.expect(lexer.Semicolon, "expected ';' after enumerated value")
			u.NodeSpan = p.spanFrom(start)
			m := &ast.Membership{Member: u}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			body.add(m)
			continue
		}
		body.add(p.parseBodyMember())
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return body.finish()
}

func (p *Parser) parseTypeFeatureMember(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	p.advance() // consume 'member'
	inner := p.parseDeclaration(start)
	if inner == nil {
		en := p.errorNodeSkip(start, "expected a body member after 'member'")
		en.SetLeadingTrivia(trivia)
		return en
	}
	m := &ast.Membership{
		Visibility:    vis,
		IsTypeFeature: true,
		Member:        inner,
	}
	m.NodeBase.NodeSpan = p.spanFrom(start)
	m.SetLeadingTrivia(trivia)
	return m
}

func (p *Parser) parseDisjointMember(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	p.advance() // consume 'disjoint'
	if p.atKeyword("from") {
		en := p.errorNodeSkip(start, "expected the disjoined type before 'from'")
		en.SetLeadingTrivia(trivia)
		return en
	}

	source := p.parseRelationshipTarget()
	if !p.acceptKeyword("from") {
		p.error(p.peek().Span, "expected 'from' after disjoint target")
	}
	target := p.parseRelationshipTarget()
	p.accept2(lexer.Semicolon)

	u := &ast.Usage{
		Kind: ast.UsagePart,
		Relationships: []*ast.Relationship{
			{Kind: ast.RelDisjoint, Target: source},
			{Kind: ast.RelDisjoint, Target: target},
		},
	}
	u.NodeBase.NodeSpan = p.spanFrom(start)
	u.SetLeadingTrivia(trivia)

	m := &ast.Membership{Visibility: vis, Member: u}
	m.NodeBase.NodeSpan = u.Span()
	m.SetLeadingTrivia(trivia)
	return m
}

// parseBodyMember parses one body member: an optional visibility prefix
// followed by a declaration (which may be a nested def/usage). Import/Alias
// carry their own visibility and are returned directly; other declarations are
// wrapped in a Membership. Mirrors parseMember.
func (p *Parser) parseBodyMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	// A member-attached `then` is taken by the body loop, which owns the member
	// list the succession it desugars to is synthesised into (see
	// succession.go). One reaching here belongs to a body that keeps no such
	// list, so there is nowhere to put the succession: report it rather than
	// parse the member with the keyword dropped, which would silently sequence
	// nothing.
	if p.atKeyword("then") {
		tok := p.advance()
		p.error(tok.Span, "`then` cannot prefix a member here: a succession sequences two members of a definition, usage, action, state, calculation or requirement body")
		return p.parseBodyMember()
	}

	// Check for `#MetadataType` prefix (user-defined keyword)
	// Parse prefixes and then parse def/usage declaration
	if p.at(lexer.Hash) {
		// Delegate to parseDefUsage which handles prefixes
		inner := p.parseDefUsage(start)
		if inner == nil {
			return nil
		}
		// Wrap in membership if not already wrapped
		if m, ok := inner.(*ast.Membership); ok {
			m.SetLeadingTrivia(trivia)
			return m
		}
		m := &ast.Membership{Visibility: vis, Member: inner}
		m.NodeSpan = p.spanFrom(start)
		m.SetLeadingTrivia(trivia)
		return m
	}

	// A metadata usage: `@Type;` or `@Type { prop = value; }`.
	if p.at(lexer.At) {
		pm := p.parseMetadataUsage(start)
		if pm == nil {
			return nil
		}
		pm.SetLeadingTrivia(trivia)
		m := &ast.Membership{Visibility: vis, Member: pm}
		m.NodeSpan = p.spanFrom(start)
		m.SetLeadingTrivia(trivia)
		return m
	}

	// A direction prefixes the feature it applies to, so a member that is only a
	// direction is that feature missing (SysML.xtext FeatureDirection).
	if dir := p.peek(); p.atDirectionKeyword() && p.peekN(1).Kind == lexer.Semicolon {
		p.advance() // consume the direction
		p.error(p.peek().Span, "expected a feature after '"+dir.KeywordID+"': write `"+dir.KeywordID+" <name> : <Type>`")
		p.advance() // consume ';'
		return nil
	}

	if p.atKeyword("import") {
		imp := p.parseImport(start, vis)
		imp.SetLeadingTrivia(trivia)
		return imp
	}
	if p.atKeyword("alias") {
		al := p.parseAlias(start, vis)
		al.SetLeadingTrivia(trivia)
		return al
	}
	if p.atTextualRepresentationStart() {
		inner := p.parseTextualRepresentation(start)
		m := &ast.Membership{Visibility: vis, Member: inner}
		m.NodeBase.NodeSpan = p.spanFrom(start)
		m.SetLeadingTrivia(trivia)
		return m
	}
	if p.atKeyword("member") {
		return p.parseTypeFeatureMember(start, vis, trivia)
	}

	// Check for timeslice usage keyword
	// Creates occurrence usage (temporal slice)
	if p.atKeyword("timeslice") {
		inner := p.parseDefUsage(start)
		if inner != nil {
			m := &ast.Membership{
				Visibility: vis,
				Member:     inner,
			}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			return m
		}
	}

	// Check for snapshot usage keyword
	// Creates occurrence usage (temporal instant)
	if p.atKeyword("snapshot") {
		inner := p.parseDefUsage(start)
		if inner != nil {
			m := &ast.Membership{
				Visibility: vis,
				Member:     inner,
			}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			return m
		}
	}

	// Check for behavioral statements in structural contexts (occurrence/part with temporal ordering)
	// These include: first/then succession edges for snapshot ordering
	if p.atKeyword("first") {
		firstTok := p.advance()
		return p.parseInitialNode(firstTok)
	}

	// Check for return statement (result member)
	// Can appear in calc body, constraint body, or requirement body
	if p.isResultKeyword() {
		return p.parseResultMember()
	}

	// Check for subset/disjoint constraint statements
	// Pattern: subset X subsets Y; OR disjoint X from Y;
	// These are anonymous features with relationships
	if p.atKeyword("disjoint") {
		return p.parseDisjointMember(start, vis, trivia)
	}
	if p.atRelationshipMember() {
		return p.parseRelationshipMember(start, vis, trivia)
	}
	if p.atKeyword("subset") {
		p.advance() // skip "subset" or "disjoint"

		// Parse first target (source)
		source := p.parseRelationshipTarget()

		// Pattern: subset X subsets Y;
		if !p.acceptKeyword("subsets") {
			p.error(p.peek().Span, "expected 'subsets' after subset source")
		}
		target := p.parseRelationshipTarget()
		p.accept2(lexer.Semicolon)

		u := &ast.Usage{
			Kind: ast.UsagePart, // Generic feature
			Relationships: []*ast.Relationship{
				{
					Kind:   ast.RelSubsets,
					Target: source,
				},
				{
					Kind:   ast.RelSubsets,
					Target: target,
				},
			},
		}
		u.NodeBase.NodeSpan = p.spanFrom(start)
		u.SetLeadingTrivia(trivia)

		m := &ast.Membership{
			Visibility: vis,
			Member:     u,
		}
		m.NodeBase.NodeSpan = u.Span()
		m.SetLeadingTrivia(trivia)
		return m
	}

	// Check for expose statement: expose <path>[::*|::**][filter];
	// Per SysML v2 8.3.26.2, an Expose is an Import: MembershipExpose
	// specializes MembershipImport and NamespaceExpose specializes
	// NamespaceImport, so the wildcard tail selects the import kind exactly as
	// it does for `import`. An Expose always imports all elements regardless of
	// visibility (isImportAll = true) and always has protected visibility.
	if p.atKeyword("expose") {
		p.advance() // consume 'expose'

		path := p.parseQualifiedName()
		if path == nil {
			p.error(p.peek().Span, "expected namespace path after 'expose'")
			return &ast.ErrorNode{Message: "expected namespace path"}
		}

		imp := &ast.Import{
			Visibility: ast.VisibilityProtected,
			IsAll:      true,
			Kind:       ast.ImportMembership,
			Imported:   path,
			IsExpose:   true,
		}
		p.parseImportTail(imp)
		imp.NodeBase.NodeSpan = p.spanFrom(start)
		imp.SetLeadingTrivia(trivia)

		p.expect(lexer.Semicolon, "expected ';' after expose statement")

		return imp
	}

	// An accept node (SysML.xtext `AcceptNode`):
	//
	//	action <name>? accept <payload> ('via' <port>)? (';' | '{' … '}')
	//
	// The payload says what the action waits for — a type (`accept scene :
	// Scene`), an event feature (`accept :> shutDown`) or a trigger expression
	// (`accept when x > 1`) — and is parsed by the one payload parser triggers
	// also use, so every spelling reaches lowering the same way.
	if p.atAcceptNode() {
		return p.parseAcceptNode(start, vis, trivia)
	}

	// A transition usage stating its ends (SysML.xtext `TransitionUsage`):
	//
	//	transition <name>? first <source> … then <target>;
	//	transition <name>? <source> to <target> …;
	//
	// One parser reads every transition, wherever it is written, so a named
	// transition carries the same trigger, guard and effect a nameless one does
	// and lowering sees one representation. A `transition` declaring no ends
	// (`transition t : Signalling;`) is an ordinary usage and parsed below.
	if p.atKeyword("transition") && p.atTransitionEnds() {
		p.advance() // consume 'transition'
		node := p.parseTransitionMember(start)
		if tr, ok := node.(interface{ SetLeadingTrivia([]ast.Trivia) }); ok {
			tr.SetLeadingTrivia(trivia)
		}
		m := &ast.Membership{Visibility: vis, Member: node}
		m.NodeSpan = node.Span()
		m.SetLeadingTrivia(trivia)
		return m
	}

	// Check for anonymous feature pattern: [modifiers] [name] : Type OR [modifiers] :>> relationships
	// Examples: private thisClock : Clock :>> self; or ref stateSpace: StateSpace; or ref :>> x
	// This handles features with visibility but no usage kind keyword
	nextKind := p.peekN(1).Kind

	// Check for (visibility OR modifier) + (name + colon OR relationship) pattern
	hasVisibility := vis != ast.VisibilityDefault
	hasModifier := p.atKeyword("ref") || p.atKeyword("readonly") || p.atKeyword("derived") || p.atKeyword("composite") || p.atKeyword("portion") || p.atKeyword("end")

	if hasVisibility || hasModifier {
		mods := p.parseFeatureModifiers()
		// Merge visibility into mods if it was parsed earlier
		if hasVisibility {
			mods.visibility = vis
		}

		// An `end` with no declaration at all is an interface body's default
		// end (SysML v2 8.2.2.14 DefaultInterfaceEnd, `isEnd ?= 'end' Usage`
		// over an optional UsageDeclaration).
		if mods.isEnd && (p.at(lexer.Semicolon) || p.at(lexer.LBrace)) {
			return p.parseAnonymousEnd(start, trivia, vis, mods)
		}

		// Special case: end shortname [mult] feature name pattern
		// Example: end self2 [1] feature sameThing: Anything
		// Also: end [1] feature transferSource (no short name)
		// Also: end ref source; (no definition keyword, just anonymous feature)
		// This declares a feature with 'end' modifier, optional short name, and multiplicity
		// Prefix metadata may stand where the kind keyword would, after the
		// modifiers (SysML.xtext ExtendedUsage): `end #original r1 : Req1;`.
		if p.at(lexer.Hash) {
			if inner := p.parseDefUsage(start); inner != nil {
				applyFeatureMods(inner, mods)
				m := &ast.Membership{Visibility: vis, Member: inner}
				m.NodeSpan = p.spanFrom(start)
				m.SetLeadingTrivia(trivia)
				return m
			}
		}

		if mods.isEnd && (p.atNameOrKeyword() || p.at(lexer.LBracket)) {
			var shortName string
			var shortNameSpan source.Span
			var mult *ast.Multiplicity
			var hasDefKeyword bool
			var endRels []*ast.Relationship // relationships parsed before definition keyword

			if p.isKindKeyword(p.peek()) {
				// `end [1] part bead : TireBead` — the kind keyword follows the
				// modifiers directly, so there is no short name, and the
				// multiplicity was already taken as an early one.
				mult = mods.earlyMultiplicity
				hasDefKeyword = true
			} else if p.atNameOrKeyword() {
				// Check if pattern matches: name [mult] (feature|occurrence|item|...)
				// OR: [mult] (feature|occurrence|...)
				// Also: name [mult] subsets X feature name (with relationship clause)
				ahead := 1
				if p.peekN(ahead).Kind == lexer.LBracket {
					// Skip past multiplicity to check for definition keyword
					ahead++
					for ahead < 20 && p.peekN(ahead).Kind != lexer.RBracket && p.peekN(ahead).Kind != lexer.EOF {
						ahead++
					}
					if p.peekN(ahead).Kind == lexer.RBracket {
						ahead++ // past ]
					}
				}

				// Skip optional relationship clauses before definition keyword
				// Pattern: end name[mult] subsets X feature Y
				for ahead < 40 {
					tok := p.peekN(ahead)
					if tok.Kind == lexer.EOF {
						break
					}
					// Check if this is a relationship keyword
					isRelKeyword := false
					if tok.Kind == lexer.Keyword {
						_, isRelKeyword = relationshipKeywords[tok.KeywordID]
						if !isRelKeyword && (tok.KeywordID == "defined" || tok.KeywordID == "inverse") {
							isRelKeyword = true
						}
						if !isRelKeyword && tok.KeywordID == "typed" && p.peekN(ahead+1).KeywordID == "by" {
							isRelKeyword = true
						}
					}
					if isRelKeyword {
						// Skip relationship keyword
						ahead++
						// Skip potential "of" after "inverse"
						if p.peekN(ahead).KeywordID == "of" {
							ahead++
						}
						// Skip relationship target (identifier/qualified name)
						for ahead < 40 {
							t := p.peekN(ahead)
							// Stop if we hit a definition or usage keyword
							if t.Kind == lexer.Keyword {
								_, isDef := p.definitionKind(t.KeywordID)
								_, isUsage := p.usageKind(t.KeywordID)
								if isDef || isUsage {
									break
								}
							}
							if t.Kind == lexer.Identifier || t.Kind == lexer.Keyword || t.Kind == lexer.Dot || t.Kind == lexer.ColonColon {
								ahead++
							} else {
								break
							}
						}
						// Skip comma for multiple targets
						if p.peekN(ahead).Kind == lexer.Comma {
							ahead++
						} else {
							break // no more relationship clauses
						}
					} else {
						break // not a relationship keyword
					}
				}

				// Check if next token after (optional) multiplicity and relationships is a definition or usage keyword
				nextTok := p.peekN(ahead)
				isDefKeyword := false
				if nextTok.Kind == lexer.Keyword {
					_, isDef := p.definitionKind(nextTok.KeywordID)
					_, isUsage := p.usageKind(nextTok.KeywordID)
					isDefKeyword = isDef || isUsage
				}

				if isDefKeyword {
					tok := p.advance()
					if tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName || tok.Kind == lexer.Keyword {
						shortName = p.src.Text(tok.Span)
						shortNameSpan = tok.Span
					}

					// Parse optional multiplicity before the definition keyword
					if p.at(lexer.LBracket) {
						mult = p.parseMultiplicity()
					}

					// Parse optional relationship clauses before definition keyword
					// Pattern: end shortname[mult] subsets X feature Y
					for p.atRelationshipKeyword() {
						rel := p.parseRelationships(true)
						endRels = append(endRels, rel...)
					}

					hasDefKeyword = true
				}
			} else if p.at(lexer.LBracket) {
				// No short name, mult comes directly: end [mult] feature
				// Also: end [mult] subsets X feature (with relationship)
				// Check if after mult there's a definition keyword
				ahead := 1
				for ahead < 20 && p.peekN(ahead).Kind != lexer.RBracket && p.peekN(ahead).Kind != lexer.EOF {
					ahead++
				}
				if p.peekN(ahead).Kind == lexer.RBracket {
					ahead++ // past ]

					// Skip optional relationship clauses before definition keyword
					for ahead < 40 {
						tok := p.peekN(ahead)
						if tok.Kind == lexer.EOF {
							break
						}
						isRelKeyword := false
						if tok.Kind == lexer.Keyword {
							_, isRelKeyword = relationshipKeywords[tok.KeywordID]
							if !isRelKeyword && (tok.KeywordID == "defined" || tok.KeywordID == "inverse") {
								isRelKeyword = true
							}
							if !isRelKeyword && tok.KeywordID == "typed" && p.peekN(ahead+1).KeywordID == "by" {
								isRelKeyword = true
							}
						}
						if isRelKeyword {
							ahead++
							if p.peekN(ahead).KeywordID == "of" {
								ahead++
							}
							for ahead < 40 {
								t := p.peekN(ahead)
								// Stop if we hit a definition or usage keyword
								if t.Kind == lexer.Keyword {
									_, isDef := p.definitionKind(t.KeywordID)
									_, isUsage := p.usageKind(t.KeywordID)
									if isDef || isUsage {
										break
									}
								}
								if t.Kind == lexer.Identifier || t.Kind == lexer.Keyword || t.Kind == lexer.Dot || t.Kind == lexer.ColonColon {
									ahead++
								} else {
									break
								}
							}
							if p.peekN(ahead).Kind == lexer.Comma {
								ahead++
							} else {
								break
							}
						} else {
							break
						}
					}

					nextTok := p.peekN(ahead)
					isDefKeyword := false
					if nextTok.Kind == lexer.Keyword {
						_, isDef := p.definitionKind(nextTok.KeywordID)
						_, isUsage := p.usageKind(nextTok.KeywordID)
						isDefKeyword = isDef || isUsage
					}

					if isDefKeyword {
						mult = p.parseMultiplicity()

						// Parse optional relationship clauses before definition keyword
						for p.atRelationshipKeyword() {
							rel := p.parseRelationships(true)
							endRels = append(endRels, rel...)
						}

						hasDefKeyword = true
					}
				}
			}

			// If we found a definition keyword, parse the full declaration
			if hasDefKeyword {
				// Now parse the actual feature/usage declaration
				// The definition keyword (feature/occurrence/etc) will be consumed by parseDeclaration
				decl := p.parseDeclaration(start)

				// If it's a usage, apply the short name, multiplicity, relationships, and end modifier
				if u, ok := decl.(*ast.Usage); ok {
					// `end part <b> bead : T` declares its short name after the
					// kind keyword, so parseDeclaration already took it.
					if shortName != "" {
						u.Ident.ShortName = shortName
						u.Ident.ShortNameSpan = shortNameSpan
					}
					if mult != nil && u.Multiplicity == nil {
						u.Multiplicity = mult
					}
					// Prepend relationships parsed before definition keyword
					if len(endRels) > 0 {
						u.Relationships = append(endRels, u.Relationships...)
					}
					u.IsEnd = true
					u.Visibility = mods.visibility
				}

				// Wrap in membership
				mem := &ast.Membership{Visibility: vis, Member: decl}
				mem.NodeSpan = p.spanFrom(start)
				mem.SetLeadingTrivia(trivia)
				return mem
			}
			// If no definition keyword found, fall through to handle as anonymous feature with modifiers
			// Pattern: end ref name; - will be handled by anonymous feature parsing below
		}

		// A feature modifier can precede the kind keyword: `ref part a : V`,
		// `composite item i`, `derived attribute c`. The declaration is parsed
		// as usual and the modifiers already consumed are applied to it.
		if p.isKindKeyword(p.peek()) || p.atKindPrefix() {
			// A keyword that only qualifies the kind after it is consumed first:
			// `derived var feature x` declares a feature, not a `var`.
			for p.atKindPrefix() && !p.isKindKeyword(p.peek()) {
				// A prefix saying what the declaration is for is part of it, whether
				// or not a modifier was written before it (`derived var feature x`).
				if w := p.kindPrefixWord(); kindPrefixKeywords[w] {
					mods.prefixKeyword = w
				}
				if p.kindPrefixWord() == varPrefixWord {
					mods.isVariable = true
				}
				p.advance()
			}
			decl := p.parseDeclaration(start)
			if decl == nil {
				en := p.errorNodeSkip(start, "expected a declaration after a feature modifier")
				en.SetLeadingTrivia(trivia)
				return en
			}
			applyFeatureMods(decl, mods)
			mem := &ast.Membership{Visibility: mods.visibility, Member: decl}
			mem.NodeSpan = p.spanFrom(start)
			mem.SetLeadingTrivia(trivia)
			return mem
		}

		// Check for name + colon (typed) OR direct relationship (anonymous) OR name + relationship OR name + semicolon OR name + multiplicity
		hasNameAndType := p.atName() && p.peekN(1).Kind == lexer.Colon
		hasRelationship := p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) || p.at(lexer.ColonColonGt) || p.atRelationshipKeyword()
		hasNameAndRelationship := p.atName() && (p.peekN(1).Kind == lexer.ColonGt || p.peekN(1).Kind == lexer.ColonGtGt || p.peekN(1).Kind == lexer.ColonColonGt)
		hasNameOnly := p.atName() && (p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.RBrace)
		hasNameAndBody := p.atName() && p.peekN(1).Kind == lexer.LBrace
		hasNameAndMult := p.atName() && p.peekN(1).Kind == lexer.LBracket // name with multiplicity (e.g., ref payload [0..*])
		// `end [1] : A;` — an unnamed feature declaring only its type.
		hasTypeOnly := p.at(lexer.Colon)

		if hasNameAndType || hasTypeOnly || hasRelationship || hasNameAndRelationship || hasNameOnly || hasNameAndBody || hasNameAndMult {
			var id ast.Identification

			// Parse optional name
			if hasNameAndType || hasNameAndRelationship || hasNameOnly || hasNameAndBody || hasNameAndMult {
				tok := p.advance()
				if p.nameToken(tok) {
					id.Name = p.src.Text(tok.Span)
					id.NameSpan = tok.Span
				}
				if hasNameAndType {
					p.advance() // consume ':'
				}
			}

			// Parse as anonymous usage (attribute by default)
			u := &ast.Usage{
				Kind:        p.anonymousUsageKind(mods),
				Ident:       id,
				Visibility:  mods.visibility,
				IsReference: mods.isReference,
				IsVariable:  mods.isVariable,
				IsDerived:   mods.isDerived,
				IsComposite: mods.isComposite,
				IsEnd:       mods.isEnd,
				IsChain:     mods.isChain,
				Direction:   mods.direction,
				IsOrdered:   mods.isOrdered,
				IsNonunique: mods.isNonunique,
			}

			if hasTypeOnly {
				p.advance() // consume ':'
			}

			// If we consumed a colon, parse typing relationship(s)
			// Support comma-separated types: : Type1, Type2, Type3
			if hasNameAndType || hasTypeOnly {
				u.Relationships = append(u.Relationships, p.parseTypingRelationships()...)
			}

			// Parse optional multiplicity
			if p.at(lexer.LBracket) {
				u.Multiplicity = p.parseMultiplicity()
			}
			if u.Multiplicity == nil {
				// `end [1] rim : Rim` — the multiplicity was taken as an early one.
				u.Multiplicity = mods.earlyMultiplicity
			}

			// Parse post-multiplicity modifiers (ordered/nonunique)
			postMods := p.parsePostModifiers()
			if postMods.isOrdered {
				u.IsOrdered = true
			}
			if postMods.isNonunique {
				u.IsNonunique = true
			}

			// Parse additional relationships
			u.Relationships = append(u.Relationships, p.parseRelationships(true)...)

			// A multiplicity may follow the specializations instead of preceding
			// them (KerML.xtext FeatureSpecializationPart): `ref redefines x[4];`.
			if u.Multiplicity == nil && p.at(lexer.LBracket) {
				u.Multiplicity = p.parseMultiplicity()
				if post := p.parsePostModifiers(); post.isOrdered || post.isNonunique {
					u.IsOrdered = u.IsOrdered || post.isOrdered
					u.IsNonunique = u.IsNonunique || post.isNonunique
				}
			}

			// Parse optional value (= expr or default expr)
			if p.acceptValueOperator() {
				u.Value = p.ParseExpression()
			}

			// Parse body or semicolon
			members, hasBody := p.parseDefUsageBody()
			u.Members = members
			u.HasBody = hasBody

			u.NodeSpan = p.spanFrom(start)
			mem := &ast.Membership{Visibility: vis, Member: u}
			mem.NodeSpan = p.spanFrom(start)
			mem.SetLeadingTrivia(trivia)
			return mem
		}
		// If not anonymous feature pattern, fallback to parseDeclaration below
	}

	// Check for anonymous feature pattern without modifiers: name : Type
	if p.atName() && nextKind == lexer.Colon {
		var id ast.Identification
		tok := p.advance()
		if p.nameToken(tok) {
			id.Name = p.src.Text(tok.Span)
			id.NameSpan = tok.Span
		}

		// Parse as anonymous usage (attribute by default)
		u := &ast.Usage{
			Kind:  ast.UsageAttribute,
			Ident: id,
		}

		// Parse typing/relationships
		p.advance() // consume ':'
		u.Relationships = append(u.Relationships, p.parseTypingRelationships()...)

		// Parse optional multiplicity
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
		}

		// Parse post-multiplicity modifiers (ordered/nonunique)
		postMods := p.parsePostModifiers()
		if postMods.isOrdered {
			u.IsOrdered = true
		}
		if postMods.isNonunique {
			u.IsNonunique = true
		}

		// Parse additional relationships
		u.Relationships = append(u.Relationships, p.parseRelationships(true)...)

		// Parse optional value (= expr or default expr)
		if p.acceptValueOperator() {
			u.Value = p.ParseExpression()
		}

		// Parse body or semicolon
		members, hasBody := p.parseDefUsageBody()
		u.Members = members
		u.HasBody = hasBody

		u.NodeSpan = p.spanFrom(start)
		mem := &ast.Membership{Visibility: vis, Member: u}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

	// Check for enum literal pattern: identifier = expr; OR identifier; OR identifier { body }
	// Examples: low = 0.25; or pass; or open { doc } or done { doc } (keyword as name)
	// But exclude usage-only keywords (inv, subject, etc.) - they're declarations, not enum literal names
	// Also exclude constraint (has both def/usage forms but shouldn't be enum literal name)
	isUsageOnlyKwForEnum := p.at(lexer.Keyword) && (p.peek().KeywordID == "subject" || p.peek().KeywordID == "objective" ||
		p.peek().KeywordID == "succession" || p.peek().KeywordID == "inv" || p.peek().KeywordID == "connector" ||
		// `connect` introduces a connection usage's ends, so `connect;` is that
		// usage missing them rather than a literal named `connect`.
		p.peek().KeywordID == "connect" ||
		p.peek().KeywordID == "satisfy" || p.peek().KeywordID == "verify" || p.peek().KeywordID == "step" || p.peek().KeywordID == "expr" || p.peek().KeywordID == "constraint" ||
		p.peek().KeywordID == "interaction" || p.peek().KeywordID == "bool" || p.peek().KeywordID == "assoc" || p.peek().KeywordID == "struct" ||
		p.peek().KeywordID == "class" || p.peek().KeywordID == "predicate" ||
		// A view or viewpoint member keyword names no literal either: `render;`
		// and `frame;` are members missing the reference they are written with
		// (ViewRenderingUsage, FramedConcernUsage), diagnosed as such.
		p.peek().KeywordID == "render" || p.peek().KeywordID == "frame" ||
		// `include` states the use case it includes, so `include;` is that
		// member missing its reference (SysML.xtext IncludeUseCaseUsage).
		p.peek().KeywordID == "include" ||
		p.peek().KeywordID == "stakeholder" || p.peek().KeywordID == "actor")
	// A relationship keyword is not a literal's name either: `redefines;` and
	// `redefines = 5;` are specializations missing their target, diagnosed as
	// such, exactly as `:>>;` and `:>> = 5;` are.
	// A kind keyword before a body declares an anonymous usage of that kind
	// (`action { … }`), not a literal named by it: a name spelling a reserved
	// keyword must be written as an unrestricted name (KerML §7.2.4).
	anonKindDecl := nextKind == lexer.LBrace && p.isKindKeyword(p.peek())
	if !isUsageOnlyKwForEnum && !anonKindDecl && !p.atFeatureSpecialization() && p.atNameOrKeyword() && (nextKind == lexer.Eq || nextKind == lexer.Semicolon || nextKind == lexer.LBrace) {
		seg, _ := p.parseNameSegmentRelaxed()
		id := ast.Identification{Name: seg.Text, NameSpan: seg.Span}

		var value ast.Node
		if p.at(lexer.Eq) {
			p.advance() // consume '='
			value = p.ParseExpression()
		}

		// Parse body or semicolon
		members, hasBody := p.parseDefUsageBody()

		u := &ast.Usage{
			Kind:    ast.UsageEnumeration,
			Ident:   id,
			Value:   value,
			Members: members,
			HasBody: hasBody,
		}
		u.NodeSpan = p.spanFrom(start)

		mem := &ast.Membership{Visibility: vis, Member: u}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

	// A keyword before a kind keyword qualifies the declaration rather than
	// naming it (`var feature x`, `assert constraint { ... }`), so it is consumed
	// and the declaration keeps the name it declares for itself. A keyword that
	// is not such a prefix is the declaration's own kind, and the keyword after
	// it is its name (`action flow { ... }` is an action named `flow`); that is
	// parsed below, which reads the name instead of dropping it.
	if p.atKindPrefix() {
		prefix := p.kindPrefixWord()
		p.advance() // consume the prefix keyword
		inner := p.parseDeclaration(start)
		if inner == nil {
			en := p.errorNodeSkip(start, "expected a body member")
			en.SetLeadingTrivia(trivia)
			return en
		}
		// A prefix that says what the declaration is for is part of it
		// (`assert constraint c` is an AssertConstraintUsage).
		if u, ok := inner.(*ast.Usage); ok && kindPrefixKeywords[prefix] && u.PrefixKeyword == "" {
			u.PrefixKeyword = prefix
		}
		if u, ok := inner.(*ast.Usage); ok && prefix == varPrefixWord {
			u.IsVariable = true
		}
		mem := &ast.Membership{Visibility: vis, Member: inner}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

	// Check for name-before-keyword pattern: <name> <keyword> { ... }
	// Example: myConstraint constraint { ... }
	if p.atName() {
		next := p.peekN(1)
		if next.Kind == lexer.Keyword {
			_, isDef := p.definitionKind(next.KeywordID)
			_, isUsage := p.usageKind(next.KeywordID)
			if isDef || isUsage {
				// Parse as named usage: consume name token, then proceed with keyword
				var id ast.Identification
				tok := p.advance()
				id.Name = p.src.Text(tok.Span)
				id.NameSpan = tok.Span
				inner := p.parseDeclaration(start)
				if u, ok := inner.(*ast.Usage); ok {
					u.Ident = id
				} else if d, ok := inner.(*ast.Definition); ok {
					d.Ident = id
				}
				if inner == nil {
					en := p.errorNodeSkip(start, "expected a body member")
					en.SetLeadingTrivia(trivia)
					return en
				}
				mem := &ast.Membership{Visibility: vis, Member: inner}
				mem.NodeSpan = p.spanFrom(start)
				mem.SetLeadingTrivia(trivia)
				return mem
			}
		}
	}

	inner := p.parseDeclaration(start)
	if inner == nil {
		en := p.errorNodeSkip(start, p.noBodyMemberMessage())
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: inner}
	mem.NodeSpan = p.spanFrom(start)
	mem.SetLeadingTrivia(trivia)
	return mem
}

// noBodyMemberMessage reports why the current token starts no body member,
// naming the missing declaration when a relationship keyword that is not a
// feature specialization stands in place of one (SysML.xtext
// SubclassificationPart, TypeRelationshipPart, FeatureRelationshipPart).
func (p *Parser) noBodyMemberMessage() string {
	const base = "expected a body member"
	t := p.peek()
	if t.Kind != lexer.Keyword {
		return base
	}
	switch t.KeywordID {
	case "specializes":
		return base + ": 'specializes' relates two types; a member refines an inherited feature by subsetting it, written 'subsets' or ':>'"
	case "unions", "intersects", "chains", "inverse", "featured":
		return base + ": '" + t.KeywordID + "' relates the declaration written before it, so a member cannot begin with it"
	}
	return base
}

// parsePerformedActionReference parses the reference form of a performed action
// usage — a feature reference with an optional body — which SysML.xtext spells
// `OwnedReferenceSubsetting FeatureSpecializationPart? ValuePart?` followed by
// an `ActionBody` (PerformActionUsageDeclaration; SysML v2 §7.17.6). The keyword
// that introduced it is already consumed; kw carries it for errors and to record
// which synonym was written.
func (p *Parser) parsePerformedActionReference(start int, mods featureMods, kw string) *ast.Usage {
	return p.parseReferenceMemberUsage(start, ast.UsageAction, kw, "action", mods, p.parseActionBodyMixed, true)
}

// atMemberKeywordUsedAsKeyword reports whether the `frame` or `render` token at
// the cursor introduces a member rather than being the name of the declaration
// it starts. Both member forms continue with the referenced name or with the
// notation's own kind keyword (SysML.xtext ViewRenderingUsage,
// FramedConcernUsage), so anything else — a multiplicity, a specialization, a
// type, a body — can only follow a name, and `frame` and `render` are legal
// names in KerML.
func (p *Parser) atMemberKeywordUsedAsKeyword(kw string) bool {
	// SysML.xtext holds both literals, so neither can name a declaration there;
	// only KerML, which has neither, reads them as names.
	if p.src.Kind() != source.KindKerML {
		return true
	}
	next := p.peekN(1)
	switch next.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return true
	case lexer.Keyword:
		if kw == "frame" {
			return next.KeywordID == "concern"
		}
		return next.KeywordID == "rendering"
	}
	return false
}

// parseReferenceMemberUsage parses the reference form that SysML.xtext spells
// `ownedRelationship += OwnedReferenceSubsetting FeatureSpecializationPart?
// ValuePart?` followed by the member's body: a performed action
// (PerformActionUsageDeclaration), the rendering a view names
// (ViewRenderingUsage) and the concern a requirement frames
// (FramedConcernUsage) are all written that way. Such a member names an
// existing feature and declares no name of its own, so the reference is
// recorded as a ReferenceSubsetting and the identification left empty; the name
// the member answers to is its reference's (KerML 7.3.4.5, ast.EffectiveName).
//
// The introducing keyword is already consumed; kw records which synonym was
// written, noun names the referenced element in diagnostics, parseBody parses
// the body members once '{' is consumed, and allowValue states whether the
// notation admits a `ValuePart`: a performed action does, the rendering a view
// names and the concern a requirement frames do not.
func (p *Parser) parseReferenceMemberUsage(start int, kind ast.UsageKind, kw, noun string, mods featureMods, parseBody func() []ast.Node, allowValue bool) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		Keyword:     kw,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		IsVariable:  mods.isVariable,
		IsEnd:       mods.isEnd,
		Visibility:  mods.visibility,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
	}
	u.NodeBase.NodeSpan = p.spanFrom(start)

	var target ast.Node
	if p.atNameOrKeyword() {
		target = p.parseRelationshipTarget()
	}
	if target != nil {
		u.Relationships = append(u.Relationships, &ast.Relationship{
			Kind:   ast.RelReferences,
			Target: target,
		})
	} else {
		p.error(p.peek().Span, fmt.Sprintf("expected a %s reference after '%s'", noun, kw))
	}

	// FeatureSpecializationPart? ValuePart?
	if p.at(lexer.LBracket) {
		u.Multiplicity = p.parseMultiplicity()
	}
	specRels := p.parseRelationships(true)
	u.Relationships = append(u.Relationships, specRels...)
	if allowValue && p.acceptValueOperator() {
		u.Value = p.ParseExpression()
	}

	switch {
	case p.atTransitionEffectStatement(start) && p.atEffectEnd():
		// A performed action written as a transition's effect is ended by the
		// transition's next clause or by the ';' the transition itself consumes.
		u.HasBody = false
	case p.accept2(lexer.Semicolon):
		u.HasBody = false
	case p.at(lexer.LBrace):
		p.advance()
		u.Members = parseBody()
		u.HasBody = true
	case allowValue && (p.atKeyword("then") || p.atKeyword("if") || p.atKeyword("do")):
		// A performed action written as a transition's effect is terminated by
		// the transition's next clause: `do perform notify then idle;`.
		u.HasBody = false
	case target != nil:
		p.error(p.peek().Span, fmt.Sprintf("expected ';' or '{' after '%s' %s reference", kw, noun))
	}

	u.NodeSpan = p.spanFrom(start)
	return u
}

// bindingEnd records the first end of a binding connector. A connector end
// reference-subsets the feature it names (KerML OwnedReferenceSubsetting), so
// it resolves outside the connector rather than as an inherited redefinition.
func bindingEnd(target ast.Node) *ast.Relationship {
	return &ast.Relationship{Kind: ast.RelReferences, Target: target}
}

// hasBindingEnd reports whether a binding already states an end.
func hasBindingEnd(u *ast.Usage) bool {
	for _, r := range u.Relationships {
		if r.Kind == ast.RelReferences {
			return true
		}
	}
	return false
}

func normalizeAnonymousBindingEnd(u *ast.Usage) *ast.Usage {
	if u == nil || u.Kind != ast.UsageBinding || u.Ident.Name == "" || hasBindingEnd(u) {
		return u
	}
	target := &ast.QualifiedName{
		Parts: []ast.NameSegment{{Text: u.Ident.Name, Span: u.Ident.NameSpan}},
	}
	target.NodeSpan = u.Ident.NameSpan
	u.Relationships = append(u.Relationships, bindingEnd(target))
	u.Ident = ast.Identification{}
	return u
}

// parseRelationshipTarget parses a relationship target which can be either:
// - A qualified name (A::B::C)
// - A feature chain (A.B.C or A::B.C.D - mix of :: and .)
// Returns Node interface (either *QualifiedName or *FeatureChainExpr).
// Does NOT consume body expressions ({ in ... }) unlike ParseExpression().
func (p *Parser) parseRelationshipTarget() ast.Node {
	start := p.peek().Span.Offset

	// Start with qualified name (handles A::B::C)
	// Use parseQualifiedNameRelaxed to allow keywords like "do" in feature chains (e.g., do.startShot)
	base := p.parseQualifiedNameRelaxed()
	if base == nil {
		return nil
	}

	// Check for dot extensions (feature chain)
	if !p.at(lexer.Dot) {
		return base // Just a qualified name
	}

	// Build feature chain expression
	var operand ast.Node = &ast.FeatureReference{Name: base}
	operand.(*ast.FeatureReference).NodeSpan = base.NodeSpan

	for p.at(lexer.Dot) {
		p.advance() // consume '.'
		// Each chaining feature of a chain is itself a qualified name
		// (KerML OwnedFeatureChaining: chainingFeature = [Feature|QualifiedName]).
		if !p.atNameOrKeyword() && !(p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon) {
			p.error(p.peek().Span, "expected a name after '.'")
			break
		}
		memberName := p.parseQualifiedNameRelaxed()
		if memberName == nil {
			break
		}

		chain := &ast.FeatureChainExpr{
			Operand: operand,
			Member:  memberName,
		}
		chain.NodeSpan = p.spanFrom(start)
		operand = chain
	}

	return operand
}

// atFeatureChainTarget reports whether an exhibit target contains a dot
// chaining part, including qualified prefixes such as `P::states.on`.
func (p *Parser) atFeatureChainTarget() bool {
	if !p.atNameOrKeyword() {
		return false
	}
	for i := 1; ; i += 2 {
		separator := p.peekN(i).Kind
		if separator == lexer.Dot {
			return true
		}
		next := p.peekN(i + 1)
		if separator != lexer.ColonColon ||
			(next.Kind != lexer.Identifier && next.Kind != lexer.UnrestrictedName &&
				next.Kind != lexer.Keyword) {
			return false
		}
	}
}

// parsePreNameRelationships parses the specializations a declaration may state
// where its name would go. A word the grammar does not reserve names the
// declaration instead, so only a reserved spelling begins a clause here.
func (p *Parser) parsePreNameRelationships(isUsage bool) []*ast.Relationship {
	if t := p.peek(); t.Kind == lexer.Keyword && !p.reservedWord(t.KeywordID) {
		return nil
	}
	return p.parseRelationships(isUsage)
}

// parseRelationships parses zero or more relationship clauses. isUsage selects
// the meaning of the symbolic `:>` operator (subsets on a usage, specializes on
// a definition). Each clause may carry a comma-separated target list; every
// target becomes its own Relationship sharing the clause kind.
func (p *Parser) parseRelationships(isUsage bool) (rels []*ast.Relationship) {
	for {
		if p.atDeclarationConjugation() {
			rels = append(rels, p.parseDeclarationConjugation())
			continue
		}
		kind, ok := p.relationshipClauseKind(isUsage)
		if !ok {
			return rels
		}
		for {
			r := p.parseRelationshipClauseTarget(kind)
			rels = append(rels, r)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
	}
}

// anonymousUsageKind returns the kind of a usage declared without a kind
// keyword. An interface body's default end is a port usage (SysML v2 8.2.2.14
// DefaultInterfaceEnd); anywhere else the kind-less form is a reference usage,
// which this parser represents as an attribute usage.
func (p *Parser) anonymousUsageKind(mods featureMods) ast.UsageKind {
	if mods.isEnd && p.bodyContext() == bodyInterface {
		return ast.UsagePort
	}
	return ast.UsageAttribute
}

// parseAnonymousEnd parses an `end` whose declaration is omitted entirely
// (`end ;`, `end { ... }`) as a body member.
func (p *Parser) parseAnonymousEnd(start int, trivia []ast.Trivia, vis ast.Visibility, mods featureMods) ast.Node {
	node := p.parseAnonymousEndUsage(start, mods)
	if en, ok := node.(*ast.ErrorNode); ok {
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: node}
	mem.NodeSpan = node.Span()
	mem.SetLeadingTrivia(trivia)
	return mem
}

// parseAnonymousEndUsage parses an `end` whose declaration is omitted entirely.
// Only an interface body's default end (SysML v2 8.2.2.14.1) and an explicit
// `end ref` (8.2.2.7.2 ReferenceUsage) may omit it.
func (p *Parser) parseAnonymousEndUsage(start int, mods featureMods) ast.Node {
	kind := ast.UsageAttribute
	switch {
	case p.bodyContext() == bodyInterface:
		kind = ast.UsagePort
	case mods.isReference:
	default:
		return p.errorNodeSkip(start,
			"this `end` must declare a name, type or `ref` (write `end ref;`): only an interface body may declare a bare `end;` (SysML v2 8.2.2.14.1 DefaultInterfaceEnd)")
	}
	u := &ast.Usage{
		Kind:         kind,
		IsEnd:        true,
		Visibility:   mods.visibility,
		IsReference:  mods.isReference,
		IsDerived:    mods.isDerived,
		IsComposite:  mods.isComposite,
		Direction:    mods.direction,
		Multiplicity: mods.earlyMultiplicity,
	}
	u.Members, u.HasBody = p.parseDefUsageBody()
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseTypingRelationships parses the comma-separated target list of a typing
// clause whose ':' has already been consumed.
func (p *Parser) parseTypingRelationships() []*ast.Relationship {
	var rels []*ast.Relationship
	for {
		rels = append(rels, p.parseRelationshipClauseTarget(ast.RelTyping))
		if !p.accept2(lexer.Comma) {
			return rels
		}
	}
}

// parseRelationshipClauseTarget parses one target of a relationship clause,
// including the `~` of a conjugated port typing (SysML v2 8.2.2.12).
func (p *Parser) parseRelationshipClauseTarget(kind ast.RelationshipKind) *ast.Relationship {
	start := p.peek().Span.Offset
	tildeTok, conjugated := p.accept(lexer.Tilde)
	if conjugated && kind != ast.RelTyping {
		p.error(tildeTok.Span, "'~' conjugates a port type and is only allowed after ':' or 'defined by'")
	}
	// Handles both qualified names and feature chains, but not body expressions.
	target := p.parseRelationshipTarget()
	if target == nil && conjugated {
		p.error(p.peek().Span, "expected a port definition name after '~'")
	}
	r := &ast.Relationship{Kind: kind, Target: target, Conjugated: conjugated && kind == ast.RelTyping}
	r.NodeSpan = p.spanFrom(start)
	return r
}

// parseTierBEnds parses the distinctive Tier B usage grammar following the
// declaration head: connector ends (connection/interface/allocation) and flow
// ends + payload (flow). Other kinds contribute nothing.
func (p *Parser) parseTierBEnds(u *ast.Usage, kind ast.UsageKind) {
	switch kind {
	case ast.UsageConnection, ast.UsageInterface:
		// `connection c connect a to b` states its ends after `connect`; `connect a
		// to b` and `interface a.p to b.p` state them after the kind keyword itself.
		switch {
		case p.atKeyword("connect"):
			p.parseConnectorEnds(u, "connect")
		case u.Keyword == "connect":
			// The keyword introduced the ends, so it is the ConnectorPart of the
			// grammar and states at least one end.
			p.parseConnectorEnds(u, "")
			if len(u.ConnectorEnds) == 0 {
				p.error(p.peek().Span, "expected connector end after 'connect'")
			}
		case p.atConnectorShorthandEnds():
			p.parseConnectorEnds(u, "")
		}
	case ast.UsageConnector:
		// Connector can use four syntaxes:
		// 1. "connect X to Y" - standard connector ends
		// 2. "from X to Y" - from/to syntax
		// 3. "to [mult] target" - single end typing (shorthand)
		// 4. "(X, Y, Z)" - n-ary end list (KerML.xtext:842 NaryConnectorDeclaration)
		if p.atKeyword("connect") {
			p.parseConnectorEnds(u, "connect")
		} else if p.at(lexer.LParen) {
			p.parseNaryConnectorEnds(u)
		} else if p.atConnectorChainFirstEnd() {
			// A feature chain as the first end states the ends after the keyword.
			p.parseConnectorEnds(u, "")
		} else if p.atKeyword("to") {
			// Single-end connector: "connector name to [mult] target"
			// This is shorthand for a connector with one implicit end
			p.advance() // consume "to"
			end := p.parseConnectorEnd()
			if end != nil {
				u.ConnectorEnds = append(u.ConnectorEnds, end)
			}
		} else {
			p.parseConnectorFromTo(u)
		}
	case ast.UsageSuccession:
		// A succession may state its ends as body members instead
		// (KerML.xtext SuccessionDeclaration:891): `succession { end ...; }`.
		if !p.at(lexer.LBrace) && !p.at(lexer.Semicolon) {
			p.parseConnectorEnds(u, "") // succession has no intermediate keyword
		}
	case ast.UsageAllocation:
		// Allocation usage syntax: allocate X to Y (no intermediate keyword, like succession)
		// Can be:
		// 1. `allocate X to Y` - 'allocate' is kind keyword
		// 2. `allocation name : Type allocate X to Y` - 'allocation' is kind keyword, 'allocate' is intermediate
		// 3. `allocation name : Type { body }` - simple allocation with body, no connector ends
		// Check for optional 'allocate' keyword (when kind keyword was 'allocation' not 'allocate')
		p.acceptKeyword("allocate")

		if p.atKeyword("to") {
			// Single-end form: allocate to target
			p.advance() // consume "to"
			end := p.parseConnectorEnd()
			if end != nil {
				u.ConnectorEnds = append(u.ConnectorEnds, end)
			}
		} else if !p.at(lexer.LBrace) && !p.at(lexer.Semicolon) {
			// Binary form: allocate source to target
			// Only parse connector ends if NOT at body start
			p.parseConnectorEnds(u, "") // no intermediate keyword
		}
		// If at LBrace or Semicolon, skip connector ends (simple allocation with body)
	case ast.UsageFlow:
		p.parseFlowEnds(u)
	case ast.UsageMetadata:
		// Metadata usage syntax: metadata Name about target1, target2, ...;
		// The 'about' clause specifies what elements this metadata annotates
		if p.acceptKeyword("about") {
			// Parse comma-separated list of targets
			for {
				target := p.parseRelationshipTarget()
				if target != nil {
					// Store as an annotation relationship: metadata annotates its target
					u.Relationships = append(u.Relationships, &ast.Relationship{
						Kind:   ast.RelAnnotates,
						Target: target,
					})
				}
				if !p.accept2(lexer.Comma) {
					break
				}
			}
		}
	case ast.UsageOccurrence:
		// Occurrence usage (message) syntax: message name of payload : Type from sender to receiver;
		// The 'from X to Y' connector ends specify sender and receiver
		if p.atKeyword("from") || p.atKeyword("to") {
			p.parseConnectorFromTo(u)
		}
	}
}

// parseConnectorEnds parses `<kw> end to end` (binary) or
// `<kw> ( end , end , ... )` (n-ary), where <kw> is `connect` or `allocate`.
// For succession, kw is empty and the pattern is directly `end then end`.
// Each end can optionally have a multiplicity: `[mult] end`.
// The connector clause is optional. On a malformed end, it records a diagnostic,
// keeps the ends parsed so far, and stops (the declaration remains a Usage).
func (p *Parser) parseConnectorEnds(u *ast.Usage, kw string) {
	// For connection/allocation, expect intermediate keyword ('connect'/'allocate')
	// For succession, no intermediate keyword (kw is empty)
	if kw != "" {
		if !p.acceptKeyword(kw) {
			return
		}
	}
	if p.at(lexer.LParen) {
		p.advance() // '('
		for {
			ce := p.parseConnectorEnd()
			if ce == nil {
				return // parseConnectorEnd recorded the diagnostic; keep partial ends
			}
			u.ConnectorEnds = append(u.ConnectorEnds, ce)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RParen, "expected ')' to close connector ends")
		return
	}
	// Binary form: end keyword end (where keyword is "to" for connection, "then" for succession).
	// For succession, support optional "first" keyword: first end then end
	if u.Kind == ast.UsageSuccession {
		p.acceptKeyword("first") // optional "first" before first end
	}
	from := p.parseConnectorEnd()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)

	// Check for optional "references" keyword after first end
	// Pattern: end X references Y to end Z
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			from.Reference = refTarget
		}
	}

	// Determine expected keyword based on usage kind
	var expectedKeyword string
	switch u.Kind {
	case ast.UsageSuccession:
		expectedKeyword = "then"
	default:
		expectedKeyword = "to"
	}

	if !p.acceptKeyword(expectedKeyword) {
		p.error(p.peek().Span, fmt.Sprintf("expected '%s' between connector ends", expectedKeyword))
		return
	}
	to := p.parseConnectorEnd()
	if to == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, to)

	// Check for optional "references" keyword after second end
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			to.Reference = refTarget
		}
	}
}

// parseConnectorEnd parses a single connector end: optional multiplicity followed by qualified name.
func (p *Parser) parseConnectorEnd() *ast.ConnectorEnd {
	start := p.peek().Span.Offset
	ce := &ast.ConnectorEnd{}

	// Optional multiplicity
	if p.at(lexer.LBracket) {
		ce.Multiplicity = p.parseMultiplicity()
	}

	// Target expression (qualified name or feature chain)
	// Use parseRelationshipTarget to avoid consuming connector keywords (from/to)
	ce.Target = p.parseRelationshipTarget()
	if ce.Target == nil {
		return nil
	}

	// Optional relationships (e.g., ::> for interface binding)
	// Parse relationships until we hit a stopping keyword (to/from/then/references) or terminator
	rels := p.parseRelationships(true)
	ce.Relationships = rels

	ce.NodeSpan = p.spanFrom(start)
	return ce
}

// parseNaryConnectorEnds parses the parenthesized end list a KerML connector
// declaration states without an introducing keyword; the grammar requires at
// least two ends there (KerML.xtext:842).
func (p *Parser) parseNaryConnectorEnds(u *ast.Usage) {
	before := len(u.ConnectorEnds)
	p.parseConnectorEnds(u, "")
	if n := len(u.ConnectorEnds) - before; n == 1 {
		p.error(u.ConnectorEnds[before].Span(),
			"expected at least two connector ends in a parenthesized end list")
	}
}

// parseConnectorFromTo parses the `from x to y` pattern for connector usages.
// Pattern: `from <end> [references <target>] to <end> [references <target>]` (binary form only).
func (p *Parser) parseConnectorFromTo(u *ast.Usage) {
	if !p.acceptKeyword("from") {
		return // Optional connector clause
	}

	from := p.parseConnectorEnd()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)

	// Check for optional "references" keyword after from end
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			from.Reference = refTarget
		}
	}

	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' between connector ends")
		return
	}

	to := p.parseConnectorEnd()
	if to == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, to)

	// Check for optional "references" keyword after to end
	if p.acceptKeyword("references") {
		refTarget := p.parseRelationshipTarget()
		if refTarget != nil {
			to.Reference = refTarget
		}
	}
}

// atConnectorChainFirstEnd reports whether a connector states a feature chain as
// its first end (`connector f.a to a.g;`), which cannot be its name
// (KerML.xtext ConnectorEnd:854 over OwnedReferenceSubsetting:699).
func (p *Parser) atConnectorChainFirstEnd() bool {
	switch p.peekN(1).Kind {
	case lexer.Dot, lexer.ColonColon:
	default:
		return false
	}
	return p.atEndThenKeyword("to")
}

// atEndThenKeyword reports whether the cursor is at a connector end — a name or
// arbitrarily deep feature chain such as `differential.leftDiffPort` — followed
// by kw.
func (p *Parser) atEndThenKeyword(kw string) bool {
	return p.endThenKeywordAt(0, kw)
}

// endThenKeywordAt is atEndThenKeyword from the token at offset from.
func (p *Parser) endThenKeywordAt(from int, kw string) bool {
	switch p.peekN(from).Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
	default:
		return false
	}
	i := from + 1
	for p.peekN(i).Kind == lexer.Dot || p.peekN(i).Kind == lexer.ColonColon {
		switch p.peekN(i + 1).Kind {
		case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
			i += 2
		default:
			return false
		}
	}
	return p.peekIsKeyword(i, kw)
}

// atFlowShorthand reports whether the parser sits at a bare flow shorthand
// `x to y` (an end immediately followed by the `to` keyword), which has no
// declaration name (SysML.xtext `FlowDeclaration`, second alternative).
func (p *Parser) atFlowShorthand() bool {
	return p.atEndThenKeyword("to")
}

// atAllocateShorthand reports whether an allocation usage names its first
// connector end rather than itself: in `allocate torqueGenerator to powerTrain`
// both names are ends, while `allocate a1 : AllocDef` declares a named usage.
func (p *Parser) atAllocateShorthand() bool {
	return p.atEndThenKeyword("to")
}

// atConnectorShorthandEnds reports whether the cursor is at connector ends stated
// with no keyword of their own: `connect x.p to y.p`, `interface (a.p, b.p)`
// (SysML.xtext ConnectorPart, InterfacePart). A multiplicity written here is the
// first end's, so the ends are recognized past it.
func (p *Parser) atConnectorShorthandEnds() bool {
	if p.at(lexer.LParen) {
		return true
	}
	return p.endThenKeywordAt(p.pastBracketed(0), "to")
}

// pastBracketed returns the offset of the token after the balanced `[…]` group
// at offset from, or from itself where no group starts there.
func (p *Parser) pastBracketed(from int) int {
	if p.peekN(from).Kind != lexer.LBracket {
		return from
	}
	depth := 0
	for i := from; p.peekN(i).Kind != lexer.EOF; i++ {
		switch p.peekN(i).Kind {
		case lexer.LBracket:
			depth++
		case lexer.RBracket:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return from
}

// atPayloadDeclaration reports whether the payload after `of` declares a feature
// of its own (`of name : T`, `of name[1] : T`) rather than stating only the
// payload's type (`of T`, `of T[1]`). The declaration form is the Payload
// alternative carrying a PayloadFeatureSpecializationPart (SysML.xtext:1303),
// whose multiplicity may precede the typing.
func (p *Parser) atPayloadDeclaration() bool {
	if !p.atName() {
		return false
	}
	return p.peekN(p.pastBracketed(1)).Kind == lexer.Colon
}

// parseFlowEnds parses an optional `of <payload>` followed by either
// `from <x> to <y>` or the shorthand `<x> to <y>`. On a malformed end it records
// a diagnostic and keeps whatever ends were parsed so far.
func (p *Parser) parseFlowEnds(u *ast.Usage) {
	start := p.peek().Span.Offset
	var fe *ast.FlowEnds
	hasOf := p.acceptKeyword("of")
	if hasOf {
		fe = &ast.FlowEnds{}
		// Payload can be:
		// 1. Simple reference: of Type, of Type[1], of [1] Type
		// 2. Typed declaration: of name : Type, of name[1] : Type, of name : Type[1]
		// Check for (name + colon) pattern to distinguish
		if p.atPayloadDeclaration() {
			// Typed declaration - parse as nested member
			// Create a usage for the payload declaration
			payloadStart := p.peek().Span.Offset
			payloadUsage := &ast.Usage{
				Kind: ast.UsageAttribute, // default to attribute
			}
			payloadUsage.Ident = p.parseIdentification()

			// A PayloadFeatureSpecializationPart takes the multiplicity on either
			// side of the typing (SysML.xtext:1309).
			if p.at(lexer.LBracket) {
				payloadUsage.Multiplicity = p.parseMultiplicity()
			}

			// Parse typing relationship
			if p.accept2(lexer.Colon) {
				typeName := p.parseQualifiedName()
				if typeName != nil {
					payloadUsage.Relationships = append(payloadUsage.Relationships, &ast.Relationship{
						Kind:   ast.RelTyping,
						Target: typeName,
					})
				}
			}
			if payloadUsage.Multiplicity == nil && p.at(lexer.LBracket) {
				payloadUsage.Multiplicity = p.parseMultiplicity()
			}

			// Parse optional value assignment: = expr
			if p.accept2(lexer.Eq) {
				payloadUsage.Value = p.ParseExpression()
			}

			// The declaration is a member like any other, so it carries its own
			// span: the symbol built from it is what go-to-definition, hover and
			// rename identify a payload by.
			payloadUsage.NodeSpan = p.spanFrom(payloadStart)

			// Store payload usage as member (nested in flow)
			u.Members = append(u.Members, payloadUsage)
			fe.PayloadDecl = payloadUsage
			// Also store reference in FlowEnds for compatibility (create QualifiedName from identifier)
			qn := &ast.QualifiedName{
				Parts: []ast.NameSegment{
					{Text: payloadUsage.Ident.Name, Span: payloadUsage.Ident.NameSpan},
				},
			}
			qn.NodeSpan = payloadUsage.Ident.NameSpan
			fe.Payload = qn
		} else if p.at(lexer.LBracket) {
			// `of [1] Publish` — the multiplicity may precede the typing
			// (SysML.xtext:1306).
			fe.PayloadMultiplicity = p.parseMultiplicity()
			fe.Payload = p.parseRelationshipTarget()
		} else {
			// Simple reference
			fe.Payload = p.parseRelationshipTarget() // Allow feature chains, not just qualified names
			// `of Publish[1]` — an OwnedFeatureTyping may be followed by an
			// OwnedMultiplicity (SysML.xtext:1305).
			if p.at(lexer.LBracket) {
				fe.PayloadMultiplicity = p.parseMultiplicity()
			}
		}
	}

	switch {
	case p.acceptKeyword("from"):
		if fe == nil {
			fe = &ast.FlowEnds{}
		}
		fe.From = p.parseRelationshipTarget() // Allow feature chains
		p.parseFlowTo(fe)
	case !hasOf && p.atName():
		// Shorthand `x to y`.
		fe = &ast.FlowEnds{}
		fe.From = p.parseRelationshipTarget() // Allow feature chains
		p.parseFlowTo(fe)
	}
	if fe != nil {
		fe.NodeSpan = p.spanFrom(start)
		u.FlowEnds = fe
	}
}

// parseFlowTo consumes the `to <end>` tail of a flow, recording a diagnostic if
// `to` is absent.
func (p *Parser) parseFlowTo(fe *ast.FlowEnds) {
	if p.acceptKeyword("to") {
		fe.To = p.parseRelationshipTarget() // Allow feature chains
		return
	}
	p.error(p.peek().Span, "expected 'to' between flow ends")
}

// atTransitionEnds reports whether the `transition` at the cursor states its
// ends — `first <source>` or `<source> to <target>` — as opposed to declaring a
// transition feature without them (`transition t : Signalling;`). The
// transition's own name, when it has one, stands between the keyword and the
// ends.
func (p *Parser) atTransitionEnds() bool {
	i := 1
	// Only the `first` spelling can carry a name of the transition's own; in the
	// `to` spelling the first name is the source itself.
	if (p.peekN(i).Kind == lexer.Identifier || p.peekN(i).Kind == lexer.UnrestrictedName) &&
		p.peekIsKeyword(i+1, "first") {
		i++
	}
	if p.peekIsKeyword(i, "first") {
		return true
	}
	// `<source> to <target>`, where the source may be a qualified name and, as a
	// state name, may be spelled with a keyword.
	for {
		switch p.peekN(i).Kind {
		case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
		default:
			return false
		}
		if p.peekN(i+1).Kind == lexer.ColonColon {
			i += 2
			continue
		}
		return p.peekIsKeyword(i+1, "to")
	}
}

// atRelationshipKeyword checks if current token is a relationship keyword (redefines, subsets, etc.).
func (p *Parser) atRelationshipKeyword() bool {
	if t := p.peek(); t.Kind == lexer.Keyword {
		if _, ok := relationshipKeywords[t.KeywordID]; ok {
			return true
		}
		// Special multi-word keywords
		if t.KeywordID == "defined" || t.KeywordID == "inverse" || t.KeywordID == "featured" {
			return true
		}
		// `typed` states a relationship only in `typed by`; elsewhere it is a name.
		if t.KeywordID == "typed" {
			n := p.peekN(1)
			return n.Kind == lexer.Keyword && n.KeywordID == "by"
		}
	}
	return false
}

// relationshipClauseKind consumes the operator/keyword that begins a
// relationship clause and returns its kind. Reports ok=false (consuming
// nothing) when the current token does not begin a relationship clause.
func (p *Parser) relationshipClauseKind(isUsage bool) (ast.RelationshipKind, bool) {
	if t := p.peek(); t.Kind == lexer.Keyword {
		if k, ok := relationshipKeywords[t.KeywordID]; ok {
			p.advance()
			// 'disjoint' requires 'from' keyword after it
			if k == ast.RelDisjoint {
				p.expect2Keyword("from")
			}
			return k, true
		}
		// `typed by` is the long spelling of ':' (KerML.xtext TypedBy:600).
		if t.KeywordID == "defined" ||
			(t.KeywordID == "typed" && p.peekN(1).KeywordID == "by") {
			p.advance()
			p.expect2Keyword("by")
			return ast.RelTyping, true
		}
		if t.KeywordID == "inverse" {
			p.advance()
			p.expect2Keyword("of")
			return ast.RelInverseOf, true
		}
		// `featured by T` states the types a feature is featured by, and is
		// reached from FeatureDeclaration alone (KerML.xtext:569, 659).
		if t.KeywordID == "featured" {
			p.advance()
			p.expect2Keyword("by")
			return ast.RelFeaturedBy, true
		}
	}
	switch p.peek().Kind {
	case lexer.Colon:
		p.advance()
		return ast.RelTyping, true
	case lexer.ColonGt:
		p.advance()
		if isUsage {
			return ast.RelSubsets, true
		}
		return ast.RelSpecializes, true
	case lexer.ColonGtGt:
		p.advance()
		return ast.RelRedefines, true
	case lexer.ColonColonGt:
		p.advance()
		return ast.RelReferences, true
	case lexer.EqGt:
		p.advance()
		return ast.RelCrosses, true
	}
	return 0, false
}

// parseMultiplicity parses `[ lower ( .. upper )? ]` when a `[` is present.
func (p *Parser) parseMultiplicity() *ast.Multiplicity {
	if p.peek().Kind != lexer.LBracket {
		return nil
	}
	start := p.peek().Span.Offset
	p.advance() // '['
	m := &ast.Multiplicity{}
	m.Lower = p.parseMultiplicityBound()
	if p.accept2(lexer.DotDot) {
		m.IsRange = true
		m.Upper = p.parseMultiplicityBound()
	}
	p.expect(lexer.RBracket, "expected ']' to close multiplicity")
	m.NodeSpan = p.spanFrom(start)
	return m
}

// parseMultiplicityBound parses a single bound: `*` (infinity) or an expression.
// The bound is parsed above range precedence so the multiplicity's own `..`
// separator is not swallowed as a range operator.
func (p *Parser) parseMultiplicityBound() ast.Node {
	if p.peek().Kind == lexer.Star {
		star := p.peek()
		p.advance()
		inf := &ast.LiteralInfinity{}
		inf.NodeSpan = star.Span
		return inf
	}
	return p.parseBinary(precAdditive)
}

// parseMetadataUsage parses one `@` metadata usage (SysML v2 MetadataUsage):
// `@Type;`, or `@Type { prop = value; }` binding its features. Each is a member
// of its own rather than a prefix of the declaration after it.
func (p *Parser) parseMetadataUsage(start int) *ast.PrefixMetadata {
	p.advance() // '@'

	metaType := p.parseQualifiedName()
	if metaType == nil {
		p.error(p.peek().Span, "expected metadata type after '@'")
		return nil
	}
	pm := &ast.PrefixMetadata{Type: metaType}

	if p.at(lexer.LBrace) {
		p.advance() // '{'
		var body []ast.Node
		for !p.at(lexer.RBrace) && !p.atEOF() {
			if m := p.parseBodyMember(); m != nil {
				body = append(body, m)
				continue
			}
			expr := p.ParseExpression()
			p.accept2(lexer.Semicolon)
			body = append(body, expr)
		}
		p.expect(lexer.RBrace, "expected '}' after metadata body")
		pm.Body = body
	} else {
		// A usage is a member of its own, so it ends here rather than annotating
		// whatever follows it — `#Type` is the prefix spelling.
		p.expect(lexer.Semicolon, "expected ';' or '{' after a metadata usage")
	}

	pm.NodeSpan = p.spanFrom(start)
	return pm
}
