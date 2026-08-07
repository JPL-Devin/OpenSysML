package parser

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
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
	"in": true, "out": true, "inout": true,
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
	"actor":       ast.UsageAttribute, // Use case actor
	"render":      ast.UsageAttribute, // View rendering
	"bind":        ast.UsageBinding,   // shorthand for binding
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
	"stakeholder":  ast.UsageAttribute, // stakeholder in viewpoints/concerns
	"frame":        ast.UsageAttribute, // frame in viewpoints/views
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
	"chains":      ast.RelChains,
}

type featureMods struct {
	isAbstract        bool
	isVariation       bool
	isReference       bool
	isEnd             bool
	isChain           bool
	isConstant        bool
	isEvent           bool // event modifier for occurrences
	isIndividual      bool // individual modifier for individuals/snapshots
	isSnapshot        bool // snapshot modifier for snapshots
	visibility        ast.Visibility
	direction         ast.FeatureDirection
	isComposite       bool
	isDerived         bool
	isReadonly        bool
	isOrdered         bool
	isNonunique       bool
	earlyMultiplicity *ast.Multiplicity // for "end [mult] ref ..." syntax
}

// atKindPrefix reports whether the current keyword qualifies the kind keyword
// after it instead of being the kind itself, as in `var feature x` or
// `item part Shape`. When it does not, the second keyword names the declaration
// (`action flow { ... }` is an action named `flow`).
func (p *Parser) atKindPrefix() bool {
	if !p.at(lexer.Keyword) || notKindPrefixKeywords[p.peek().KeywordID] {
		return false
	}
	if !isKindKeyword(p.peekN(1)) {
		return false
	}
	return kindPrefixKeywords[p.peek().KeywordID] || !namesDeclaration(p.peekN(2))
}

// atSecondaryKind reports whether the current kind keyword belongs to the kind
// of the declaration whose first kind keyword was already consumed, rather than
// being that declaration's name.
func (p *Parser) atSecondaryKind(firstKeyword string) bool {
	if notKindPrefixKeywords[firstKeyword] || !isKindKeyword(p.peek()) {
		return false
	}
	if compoundDefKinds[[2]string{firstKeyword, p.peek().KeywordID}] || kindPrefixKeywords[firstKeyword] {
		return true
	}
	return !namesDeclaration(p.peekN(1))
}

