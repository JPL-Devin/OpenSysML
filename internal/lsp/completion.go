package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Completion implements textDocument/completion: members of what a member
// access names, else the names in scope plus library names and keywords.
// Prefix filtering is left to the client.
func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	c := &completionItems{seen: map[string]bool{}}

	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc != nil && doc.Scope != nil {
		offset := positionToOffset(doc.Content, params.Position)
		scope := enclosingScope(doc.Scope, offset)
		if path, ok := memberPathBefore(doc.Content, offset); ok {
			for _, sym := range s.ws.MembersOnPath(scope, path) {
				c.addSymbol(s, sym)
			}
			return c.list(), nil
		}
		for ; scope != nil; scope = scope.Parent() {
			for _, sym := range scope.Members() {
				c.addSymbol(s, sym)
			}
		}
	}

	for _, sym := range s.ws.TopLevelSymbols() {
		c.addSymbol(s, sym)
	}
	for _, kw := range lexer.Keywords() {
		c.add(protocol.CompletionItem{
			Label:  kw,
			Kind:   protocol.CompletionItemKindKeyword,
			Detail: "keyword",
		})
	}

	return c.list(), nil
}

// completionItems accumulates items, keeping the first offered under a label so
// a nearer declaration wins over an inherited or library name.
type completionItems struct {
	seen  map[string]bool
	items []protocol.CompletionItem
}

func (c *completionItems) add(item protocol.CompletionItem) {
	if item.Label == "" || c.seen[item.Label] {
		return
	}
	c.seen[item.Label] = true
	c.items = append(c.items, item)
}

// addSymbol offers a declared element with its real kind, detail and docs.
func (c *completionItems) addSymbol(s *Server, sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	item := protocol.CompletionItem{
		Label:  leafName(sym.Name),
		Kind:   completionKind(sym.Kind),
		Detail: completionDetail(sym),
	}
	if docText := s.symbolDocumentation(sym); docText != "" {
		item.Documentation = protocol.MarkupContent{Kind: protocol.PlainText, Value: docText}
	}
	c.add(item)
}

func (c *completionItems) list() *protocol.CompletionList {
	return &protocol.CompletionList{IsIncomplete: false, Items: c.items}
}

// symbolDocumentation returns the comment trivia preceding a symbol's
// declaration, when the document declaring it is loaded.
func (s *Server) symbolDocumentation(sym *symbols.Symbol) string {
	if len(sym.LeadingTrivia) == 0 || sym.DocName == "" {
		return ""
	}
	doc := s.ws.Document(sym.DocName)
	if doc == nil {
		return ""
	}
	return leadingDocText(doc.Content, sym.LeadingTrivia)
}

// completionDetail describes an element the way hover does: its kind, plus the
// type it declares when it declares one.
func completionDetail(sym *symbols.Symbol) string {
	detail := sym.Kind.String()
	if t := declaredTypeText(sym); t != "" {
		detail += " : " + t
	}
	return detail
}

// declaredTypeText returns the qualified name a usage declares as its type, or
// "" for anything else — a definition, or an untyped usage.
func declaredTypeText(sym *symbols.Symbol) string {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return ""
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			return semantics.QualifiedNameText(qn)
		}
	}
	return ""
}

// leafName returns the last segment of a name: a cached library symbol is named
// by its qualified name, which is not what completing a member inserts.
func leafName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

// completionKind maps a symbol kind to the LSP completion kind, so client icons
// distinguish definitions, usages, behaviors and packages.
func completionKind(k symbols.SymbolKind) protocol.CompletionItemKind {
	switch k {
	case symbols.SymbolPackage, symbols.SymbolNamespace, symbols.SymbolDependency:
		return protocol.CompletionItemKindModule
	case symbols.SymbolAlias:
		return protocol.CompletionItemKindReference
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return protocol.CompletionItemKindText

	// Definitions name types.
	case symbols.SymbolEnumerationDef:
		return protocol.CompletionItemKindEnum
	case symbols.SymbolActionDef, symbols.SymbolCalcDef,
		symbols.SymbolConstraintDef, symbols.SymbolRequirementDef,
		symbols.SymbolCaseDef, symbols.SymbolAnalysisCaseDef,
		symbols.SymbolVerificationCaseDef, symbols.SymbolUseCaseDef:
		return protocol.CompletionItemKindFunction
	case symbols.SymbolPartDef, symbols.SymbolAttributeDef, symbols.SymbolItemDef,
		symbols.SymbolOccurrenceDef, symbols.SymbolIndividualDef,
		symbols.SymbolMetadataDef, symbols.SymbolMetaclass,
		symbols.SymbolViewDef, symbols.SymbolViewpointDef, symbols.SymbolRenderingDef,
		symbols.SymbolConcernDef, symbols.SymbolConnectionDef, symbols.SymbolFlowDef,
		symbols.SymbolPortDef, symbols.SymbolInterfaceDef, symbols.SymbolAllocationDef,
		symbols.SymbolStateDef:
		return protocol.CompletionItemKindClass

	// Usages name features of an enclosing element.
	case symbols.SymbolEnumerationUsage:
		return protocol.CompletionItemKindEnumMember
	case symbols.SymbolActionUsage, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage,
		symbols.SymbolCaseUsage, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseUsage, symbols.SymbolUseCaseUsage:
		return protocol.CompletionItemKindMethod
	case symbols.SymbolAttributeUsage:
		return protocol.CompletionItemKindProperty
	default:
		return protocol.CompletionItemKindField
	}
}

// memberPathBefore returns the segments of the member access ending at offset:
// `v.`, `v.wh` and `A::B::` yield [v], [v] and [A B]. The partial name being
// typed is excluded, since the client filters on it.
func memberPathBefore(content []byte, offset int) ([]string, bool) {
	if offset > len(content) {
		offset = len(content)
	}
	i := identStart(content, offset)
	var path []string
	for {
		sep := separatorBefore(content, i)
		if sep == 0 {
			break
		}
		i -= sep
		start := identStart(content, i)
		if start == i {
			return nil, false // a separator with no name before it
		}
		path = append([]string{string(content[start:i])}, path...)
		i = start
	}
	return path, len(path) > 0
}

// separatorBefore returns the length of the member separator ending at i — 2
// for `::`, 1 for `.` — or 0 when no separator ends there.
func separatorBefore(content []byte, i int) int {
	if i >= 2 && content[i-1] == ':' && content[i-2] == ':' {
		return 2
	}
	if i >= 1 && content[i-1] == '.' {
		return 1
	}
	return 0
}

// identStart returns the offset at which the identifier ending at i begins,
// or i itself when no identifier character precedes i.
func identStart(content []byte, i int) int {
	start := i
	for start > 0 && isIdentByte(content[start-1]) {
		start--
	}
	return start
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// enclosingScope returns the deepest scope whose owning declaration span
// contains offset, starting from root. Falls back to root.
func enclosingScope(root *symbols.Scope, offset int) *symbols.Scope {
	best := root
	for _, sym := range root.Members() {
		if sym.Scope == nil {
			continue
		}
		sp := sym.DeclSpan
		if offset >= sp.Offset && offset < sp.End() {
			return enclosingScope(sym.Scope, offset)
		}
	}
	return best
}
