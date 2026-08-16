package repl

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// doView reports what a view exposes and the views nested in it. A view exposing
// nothing says so; an element that is no view is semantics.ErrNotAView.
func (s *Session) doView(name string) ([]string, bool, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, false, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, err
	}
	model := ctx.Model()
	exposed, err := model.ExposedElements(sym)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	out := []string{fmt.Sprintf("view %s", notationName(fqn))}
	if len(exposed) == 0 {
		out = append(out, "  exposes nothing")
	} else {
		out = append(out, "  exposes")
		for _, elem := range exposed {
			out = append(out, "    "+s.viewElementLine(elem))
		}
	}
	nested, err := model.NestedViews(sym)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	if len(nested) > 0 {
		out = append(out, "  nested views")
		for _, view := range nested {
			out = append(out, "    "+s.viewElementLine(view))
		}
	}
	return out, false, nil
}

// viewElementLine names an element by qualified name and kind, as %search does.
func (s *Session) viewElementLine(sym *symbols.Symbol) string {
	name := sym.Name
	if idx := s.browseIndex(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			name = fqn
		}
	}
	return fmt.Sprintf("%s (%s)", notationName(name), sym.Kind.String())
}