// isKindKeyword reports whether the token is a def or usage kind keyword.
func isKindKeyword(t lexer.Token) bool {
	if t.Kind != lexer.Keyword {
		return false
	}
	_, isDef := definitionKindKeywords[t.KeywordID]
	_, isUsage := usageKindKeywords[t.KeywordID]
	return isDef || isUsage
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

// atDefUsageStart reports whether the current token begins a def/usage
// declaration: a feature-modifier keyword or a kind keyword.
func (p *Parser) atDefUsageStart() bool {
	t := p.peek()
	// Check for relationship tokens that can precede kind keyword (e.g., :>> num)
	if t.Kind == lexer.ColonGt || t.Kind == lexer.ColonGtGt || t.Kind == lexer.Colon || t.Kind == lexer.Tilde {
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
	_, isDef := definitionKindKeywords[t.KeywordID]
	_, isUsage := usageKindKeywords[t.KeywordID]
	return isDef || isUsage
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
func isModifierOrKindKeyword(kw string) bool {
	_, isMod := featureModifierKeywords[kw]
	_, isDef := definitionKindKeywords[kw]
	_, isUsage := usageKindKeywords[kw]
	return isMod || isDef || isUsage
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
		case "constant":
			m.isConstant = true
		case "event":
			// Check if standalone usage: event <name>; (no typing/body)
			// If followed by identifier/qualified name (not keyword), it's usage keyword
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Identifier || (nextTok.Kind == lexer.Keyword && !isModifierOrKindKeyword(nextTok.KeywordID)) {
				// Treat as usage keyword, stop consuming modifiers
				return m
			}
			m.isEvent = true
		case "individual":
			// Check if standalone def/usage: individual def/occurrence/part ...
			// If followed by def/usage keyword OR typing colon, it's def/usage keyword, not modifier
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Colon || nextTok.Kind == lexer.ColonGt || nextTok.Kind == lexer.ColonGtGt {
				// individual : Type → anonymous usage
				return m
			}
			if nextTok.Kind == lexer.Keyword {
				if nextTok.KeywordID == "def" {
					// individual def → DefIndividual keyword
					return m
				}
				if _, isUsageKw := usageKindKeywords[nextTok.KeywordID]; isUsageKw {
					// individual <usageKind> → usage keyword, not modifier
					return m
				}
			}
			m.isIndividual = true
		case "snapshot":
			// Check if standalone usage: snapshot <name> ...
			// If followed by identifier/qualified name, could be usage keyword
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Identifier || (nextTok.Kind == lexer.Keyword && !isModifierOrKindKeyword(nextTok.KeywordID)) {
				// Treat as usage keyword, stop consuming modifiers
				return m
			}
			m.isSnapshot = true
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

	// Check for relationship tokens before modifiers (e.g., :>> x)
	// These indicate anonymous usages (attribute is default kind)
	tok := p.peek()
	if tok.Kind == lexer.ColonGt || tok.Kind == lexer.ColonGtGt || tok.Kind == lexer.Colon {
		// No modifiers, no kind keyword - parse as anonymous attribute usage
		u := p.parseUsage(start, ast.UsageAttribute, featureMods{}, false)
		return applyPrefixes(u)
	}

	mods := p.parseFeatureModifiers()

	// Two-word `use case` kind keyword.
	if p.atUseCase() {
		p.advance() // 'use'
		p.advance() // 'case'
		if p.atKeyword("def") {
			p.advance() // 'def'
			return applyPrefixes(p.parseDefinition(start, ast.DefUseCase, mods, false))
		}
		return applyPrefixes(p.parseUsage(start, ast.UsageUseCase, mods, false))
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
		p.advance()   // consume 'perform'
		kw = "action" // treat as regular action keyword
		// Continue to dual-keyword path (don't enter usage-only block)
	} else if kw == "subject" || kw == "objective" || kw == "succession" || kw == "inv" || kw == "connector" || kw == "bind" || kw == "satisfy" || kw == "verify" || kw == "include" || kw == "step" || kw == "expr" || kw == "interaction" || kw == "require" || kw == "transition" || kw == "perform" || kw == "exhibit" || kw == "variant" || kw == "assert" || kw == "assume" || kw == "event" || kw == "stakeholder" || kw == "frame" || kw == "actor" || kw == "expose" || kw == "render" || kw == "timeslice" || kw == "snapshot" {
		// Check for usage-only keywords that never have def forms

		// Special case: perform <ref>; (shorthand without action keyword)
		// Must check BEFORE consuming keyword token
		if kw == "perform" && !p.atKeyword("action") {
			p.advance() // consume 'perform'
			// Parse reference target (can be feature chain like takePhoto.focus)
			u := &ast.Usage{
				Kind:        ast.UsageAction,
				IsAbstract:  mods.isAbstract,
				IsReference: mods.isReference,
				IsEnd:       mods.isEnd,
				Visibility:  mods.visibility,
				Direction:   mods.direction,
				IsComposite: mods.isComposite,
			}
			u.NodeBase.NodeSpan = p.spanFrom(start)

			// Parse reference target (qualified name or feature chain)
			target := p.parseRelationshipTarget()
			if target != nil {
				// The performed action is related to the perform action usage
				// by a reference subsetting (SysML 7.17.6).
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelReferences,
					Target: target,
				})
			} else {
				p.error(p.peek().Span, "expected an action reference after 'perform'")
			}

			// Expect semicolon or body
			if p.accept2(lexer.Semicolon) {
				u.HasBody = false
			} else if p.at(lexer.LBrace) {
				p.advance()
				u.Members = p.parseActionBodyMixed()
				u.HasBody = true
			}

			u.NodeSpan = p.spanFrom(start)
			return applyPrefixes(u)
		}

		p.advance() // consume the kind keyword
		isAll := p.acceptKeyword("all")

		// Special case: include use case <name> (full form)
		// If include is followed by "use case", consume them and parse as use case with includes relationship
		if kw == "include" && p.atUseCase() {
			p.advance() // consume 'use'
			p.advance() // consume 'case'
			u := p.parseUsage(start, ast.UsageUseCase, mods, isAll)
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

		// Special case: succession flow from X to Y
		// If succession is followed by flow keyword, parse as flow usage with succession typing
		if kw == "succession" && p.atKeyword("flow") {
			p.advance() // consume 'flow'
			u := p.parseUsage(start, ast.UsageFlow, mods, isAll)
			// Add implicit succession typing - succession concept applies to this flow
			// Use typing relationship to indicate this flow has succession semantics
			if u != nil {
				// Add succession as typing (could also use specialization)
				// For now, treat as semantic annotation - flow inherits succession characteristics
				// Implementation note: May need dedicated AST flag or relationship for this hybrid
			}
			return applyPrefixes(u)
		}

		// Special case: event <ref>; (shorthand event occurrence reference)
		// If event is NOT followed by occurrence keyword or typing colon, parse as occurrence usage with reference
		if kw == "event" && !p.atKeyword("occurrence") && !p.at(lexer.Colon) {
			// Parse reference target (qualified name like driver::setSpeedSent)
			u := &ast.Usage{
				Kind:        ast.UsageOccurrence,
				IsEvent:     true,
				IsAbstract:  mods.isAbstract,
				IsReference: mods.isReference,
				IsEnd:       mods.isEnd,
				Visibility:  mods.visibility,
				Direction:   mods.direction,
				IsComposite: mods.isComposite,
			}
			u.NodeBase.NodeSpan = p.spanFrom(start)

			// Parse reference target (qualified name or feature chain)
			target := p.parseRelationshipTarget()
			if target != nil {
				// Add as references relationship
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelReferences,
					Target: target,
				})
			}

			// Optional multiplicity after reference
			if p.at(lexer.LBracket) {
				u.Multiplicity = p.parseMultiplicity()
			}

			// Optional relationships (e.g., :>> target)
			rels, _ := p.parseRelationships(true)
			u.Relationships = append(u.Relationships, rels...)

			// Expect semicolon or body
			if p.accept2(lexer.Semicolon) {
				u.HasBody = false
			} else if p.at(lexer.LBrace) {
				p.advance()
				members, hasBody := p.parseDefUsageBody()
				u.Members = members
				u.HasBody = hasBody
			}

			u.NodeSpan = p.spanFrom(start)
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
		kindKeyword := kw
		if isKindKeyword(p.peek()) &&
			(kindPrefixKeywords[kw] || (kw == "variant" && !namesDeclaration(p.peekN(1)))) {
			kindKeyword = p.peek().KeywordID
			p.advance()
		}
		return applyPrefixes(p.parseUsage(start, usageKindKeywords[kindKeyword], mods, isAll))
	}

	// Special case: if current token is 'def' (after prefixes/modifiers), parse as generic definition
	// This handles patterns like `#scenario def X` where prefix acts as semantic annotation
	if p.atKeyword("def") {
		p.advance() // consume 'def'
		// Use generic definition kind (or could extract from prefix)
		return applyPrefixes(p.parseDefinition(start, ast.DefClass, mods, false))
	}

	defKind, ok := definitionKindKeywords[kw]
	if !ok {
		// Fallback: if we have modifiers but no kind keyword, assume it's a generic usage (e.g., "in x: Integer;")
		// This is common for parameters in calc/action bodies.
		// Also check if name + multiplicity/modifiers follow (e.g., "in seq[1..*] ordered;")
		hasModifiers := mods.direction != ast.DirNone || mods.isReference || mods.isEnd || mods.isComposite || mods.isDerived
		hasNameWithMultOrMods := p.atNameOrKeyword() && (p.peekN(1).Kind == lexer.LBracket || p.peekN(1).Kind == lexer.Colon || isPostModifierKeyword(p.peekN(1)))
		// SysML v2 §7.27.4: a user-defined keyword may declare a usage without
		// any language-defined keyword (`#cause 'battery old' { ... }`). The
		// kind of such a usage comes from the metadata, not the syntax.
		keywordOnlyUsage := len(prefixes) > 0 && (p.at(lexer.Identifier) || p.at(lexer.UnrestrictedName))
		if hasModifiers || hasNameWithMultOrMods || keywordOnlyUsage {
			return applyPrefixes(p.parseUsage(start, ast.UsageAttribute, mods, false))
		}
		return applyPrefixes(nil)
	}
	p.advance() // consume the kind keyword

	// Parse 'all' modifier if present (appears after keyword, before name)
	isAll := p.acceptKeyword("all")

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
	if p.atSecondaryKind(kw) {
		defKind = definitionKindKeywords[p.peek().KeywordID]
		p.advance() // consume secondary keyword
	}

	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return applyPrefixes(p.parseDefinition(start, defKind, mods, isAll))
	}

	// Check if this is a definition-only keyword (not in usageKindKeywords)
	// Examples: metaclass, struct, class, predicate, bool
	// These keywords don't require "def" suffix and can't be used as usages
	_, hasUsageForm := usageKindKeywords[kw]
	if !hasUsageForm {
		// Definition-only keyword - parse as definition directly
		return applyPrefixes(p.parseDefinition(start, defKind, mods, isAll))
	}

	// Note: 'datatype' is treated uniformly as a usage keyword by the parser.
	// Semantic classification (def vs usage) is deferred to the symbol builder
	// and semantics passes, which have full context (relationships, body structure).
	// This follows Phase 4 principle: parse syntax uniformly, classify semantically.

	return applyPrefixes(p.parseUsage(start, usageKindKeywords[kw], mods, isAll))
}

