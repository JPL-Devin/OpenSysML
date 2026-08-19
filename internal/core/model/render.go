package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// PseudoViewPrefix marks a rendering asked for by element rather than by a view
// the document declares: `#tree`, `#state:<fqn>`, `#action:<fqn>`,
// `#interconnection:<fqn>`. Nothing is added to the model or the index for one.
const PseudoViewPrefix = "#"

// ErrNoView is a document that declares no view, which is rendered through a
// pseudo-view instead.
var ErrNoView = errors.New("declares no view")

// ViewInfo is one view a document declares: its qualified name, the rendering
// kind it states, whether this implementation produces that kind, and why not
// when it does not.
type ViewInfo struct {
	Name      string
	Kind      view.Kind
	Supported bool
	Reason    string
}

// Views lists the views a document declares, in qualified-name order, each with
// the rendering kind it states. A view stating a kind that is recognized but not
// produced — sequence, geometry, textual — is listed as unsupported with the
// reason, so a client can say why it cannot be drawn.
func (w *Workspace) Views(doc string) []ViewInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	renderer := w.rendererLocked(doc)
	if renderer == nil {
		return nil
	}
	out := []ViewInfo{}
	for _, sym := range w.documentViewsLocked(doc) {
		info := ViewInfo{Name: notationFQN(w.index, sym), Supported: true}
		kind, _, err := renderer.KindOf(sym)
		switch {
		case err == nil:
			info.Kind = kind
		default:
			info.Supported = false
			info.Reason = err.Error()
			var unsupported *view.UnsupportedKindError
			if errors.As(err, &unsupported) {
				info.Kind = unsupported.Kind
			}
		}
		out = append(out, info)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RenderView renders a view of a document. fqn names a view the document
// declares, by qualified name; a pseudo-view (`#tree`, `#state:<fqn>`,
// `#action:<fqn>`, `#interconnection:<fqn>`) renders an element as if a view
// exposing it had been declared; and "" renders the document's own view, which
// is the ambiguity error when it declares more than one and ErrNoView when it
// declares none.
func (w *Workspace) RenderView(doc, fqn string) (*view.Rendering, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	renderer := w.rendererLocked(doc)
	if renderer == nil {
		return nil, fmt.Errorf("%s: no such document", doc)
	}
	if strings.HasPrefix(fqn, PseudoViewPrefix) {
		return w.renderPseudoLocked(doc, fqn, renderer)
	}
	if fqn == "" {
		views := w.documentViewsLocked(doc)
		switch len(views) {
		case 0:
			return nil, fmt.Errorf("%s: %w; render #tree, #state:<name>, #action:<name> or #interconnection:<name> instead", doc, ErrNoView)
		case 1:
			return renderer.Render(views[0])
		default:
			return nil, fmt.Errorf("%s: declares %d views (%s); name the one to render",
				doc, len(views), strings.Join(viewNames(w.index, views), ", "))
		}
	}
	sym := w.viewNamedLocked(doc, fqn)
	if sym == nil {
		return nil, fmt.Errorf("%s: no view named %s", doc, fqn)
	}
	rendering, err := renderer.Render(sym)
	if err != nil {
		if errors.Is(err, semantics.ErrNotAView) {
			return nil, fmt.Errorf("%s: %w", fqn, err)
		}
		return nil, err
	}
	return rendering, nil
}

// renderPseudoLocked renders a pseudo-view: `#<kind>` renders the document's own
// top-level elements, `#<kind>:<fqn>` the element named.
func (w *Workspace) renderPseudoLocked(doc, spec string, renderer *view.Renderer) (*view.Rendering, error) {
	name, target, _ := strings.Cut(strings.TrimPrefix(spec, PseudoViewPrefix), ":")
	kind, ok := pseudoKinds[name]
	if !ok {
		return nil, fmt.Errorf("%s is no pseudo-view: write %s", spec, strings.Join(pseudoViewSpecs(), ", "))
	}

	exposed := []*symbols.Symbol{}
	stated := fmt.Sprintf("no view declared; rendering %s directly", doc)
	if target != "" {
		sym := w.declaredInLocked(doc, target)
		if sym == nil {
			return nil, fmt.Errorf("%s: %s names nothing in this document", spec, target)
		}
		exposed = append(exposed, sym)
		stated = fmt.Sprintf("no view declared; rendering %s directly", notationFQN(w.index, sym))
	} else {
		exposed = append(exposed, w.topLevelDeclarationsLocked(doc)...)
	}
	return renderer.RenderExposed(exposed, kind, stated)
}

// pseudoKinds are the rendering kinds a pseudo-view names.
var pseudoKinds = map[string]view.Kind{
	"tree":            view.KindTree,
	"interconnection": view.KindInterconnection,
	"state":           view.KindState,
	"action":          view.KindAction,
	"table":           view.KindTable,
}

// pseudoViewSpecs names the pseudo-views, for an error message.
func pseudoViewSpecs() []string {
	out := make([]string, 0, len(pseudoKinds))
	for name := range pseudoKinds {
		out = append(out, PseudoViewPrefix+name)
	}
	sort.Strings(out)
	return out
}

// rendererLocked builds a renderer over the workspace index, reading the
// document's own content for the labels a rendering takes verbatim. It is the
// same construction Session.viewRenderer makes in the REPL.
func (w *Workspace) rendererLocked(doc string) *view.Renderer {
	d := w.docs[doc]
	if d == nil {
		return nil
	}
	resolver, sem := w.newResolver()
	sf := source.New(doc, d.Content)
	text := func(name string, span source.Span) string {
		if name != doc {
			return ""
		}
		return sf.Text(span)
	}
	return view.NewRenderer(sem, resolver, text)
}

// documentViewsLocked are the views the document declares, outermost first, in
// declaration order.
func (w *Workspace) documentViewsLocked(doc string) []*symbols.Symbol {
	var out []*symbols.Symbol
	walkScope(w.index.DocumentRoot(doc), func(sym *symbols.Symbol) {
		if semantics.IsView(sym) {
			out = append(out, sym)
		}
	})
	return out
}

// topLevelDeclarationsLocked are the elements a document declares at its root,
// and the members of the packages there, which is what a pseudo-view naming no
// element exposes.
func (w *Workspace) topLevelDeclarationsLocked(doc string) []*symbols.Symbol {
	root := w.index.DocumentRoot(doc)
	if root == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, sym := range scopeMembers(root) {
		if sym.Kind == symbols.SymbolPackage {
			out = append(out, scopeMembers(sym.Scope)...)
			continue
		}
		out = append(out, sym)
	}
	return out
}

// viewNamedLocked is the view fqn names, resolved as the index spells it and as
// the document itself spells it, so a client may name a view either way.
func (w *Workspace) viewNamedLocked(doc, fqn string) *symbols.Symbol {
	if sym := w.declaredInLocked(doc, fqn); sym != nil && semantics.IsView(sym) {
		return sym
	}
	return nil
}

// declaredInLocked is the element fqn names in the document: by qualified name in
// the index, else by qualified or simple name among the document's own
// declarations.
func (w *Workspace) declaredInLocked(doc, fqn string) *symbols.Symbol {
	for _, sym := range w.index.LookupQualified(fqn) {
		if sym.DocName == doc {
			return sym
		}
	}
	var found *symbols.Symbol
	walkScope(w.index.DocumentRoot(doc), func(sym *symbols.Symbol) {
		if found != nil {
			return
		}
		if w.index.GetFQN(sym) == fqn || sym.Name == fqn {
			found = sym
		}
	})
	return found
}

// walkScope visits every symbol declared in scope and in the scopes nested in
// it, in declaration order.
func walkScope(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	if scope == nil {
		return
	}
	for _, sym := range scopeMembers(scope) {
		visit(sym)
		walkScope(sym.Scope, visit)
	}
}

// scopeMembers are the symbols a scope declares, named and anonymous, in
// declaration order.
func scopeMembers(scope *symbols.Scope) []*symbols.Symbol {
	if scope == nil {
		return nil
	}
	members := scope.AllMembers()
	out := make([]*symbols.Symbol, 0, len(members))
	for _, sym := range members {
		// An alias and an import bring in what another namespace declares; the
		// document declares neither.
		if sym == nil || sym.Kind == symbols.SymbolAlias || sym.DocName != scope.DocName() {
			continue
		}
		out = append(out, sym)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}

// viewNames names views for an ambiguity message.
func viewNames(idx *symbols.Index, views []*symbols.Symbol) []string {
	out := make([]string, 0, len(views))
	for _, sym := range views {
		out = append(out, notationFQN(idx, sym))
	}
	return out
}

// notationFQN names a symbol by qualified name as the index spells it, falling
// back to its own name.
func notationFQN(idx *symbols.Index, sym *symbols.Symbol) string {
	if idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}