func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, mods featureMods, isAll bool) *ast.Definition {
	def := &ast.Definition{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsVariation: mods.isVariation,
		IsAll:       isAll,
		IsConstant:  mods.isConstant,
		IsEvent:     mods.isEvent,
		Visibility:  mods.visibility,
		Ident:       p.parseIdentification(),
	}
	def.Relationships, _ = p.parseRelationships(false)

	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
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
	case ast.DefRequirement:
		// Requirement def bodies: requirement body OR generic
		// Lookahead: if body starts with requirement keywords → parseRequirementBody
		// Otherwise → generic parseDefUsageBody
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isRequirementKeyword() {
				members = p.parseRequirementBody()
			} else {
				members = p.parseActionBodyGeneric()
			}
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
	default:
		members, hasBody = p.parseDefUsageBody()
	}

	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}

// parseActionBodyGeneric parses generic action def body (same as parseDefUsageBody internals)
func (p *Parser) parseActionBodyGeneric() []ast.Node {
	var members []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		m := p.parseBodyMember()
		if m != nil {
			// Check for namespace-level succession: 'then' after member
			if p.atKeyword("then") {
				p.advance() // consume 'then'

				// Apply succession to membership node
				if mem, ok := m.(*ast.Membership); ok {
					mem.HasSuccession = true
					// Target will be next member parsed in loop
				}
			}
			members = append(members, m)
		}
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return members
}

// isBehavioralKeyword checks if next token is a behavioral keyword
func (p *Parser) isBehavioralKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	switch kw {
	case "first", "done", "fork", "join", "merge", "decision", "action", "then",
		"assign", "perform", "while", "loop", "if", "send", "terminate", "for":
		return true
	}
	return false
}

// isResultKeyword checks if next token is 'return'
func (p *Parser) isResultKeyword() bool {
	return p.at(lexer.Keyword) && p.peek().KeywordID == "return"
}

// isRequirementKeyword checks if next token is requirement-related
func (p *Parser) isRequirementKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	return kw == "subject" || kw == "assume" || kw == "require" || kw == "actor" || kw == "doc"
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

func (p *Parser) parseUsage(start int, kind ast.UsageKind, mods featureMods, isAll bool) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		IsAll:       isAll,
		IsEnd:       mods.isEnd,
		IsChain:     mods.isChain,
		IsConstant:  mods.isConstant,
		IsEvent:     mods.isEvent,
		Visibility:  mods.visibility,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
		IsDerived:   mods.isDerived,
		IsOrdered:   mods.isOrdered,
		IsNonunique: mods.isNonunique,
	}

	// Apply early multiplicity if present (for "end [mult] ref ..." syntax)
	if mods.earlyMultiplicity != nil {
		u.Multiplicity = mods.earlyMultiplicity
	}

	// Handle UsageSatisfy special syntax:
	// Full form: satisfy [requirement] <name> by <name> { body }
	// Short form: satisfy/verify <name>;
	if kind == ast.UsageSatisfy {
		// Per SatisfyRequirementUsage: without the `requirement` keyword the name is
		// a reference subsetting of an existing requirement usage, not a typing;
		// with the keyword it declares a new requirement usage.
		declaresRequirement := p.acceptKeyword("requirement")

		reqName := p.parseQualifiedName()
		if reqName != nil {
			if declaresRequirement && len(reqName.Parts) == 1 {
				u.Ident.Name = reqName.Parts[0].Text
				u.Ident.NameSpan = reqName.Parts[0].Span
			} else {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelSubsets,
					Target: reqName,
				})
			}
		}

		// Check for optional "by" clause
		if p.acceptKeyword("by") {
			subjTarget := p.parseRelationshipTarget()
			if subjTarget != nil {
				// Store subject as identification or relationship depending on node type
				// If it's a simple qualified name, use as identification
				// If it's a feature chain or other expression, store as relationship
				if qn, ok := subjTarget.(*ast.QualifiedName); ok && len(qn.Parts) > 0 && u.Ident.Name == "" {
					u.Ident.Name = qn.Parts[0].Text
					u.Ident.NameSpan = qn.Parts[0].Span
				} else {
					// Store as a subject relationship for complex expressions
					u.Relationships = append(u.Relationships, &ast.Relationship{
						Kind:   ast.RelSubject,
						Target: subjTarget,
					})
				}
			}
		}

		// Parse body (requirement body) or semicolon
		members, hasBody := p.parseDefUsageBody()
		u.Members = members
		u.HasBody = hasBody
		u.NodeSpan = p.spanFrom(start)
		return u
	}

	// Handle UsageBinding special syntax: binding [mult] name = [mult] target; OR binding name[mult] of [mult] target = [mult] value;
	if kind == ast.UsageBinding {
		// Check for multiplicity before name: binding [mult] name ...
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
		}

		// `binding [mult] bind [mult] src = [mult] tgt` states the connector's
		// ends after the `bind` keyword instead of naming the connector, so the
		// keyword is consumed rather than read as the name.
		if p.atKeyword("bind") {
			p.advance()
			if p.at(lexer.LBracket) {
				p.parseMultiplicity() // end multiplicity, not the connector's
			}
			if source := p.parseRelationshipTarget(); source != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelRedefines, // Use redefines to mark binding source
					Target: source,
				})
			}
		} else if p.atNameOrKeyword() && p.peekN(1).Kind != lexer.Dot && p.peekN(1).Kind != lexer.LBracket {
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
			// Feature chain or qualified name - parse as relationship target
			// Store in relationships as source (redefines relationship to indicate binding source)
			source := p.parseRelationshipTarget()
			if source != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelRedefines, // Use redefines to mark binding source
					Target: source,
				})
			}
		}

		// Check for multiplicity after name (before "of"): name[mult] of ...
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
		}

		// Check for source expression: binding [mult] name[mult2] source = target
		// If we have name[mult] and next token is NOT "of" or "=", parse source expression
		if u.Ident.Name != "" && !p.atKeyword("of") && !p.at(lexer.Eq) && (p.atName() || p.atNameOrKeyword()) {
			// Parse source as relationship target
			source := p.parseRelationshipTarget()
			if source != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelRedefines, // Use redefines to mark binding source
					Target: source,
				})
			}
		}

		// Check for "of" keyword (binding name of [mult] target = value)
		if p.acceptKeyword("of") {
			// Parse source multiplicity and target
			if p.at(lexer.LBracket) {
				// Store source multiplicity somewhere - for now skip or use relationships
				p.parseMultiplicity() // consume but don't store separately
			}
			// Parse target as typing relationship
			target := p.parseRelationshipTarget()
			if target != nil {
				u.Relationships = append(u.Relationships, &ast.Relationship{
					Kind:   ast.RelTyping,
					Target: target,
				})
			}
		}

		// Parse value: = [mult] expr
		if p.accept2(lexer.Eq) {
			// Optional multiplicity before value expression
			if p.at(lexer.LBracket) {
				p.parseMultiplicity() // consume multiplicity prefix in value
			}
			u.Value = p.ParseExpression()
		}

		// Parse body or semicolon
		members, hasBody := p.parseDefUsageBody()
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

	// Handle shorthand: `feature redefines x` means `feature x redefines x`
	// Check if relationship keyword followed by simple name (not qualified name or feature chain)
	// BUT NOT if followed by multiplicity (e.g., `part redefines cyl[4]` is anonymous redefining cyl, not shorthand)
	var preRels []*ast.Relationship
	var conjugated bool
	if p.at(lexer.Keyword) {
		if relKind, ok := relationshipKeywords[p.peek().KeywordID]; ok {
			// Peek ahead to see if simple name follows (not :: or . or [)
			nextTok := p.peekN(1)
			nextNext := p.peekN(2)
			isSimpleName := (nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.UnrestrictedName) &&
				(nextNext.Kind != lexer.ColonColon && nextNext.Kind != lexer.Dot && nextNext.Kind != lexer.LBracket)
			if isSimpleName {
				// Shorthand: relationship keyword + simple name
				p.advance() // consume relationship keyword
				u.Ident = p.parseIdentification()
				// Create implicit relationship targeting same name
				rel := &ast.Relationship{
					Kind:   relKind,
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: u.Ident.Name, Span: u.Ident.NameSpan}}},
				}
				preRels = append(preRels, rel)
				// Check for additional comma-separated targets (e.g., `feature redefines x, y, z`)
				// Even in shorthand form, we can have multiple targets after the first
				for p.accept2(lexer.Comma) {
					target := p.parseRelationshipTarget()
					if target != nil {
						r := &ast.Relationship{Kind: relKind, Target: target}
						preRels = append(preRels, r)
					} else {
						break
					}
				}
			} else {
				// Normal relationship parsing (qualified names, feature chains, or no name after keyword)
				preRels, conjugated = p.parseRelationships(true)
				// A bare flow shorthand `flow x to y` and succession `succession x then y` have no declaration name
				if !(kind == ast.UsageFlow && p.atFlowShorthand()) && !(kind == ast.UsageSuccession && isAnonymous) {
					u.Ident = p.parseUsageIdentification(kind)
				}
			}
		} else {
			// Not a relationship keyword
			preRels, conjugated = p.parseRelationships(true)
			// A bare flow shorthand `flow x to y` and anonymous succession `succession x then y` have no declaration name
			if !(kind == ast.UsageFlow && p.atFlowShorthand()) && !(kind == ast.UsageSuccession && isAnonymous) {
				u.Ident = p.parseUsageIdentification(kind)
			}
		}
	} else {
		// No relationship shorthand
		preRels, conjugated = p.parseRelationships(true)
		// A bare flow shorthand `flow x to y` and anonymous succession `succession x then y` have no declaration name
		// Anonymous connector starts with 'from' keyword (e.g., `connector : X from y to z`)
		skipIdentification := (kind == ast.UsageFlow && p.atFlowShorthand()) ||
			(kind == ast.UsageSuccession && isAnonymous) ||
			(kind == ast.UsageAllocation && p.atAllocateShorthand()) ||
			(kind == ast.UsageConnector && p.atKeyword("from"))
		if !skipIdentification {
			u.Ident = p.parseUsageIdentification(kind)
		}
	}

	// Parse post-identification relationships (e.g., : Type)
	postIdRels, postConj := p.parseRelationships(true)
	u.Relationships = append(preRels, postIdRels...)
	u.IsConjugated = conjugated || postConj

	// For anonymous succession/flow, skip multiplicity parsing - it belongs to connector ends
	// UNLESS earlyMultiplicity was already parsed (e.g., `succession [mult] first ...`)
	skipMultiplicity := (kind == ast.UsageSuccession || kind == ast.UsageFlow) && u.Ident.Name == "" && earlyMultiplicity == nil
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
	postRels, _ := p.parseRelationships(true)
	u.Relationships = append(u.Relationships, postRels...)

	if p.accept2(lexer.Eq) || p.accept2(lexer.ColonEq) || p.acceptKeyword("default") {
		u.Value = p.ParseExpression()
	}
	p.parseTierBEnds(u, kind)

	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	switch kind {
	case ast.UsageAction:
		// Action usage bodies: mixed (declarations + behavioral statements)
		// Support THREE forms:
		// 1. action name; (no body)
		// 2. action name { in item x; action nested {...}; first ...; } (braced mixed body)
		// 3. action name \n statements (inline behavioral body without braces)
		if p.accept2(lexer.Semicolon) {
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
			// Parse statements until we hit something that's NOT a behavioral statement
			// Typically one statement, but could be multiple connected with 'then'
			// EXCEPT: if 'then' is followed by a declaration keyword (action/feature/etc),
			// it's namespace-level succession, not behavioral succession - stop parsing body
			for p.isBehavioralKeyword() && !p.atEOF() {
				// Check if 'then' is namespace succession (then <visibility>? <defKeyword>)
				if p.atKeyword("then") {
					next := p.peekN(1)
					// If followed by visibility or definition/usage keyword, it's namespace succession - stop
					if next.Kind == lexer.Keyword {
						if _, isVis := map[string]bool{"public": true, "private": true, "protected": true}[next.KeywordID]; isVis {
							break // namespace succession
						}
						if _, isDef := definitionKindKeywords[next.KeywordID]; isDef {
							break // namespace succession
						}
						if _, isUsage := usageKindKeywords[next.KeywordID]; isUsage {
							break // namespace succession
						}
					}
				}
				members = append(members, p.parseActionMember())
			}
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
	case ast.UsageRequirement:
		// Requirement bodies: { subject/assume/require/actor ... }
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseRequirementBody()
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
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		m := p.parseBodyMember()
		if m != nil {
			// Check for namespace-level succession: 'then' after member
			if p.atKeyword("then") {
				p.advance() // consume 'then'

				// Apply succession to membership node
				if mem, ok := m.(*ast.Membership); ok {
					mem.HasSuccession = true
					// Optional: parse guard condition if present
					// For now, store target will be resolved during semantic analysis
					// We just mark that this member has a succession edge
				}
			}
			members = append(members, m)
		}
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return members, true
}

// parseBodyMember parses one body member: an optional visibility prefix
// followed by a declaration (which may be a nested def/usage). Import/Alias
// carry their own visibility and are returned directly; other declarations are
// wrapped in a Membership. Mirrors parseMember.
func (p *Parser) parseBodyMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	// Check for 'then' prefix (namespace-level succession marker)
	// Pattern: then include use case ...; OR then done;
	// Consume 'then', parse declaration, mark with HasSuccession
	if p.atKeyword("then") {
		p.advance() // consume 'then'

		// Recursively parse the member after 'then'
		innerMember := p.parseBodyMember()
		if mem, ok := innerMember.(*ast.Membership); ok {
			mem.HasSuccession = true
		}
		return innerMember
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

	// Check for metadata annotation: @Type{props}
	// This creates a metadata usage (annotation statement)
	if p.at(lexer.At) {
		metadata := p.parseMetadataAnnotations()
		if len(metadata) > 0 {
			// Wrap first metadata in membership
			// (Multiple annotations would need different representation)
			pm := metadata[0]
			pm.SetLeadingTrivia(trivia)
			m := &ast.Membership{
				Visibility: vis,
				Member:     pm,
			}
			m.NodeSpan = p.spanFrom(start)
			m.SetLeadingTrivia(trivia)
			return m
		}
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

	// Check for redefines statement: `redefines name = expr;` OR `redefines parent.child = expr;`
	// This is shorthand for anonymous feature with redefines relationship and value
	// Example: redefines innerSpaceDimension = 0;
	// Example: redefines parent.value = 100;
	if p.atKeyword("redefines") {
		// Lookahead: check if pattern is `redefines <target> = <value>`
		// Need to skip past qualified name or feature chain to find '='
		i := 1
		for i < 10 { // reasonable lookahead limit
			tk := p.peekN(i)
			if tk.Kind == lexer.Eq {
				// Found pattern - parse it
				p.advance() // skip "redefines"
				target := p.parseRelationshipTarget()
				p.expect(lexer.Eq, "expected '=' after redefines target")
				value := p.ParseExpression()
				p.accept2(lexer.Semicolon)

				u := &ast.Usage{
					Kind: ast.UsagePart, // Generic feature
					Relationships: []*ast.Relationship{
						{
							Kind:   ast.RelRedefines,
							Target: target,
						},
					},
					Value: value,
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
			if tk.Kind == lexer.Identifier || tk.Kind == lexer.Dot || tk.Kind == lexer.ColonColon {
				i++
				continue
			}
			// Hit something else - not redefines statement pattern
			break
		}
	}

	// Check for subset/disjoint constraint statements
	// Pattern: subset X subsets Y; OR disjoint X from Y;
	// These are anonymous features with relationships
	if p.atKeyword("subset") || p.atKeyword("disjoint") {
		isDisjoint := p.atKeyword("disjoint")
		p.advance() // skip "subset" or "disjoint"

		// Parse first target (source)
		source := p.parseRelationshipTarget()

		if isDisjoint {
			// Pattern: disjoint X from Y;
			if !p.acceptKeyword("from") {
				p.error(p.peek().Span, "expected 'from' after disjoint target")
			}
			target := p.parseRelationshipTarget()
			p.accept2(lexer.Semicolon)

			u := &ast.Usage{
				Kind: ast.UsagePart, // Generic feature
				Relationships: []*ast.Relationship{
					{
						Kind:   ast.RelDisjoint,
						Target: source,
					},
					{
						Kind:   ast.RelDisjoint,
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
		} else {
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
	}

	// Check for expose statement: expose <path>::**[filter];
	// View-specific member for exposing elements from a namespace
	if p.atKeyword("expose") {
		p.advance() // consume 'expose'

		// Parse import path with wildcard (similar to import parsing)
		// Pattern: <namespace>::** or <namespace>::**[filter]
		path := p.parseQualifiedName()
		if path == nil {
			p.error(p.peek().Span, "expected namespace path after 'expose'")
			return &ast.ErrorNode{Message: "expected namespace path"}
		}

		// Check for wildcard tail: :: * or :: **
		isRecursive := false
		if p.at(lexer.ColonColon) {
			nk := p.peekN(1).Kind
			if nk == lexer.Star {
				p.advance() // ::
				p.advance() // *
				// Check for recursive: :: **
				if p.at(lexer.ColonColon) && p.peekN(1).Kind == lexer.StarStar {
					p.advance() // ::
					p.advance() // **
					isRecursive = true
				}
			} else if nk == lexer.StarStar {
				p.advance() // ::
				p.advance() // **
				isRecursive = true
			}
		}

		// Check for optional filter expression: [filter]
		var filterExpr ast.Node
		if p.accept2(lexer.LBracket) {
			filterExpr = p.ParseExpression()
			p.expect(lexer.RBracket, "expected ']' after filter expression")
		}

		// Create Import node (expose uses similar semantics to import)
		// Store filter in FilterExpr field if present
		imp := &ast.Import{
			Imported:    path,
			IsRecursive: isRecursive,
			FilterExpr:  filterExpr,
		}
		imp.NodeBase.NodeSpan = p.spanFrom(start)
		imp.SetLeadingTrivia(trivia)

		p.expect(lexer.Semicolon, "expected ';' after expose statement")

		return imp
	}

	// Check for inline connector statement: connect [mult] X to [mult] Y;
	// This is anonymous connection usage
	if p.atKeyword("connect") {
		p.advance() // consume 'connect'

		u := &ast.Usage{
			Kind: ast.UsageConnection,
		}
		u.NodeBase.NodeSpan = p.spanFrom(start)
		u.SetLeadingTrivia(trivia)

		// Parse connector ends with optional multiplicity
		// First end
		firstEnd := p.parseConnectorEnd()
		if firstEnd == nil {
			p.error(p.peek().Span, "expected connector end after 'connect'")
			return &ast.ErrorNode{Message: "expected connector end"}
		}
		u.ConnectorEnds = append(u.ConnectorEnds, firstEnd)

		// Expect 'to' keyword
		if !p.acceptKeyword("to") {
			p.error(p.peek().Span, "expected 'to' after first connector end")
		}

		// Second end
		secondEnd := p.parseConnectorEnd()
		if secondEnd == nil {
			p.error(p.peek().Span, "expected connector end after 'to'")
		} else {
			u.ConnectorEnds = append(u.ConnectorEnds, secondEnd)
		}

		p.expect(lexer.Semicolon, "expected ';' after connect statement")

		m := &ast.Membership{
			Visibility: vis,
			Member:     u,
		}
		m.NodeBase.NodeSpan = u.Span()
		m.SetLeadingTrivia(trivia)
		return m
	}

	// Check for bare redefines/subsets/specializes statement (no visibility/modifiers)
	// Pattern: redefines <target> = <value>;
	if p.atKeyword("redefines") || p.atKeyword("subsets") || p.atKeyword("specializes") {
		relKind := ast.RelRedefines
		if p.atKeyword("subsets") {
			relKind = ast.RelSubsets
		} else if p.atKeyword("specializes") {
			relKind = ast.RelSpecializes
		}

		// Lookahead: check if pattern is `<rel> <target> = <value>`
		hasEq := false
		for i := 1; i < 10; i++ {
			tk := p.peekN(i)
			if tk.Kind == lexer.Eq {
				hasEq = true
				break
			}
			if tk.Kind != lexer.Identifier && tk.Kind != lexer.Keyword && tk.Kind != lexer.Dot && tk.Kind != lexer.ColonColon {
				break
			}
		}

		if hasEq {
			p.advance() // skip relationship keyword
			target := p.parseRelationshipTarget()
			p.expect(lexer.Eq, "expected '=' in relationship statement")
			value := p.ParseExpression()
			p.accept2(lexer.Semicolon)

			u := &ast.Usage{
				Kind: ast.UsagePart,
				Relationships: []*ast.Relationship{
					{
						Kind:   relKind,
						Target: target,
					},
				},
				Value: value,
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
	}

	// Check for inline flow statement: flow [from] X to Y;
	// This is anonymous flow usage
	// Distinguish from flow declarations: flow : Type ... or flow name : Type ...
	// If next token after 'flow' is identifier (not colon), it's inline flow
	if p.atKeyword("flow") {
		// Lookahead: flow <id/featureChain> to ... = inline flow
		//            flow : Type ... = flow usage declaration
		//            flow <name> : Type ... = flow usage declaration
		nextTok := p.peekN(1)
		isInlineFlow := false
		if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.Keyword {
			// Could be flow name or flow source
			// Check if followed by 'to' or 'from' (inline) vs colon/relationship (declaration)
			tok2 := p.peekN(2)
			if tok2.Kind == lexer.Keyword && (tok2.KeywordID == "to" || tok2.KeywordID == "from") {
				isInlineFlow = true
			} else if tok2.Kind == lexer.Dot || tok2.Kind == lexer.ColonColon {
				// Feature chain - likely inline flow
				isInlineFlow = true
			}
		}

		if isInlineFlow {
			p.advance() // consume 'flow'

			u := &ast.Usage{
				Kind: ast.UsageFlow,
			}
			u.NodeBase.NodeSpan = p.spanFrom(start)
			u.SetLeadingTrivia(trivia)

			// Check for optional 'from' keyword
			optionalFrom := p.acceptKeyword("from")
			_ = optionalFrom // both forms valid

			// Parse flow ends: source to target
			p.parseTierBEnds(u, ast.UsageFlow)

			p.expect(lexer.Semicolon, "expected ';' after flow statement")

			m := &ast.Membership{
				Visibility: vis,
				Member:     u,
			}
			m.NodeBase.NodeSpan = u.Span()
			m.SetLeadingTrivia(trivia)
			return m
		}
	}

	// Check for inline allocate statement: allocate X to Y;
	// This is anonymous allocation usage
	// Distinguish from allocation declarations: allocation : Type ... or allocation name : Type ...
	if p.atKeyword("allocate") {
		// Lookahead: allocate <id/featureChain> to ... = inline allocate
		//            allocate : Type ... = allocation usage declaration
		//            allocate <name> : Type ... = allocation usage declaration
		nextTok := p.peekN(1)
		isInlineAllocate := false
		if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.Keyword {
			// Could be allocation name or source
			// Check if followed by 'to' (inline) vs colon/relationship (declaration)
			tok2 := p.peekN(2)
			if tok2.Kind == lexer.Keyword && tok2.KeywordID == "to" {
				isInlineAllocate = true
			} else if tok2.Kind == lexer.Dot || tok2.Kind == lexer.ColonColon {
				// Feature chain - likely inline allocate
				isInlineAllocate = true
			}
		}

		if isInlineAllocate {
			p.advance() // consume 'allocate'

			u := &ast.Usage{
				Kind: ast.UsageAllocation,
			}
			u.NodeBase.NodeSpan = p.spanFrom(start)
			u.SetLeadingTrivia(trivia)

			// Parse allocation ends: source to target
			p.parseTierBEnds(u, ast.UsageAllocation)

			p.expect(lexer.Semicolon, "expected ';' after allocate statement")

			m := &ast.Membership{
				Visibility: vis,
				Member:     u,
			}
			m.NodeBase.NodeSpan = u.Span()
			m.SetLeadingTrivia(trivia)
			return m
		}
	}

	// Check for accept action syntax: action <name> accept <param> : Type [via <port>];
	// Pattern: action trigger accept scene : Scene;
	// Pattern: action trigger accept scene : Scene via viewPort;
	if p.atKeyword("action") {
		// Lookahead for accept pattern: action <id> accept <id> : Type
		if p.peekN(1).Kind == lexer.Identifier {
			tok2 := p.peekN(2)
			if tok2.Kind == lexer.Keyword && tok2.KeywordID == "accept" {
				// Accept action syntax - create simple action with output feature
				p.advance() // consume 'action'
				actionName := p.src.Text(p.peek().Span)
				p.advance() // consume name
				p.advance() // consume 'accept'

				// Parse param: name : Type
				paramName := ""
				var paramType *ast.QualifiedName
				if p.at(lexer.Identifier) {
					paramName = p.src.Text(p.peek().Span)
					p.advance()
				}
				if p.accept2(lexer.Colon) {
					paramType = p.parseQualifiedName()
				}

				// Optional 'via' port
				var viaPort *ast.QualifiedName
				if p.acceptKeyword("via") {
					viaPort = p.parseQualifiedName()
				}

				// Create action usage with output attribute member
				actionUsage := &ast.Usage{
					Kind: ast.UsageAction,
					Ident: ast.Identification{
						Name:     actionName,
						NameSpan: source.Span{}, // fill from token if needed
					},
				}

				// Add output parameter as attribute member
				if paramName != "" && paramType != nil {
					paramUsage := &ast.Usage{
						Kind: ast.UsageAttribute,
						Ident: ast.Identification{
							Name:     paramName,
							NameSpan: source.Span{},
						},
						Relationships: []*ast.Relationship{
							{Kind: ast.RelTyping, Target: paramType},
						},
						Direction: ast.DirOut,
						IsAccept:  true, // Mark as accept parameter
					}
					paramUsage.NodeSpan = p.spanFrom(start)
					actionUsage.Members = append(actionUsage.Members, &ast.Membership{
						Member: paramUsage,
					})
				}

				// Add via port as the receiving port relationship
				if viaPort != nil {
					actionUsage.Relationships = append(actionUsage.Relationships, &ast.Relationship{
						Kind:   ast.RelVia,
						Target: viaPort,
					})
				}

				p.expect(lexer.Semicolon, "expected ';' after accept action")

				actionUsage.NodeSpan = p.spanFrom(start)
				actionUsage.SetLeadingTrivia(trivia)

				m := &ast.Membership{
					Visibility: vis,
					Member:     actionUsage,
				}
				m.NodeSpan = actionUsage.Span()
				m.SetLeadingTrivia(trivia)
				return m
			}
		}
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

		// Check for redefines statement after modifiers: `portion redefines name = expr;`
		// This is shorthand for anonymous feature with modifiers + redefines relationship + value
		if p.atKeyword("redefines") {
			// Lookahead: check if pattern is `redefines <target> = <value>`
			i := 1
			for i < 10 {
				tk := p.peekN(i)
				if tk.Kind == lexer.Eq {
					// Found '=' - this is a redefines statement
					p.advance() // skip "redefines"
					target := p.parseRelationshipTarget()
					p.expect(lexer.Eq, "expected '=' in redefines statement")
					value := p.ParseExpression()
					p.accept2(lexer.Semicolon)

					u := &ast.Usage{
						Kind:        ast.UsagePart,
						IsComposite: mods.isComposite,
						IsReference: mods.isReference,
						IsDerived:   mods.isDerived,
						IsEnd:       mods.isEnd,
						Relationships: []*ast.Relationship{
							{
								Kind:   ast.RelRedefines,
								Target: target,
							},
						},
						Value: value,
					}
					u.NodeBase.NodeSpan = p.spanFrom(start)
					u.SetLeadingTrivia(trivia)

					m := &ast.Membership{
						Visibility: mods.visibility,
						Member:     u,
					}
					m.NodeBase.NodeSpan = u.Span()
					m.SetLeadingTrivia(trivia)
					return m
				}
				if tk.Kind == lexer.Identifier || tk.Kind == lexer.Keyword || tk.Kind == lexer.Dot || tk.Kind == lexer.ColonColon {
					i++
					continue
				}
				break
			}
		}

		// Special case: end shortname [mult] feature name pattern
		// Example: end self2 [1] feature sameThing: Anything
		// Also: end [1] feature transferSource (no short name)
		// Also: end ref source; (no definition keyword, just anonymous feature)
		// This declares a feature with 'end' modifier, optional short name, and multiplicity
		if mods.isEnd && (p.atNameOrKeyword() || p.at(lexer.LBracket)) {
			var shortName string
			var shortNameSpan source.Span
			var mult *ast.Multiplicity
			var hasDefKeyword bool
			var endRels []*ast.Relationship // relationships parsed before definition keyword

			// Parse optional short name (if not starting with '[')
			if p.atNameOrKeyword() {
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
								_, isDef := definitionKindKeywords[t.KeywordID]
								_, isUsage := usageKindKeywords[t.KeywordID]
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
					_, isDef := definitionKindKeywords[nextTok.KeywordID]
					_, isUsage := usageKindKeywords[nextTok.KeywordID]
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
						rel, _ := p.parseRelationships(true)
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
									_, isDef := definitionKindKeywords[t.KeywordID]
									_, isUsage := usageKindKeywords[t.KeywordID]
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
						_, isDef := definitionKindKeywords[nextTok.KeywordID]
						_, isUsage := usageKindKeywords[nextTok.KeywordID]
						isDefKeyword = isDef || isUsage
					}

					if isDefKeyword {
						mult = p.parseMultiplicity()

						// Parse optional relationship clauses before definition keyword
						for p.atRelationshipKeyword() {
							rel, _ := p.parseRelationships(true)
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
					u.Ident.ShortName = shortName
					u.Ident.ShortNameSpan = shortNameSpan
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

		// Check for name + colon (typed) OR direct relationship (anonymous) OR name + relationship OR name + semicolon OR name + multiplicity
		hasNameAndType := p.atName() && p.peekN(1).Kind == lexer.Colon
		hasRelationship := p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) || p.at(lexer.ColonColonGt) || p.atRelationshipKeyword()
		hasNameAndRelationship := p.atName() && (p.peekN(1).Kind == lexer.ColonGt || p.peekN(1).Kind == lexer.ColonGtGt || p.peekN(1).Kind == lexer.ColonColonGt)
		hasNameOnly := p.atName() && (p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.RBrace)
		hasNameAndMult := p.atName() && p.peekN(1).Kind == lexer.LBracket // name with multiplicity (e.g., ref payload [0..*])
		// Allow 'var' keyword as name for anonymous features (common in actions/loops)
		hasVarKeyword := p.atKeyword("var") && (p.peekN(1).Kind == lexer.LBracket || p.peekN(1).Kind == lexer.Colon ||
			p.peekN(1).Kind == lexer.ColonGt || p.peekN(1).Kind == lexer.ColonGtGt || p.peekN(1).Kind == lexer.ColonColonGt ||
			p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.RBrace)

		if hasNameAndType || hasRelationship || hasNameAndRelationship || hasNameOnly || hasNameAndMult || hasVarKeyword {
			var id ast.Identification

			// Parse optional name
			if hasNameAndType || hasNameAndRelationship || hasNameOnly || hasNameAndMult || hasVarKeyword {
				tok := p.advance()
				if tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName {
					id.Name = p.src.Text(tok.Span)
					id.NameSpan = tok.Span
				} else if tok.Kind == lexer.Keyword && tok.KeywordID == "var" {
					// Allow 'var' keyword as feature name
					id.Name = "var"
					id.NameSpan = tok.Span
				}
				if hasNameAndType {
					p.advance() // consume ':'
				}
			}

			// Parse as anonymous usage (attribute by default)
			u := &ast.Usage{
				Kind:        ast.UsageAttribute,
				Ident:       id,
				Visibility:  mods.visibility,
				IsReference: mods.isReference,
				IsDerived:   mods.isDerived,
				IsComposite: mods.isComposite,
				IsEnd:       mods.isEnd,
				IsChain:     mods.isChain,
				Direction:   mods.direction,
				IsOrdered:   mods.isOrdered,
				IsNonunique: mods.isNonunique,
			}

			// If we consumed a colon, parse typing relationship(s)
			// Support comma-separated types: : Type1, Type2, Type3
			if hasNameAndType {
				for {
					u.Relationships = append(u.Relationships, &ast.Relationship{
						Kind:   ast.RelTyping,
						Target: p.parseQualifiedName(),
					})
					// Check for comma - if present, parse additional type
					if !p.accept2(lexer.Comma) {
						break
					}
				}
			}

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
			moreRels, conjugated := p.parseRelationships(true)
			u.Relationships = append(u.Relationships, moreRels...)
			u.IsConjugated = conjugated

			// Parse optional value (= expr or default expr)
			if p.accept2(lexer.Eq) || p.acceptKeyword("default") {
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
		if tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName {
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
		u.Relationships = append(u.Relationships, &ast.Relationship{
			Kind:   ast.RelTyping,
			Target: p.parseQualifiedName(),
		})

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
		moreRels, conjugated := p.parseRelationships(true)
		u.Relationships = append(u.Relationships, moreRels...)
		u.IsConjugated = conjugated

		// Parse optional value (= expr or default expr)
		if p.accept2(lexer.Eq) || p.acceptKeyword("default") {
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
		p.peek().KeywordID == "satisfy" || p.peek().KeywordID == "verify" || p.peek().KeywordID == "step" || p.peek().KeywordID == "expr" || p.peek().KeywordID == "constraint" ||
		p.peek().KeywordID == "interaction" || p.peek().KeywordID == "bool" || p.peek().KeywordID == "assoc" || p.peek().KeywordID == "struct" ||
		p.peek().KeywordID == "class" || p.peek().KeywordID == "predicate")
	if !isUsageOnlyKwForEnum && p.atNameOrKeyword() && (nextKind == lexer.Eq || nextKind == lexer.Semicolon || nextKind == lexer.LBrace) {
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
	if p.at(lexer.Keyword) && p.atKindPrefix() {
		p.advance() // consume the prefix keyword
		inner := p.parseDeclaration(start)
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

	// Check for name-before-keyword pattern: <name> <keyword> { ... }
	// Example: myConstraint constraint { ... }
	if p.atName() {
		next := p.peekN(1)
		if next.Kind == lexer.Keyword {
			_, isDef := definitionKindKeywords[next.KeywordID]
			_, isUsage := usageKindKeywords[next.KeywordID]
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
		en := p.errorNodeSkip(start, "expected a body member")
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: inner}
	mem.NodeSpan = p.spanFrom(start)
	mem.SetLeadingTrivia(trivia)
	return mem
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
		// Parse member name
		seg, ok := p.parseNameSegmentRelaxed()
		if !ok {
			p.error(p.peek().Span, "expected a name after '.'")
			break
		}

		// Create member name as QualifiedName
		memberName := &ast.QualifiedName{Parts: []ast.NameSegment{seg}}
		memberName.NodeSpan = seg.Span

		chain := &ast.FeatureChainExpr{
			Operand: operand,
			Member:  memberName,
		}
		chain.NodeSpan = p.spanFrom(start)
		operand = chain
	}

	return operand
}

// parseRelationships parses zero or more relationship clauses. isUsage selects
// the meaning of the symbolic `:>` operator (subsets on a usage, specializes on
// a definition). Each clause may carry a comma-separated target list; every
// target becomes its own Relationship sharing the clause kind.
func (p *Parser) parseRelationships(isUsage bool) (rels []*ast.Relationship, conjugated bool) {
	for {
		kind, ok := p.relationshipClauseKind(isUsage)
		if !ok {
			return rels, conjugated
		}
		for {
			start := p.peek().Span.Offset
			// A leading `~` on a typing target is conjugation (`: ~ Type`).
			if p.accept2(lexer.Tilde) && kind == ast.RelTyping {
				conjugated = true
			}
			// Parse target using specialized parser that handles both qualified names
			// and feature chains but does NOT consume body expressions.
			target := p.parseRelationshipTarget()
			r := &ast.Relationship{Kind: kind, Target: target}
			r.NodeSpan = p.spanFrom(start)
			rels = append(rels, r)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
	}
}

// parseTierBEnds parses the distinctive Tier B usage grammar following the
// declaration head: connector ends (connection/interface/allocation) and flow
// ends + payload (flow). Other kinds contribute nothing.
func (p *Parser) parseTierBEnds(u *ast.Usage, kind ast.UsageKind) {
	switch kind {
	case ast.UsageConnection, ast.UsageInterface:
		p.parseConnectorEnds(u, "connect")
	case ast.UsageConnector:
		// Connector can use three syntaxes:
		// 1. "connect X to Y" - standard connector ends
		// 2. "from X to Y" - from/to syntax
		// 3. "to [mult] target" - single end typing (shorthand)
		if p.atKeyword("connect") {
			p.parseConnectorEnds(u, "connect")
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
	case ast.UsageTransition:
		// Transition usage syntax: first <end> accept <param> via <end> then <end>
		// Pattern: transition name first start accept payload: Type via receiver then done;
		p.parseTransitionUsageEnds(u)
	case ast.UsageSuccession:
		p.parseConnectorEnds(u, "") // succession has no intermediate keyword
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
	rels, _ := p.parseRelationships(true)
	ce.Relationships = rels

	ce.NodeSpan = p.spanFrom(start)
	return ce
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

// atFlowShorthand reports whether the parser sits at a bare flow shorthand
// `x to y` (a name immediately followed by the `to` keyword), which has no
// declaration name.
func (p *Parser) atFlowShorthand() bool {
	if !p.atName() {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "to"
}

// atAllocateShorthand reports whether an allocation usage names its first
// connector end rather than itself: in `allocate torqueGenerator to powerTrain`
// both names are ends, while `allocate a1 : AllocDef` declares a named usage.
func (p *Parser) atAllocateShorthand() bool {
	if !p.atName() {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "to"
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
		// 1. Simple reference: of Type
		// 2. Typed declaration: of name : Type
		// Check for (name + colon) pattern to distinguish
		if p.atName() && p.peekN(1).Kind == lexer.Colon {
			// Typed declaration - parse as nested member
			// Create a usage for the payload declaration
			payloadStart := p.peek().Span.Offset
			payloadUsage := &ast.Usage{
				Kind: ast.UsageAttribute, // default to attribute
			}
			payloadUsage.Ident = p.parseIdentification()

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
		} else {
			// Simple reference
			fe.Payload = p.parseRelationshipTarget() // Allow feature chains, not just qualified names
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

// parseTransitionUsageEnds parses transition usage connector-like ends with special keywords.
// Pattern: first <end> accept <param> via <end> then <end>
// Example: transition t first start accept payload: Type via receiver then done;
func (p *Parser) parseTransitionUsageEnds(u *ast.Usage) {
	// Parse "first <end>" if present
	if p.acceptKeyword("first") {
		end := p.parseConnectorEnd()
		if end != nil {
			u.ConnectorEnds = append(u.ConnectorEnds, end)
		}
	}

	// Parse "accept <param>" - this is body parameter, not connector end
	if p.acceptKeyword("accept") {
		// Parse parameter with optional name and type
		var paramName string
		var paramType *ast.QualifiedName

		if seg, ok := p.parseNameSegment(); ok {
			paramName = seg.Text
			if p.at(lexer.Colon) {
				p.advance() // :
				paramType = p.parseQualifiedName()
			}
		}

		// Store as body parameter in Members (similar to calc/function params)
		// For now, create simple documentation node as placeholder
		// Full implementation would need BodyParam support in Usage.Members
		_ = paramName // use later when we extend AST
		_ = paramType
	}

	// Parse "via <end>" if present
	if p.acceptKeyword("via") {
		end := p.parseConnectorEnd()
		if end != nil {
			u.ConnectorEnds = append(u.ConnectorEnds, end)
		}
	}

	// Parse "then <end>"
	if p.acceptKeyword("then") {
		end := p.parseConnectorEnd()
		if end != nil {
			u.ConnectorEnds = append(u.ConnectorEnds, end)
		}
	}
}

// atRelationshipKeyword checks if current token is a relationship keyword (redefines, subsets, etc.).
func (p *Parser) atRelationshipKeyword() bool {
	if t := p.peek(); t.Kind == lexer.Keyword {
		if _, ok := relationshipKeywords[t.KeywordID]; ok {
			return true
		}
		// Special multi-word keywords
		if t.KeywordID == "defined" || t.KeywordID == "inverse" {
			return true
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
		if t.KeywordID == "defined" {
			p.advance()
			p.expect2Keyword("by")
			return ast.RelTyping, true
		}
		if t.KeywordID == "inverse" {
			p.advance()
			p.expect2Keyword("of")
			return ast.RelInverseOf, true
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

// parseMetadataAnnotations parses optional metadata annotations: @Type{props}
// Can appear multiple times. Returns slice of PrefixMetadata nodes.
func (p *Parser) parseMetadataAnnotations() []*ast.PrefixMetadata {
	var metadata []*ast.PrefixMetadata

	for p.at(lexer.At) {
		start := p.peek().Span.Offset
		p.advance() // consume '@'

		// Parse metadata type
		metaType := p.parseQualifiedName()
		if metaType == nil {
			p.error(p.peek().Span, "expected metadata type after '@'")
			break
		}

		pm := &ast.PrefixMetadata{
			Type: metaType,
		}

		// Optional body: {prop = value; ...}
		if p.at(lexer.LBrace) {
			p.advance() // consume '{'
			// Parse body as generic members (assignments/nested declarations)
			var body []ast.Node
			for !p.at(lexer.RBrace) && !p.atEOF() {
				// Parse as body member or expression statement
				m := p.parseBodyMember()
				if m != nil {
					body = append(body, m)
				} else {
					// Try expression + semicolon
					expr := p.ParseExpression()
					p.accept2(lexer.Semicolon)
					body = append(body, expr)
				}
			}
			p.expect(lexer.RBrace, "expected '}' after metadata body")
			pm.Body = body
		}

		pm.NodeSpan = p.spanFrom(start)
		metadata = append(metadata, pm)
	}

	return metadata
}
