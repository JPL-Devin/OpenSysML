package repl

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// doView reports what a view exposes, the way every other name-taking command
// reports: a name the session cannot find, or an element that is no view, is a
// line rather than a failure of the command.
func (s *Session) doView(name string) ([]string, bool, error) {
	lines, err := s.View(name)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{"error: " + err.Error()}, false, nil
	}
	return lines, false, nil
}

// View reports what a view exposes and the views nested in it. A view exposing
// nothing says so; an element that is no view is semantics.ErrNotAView.
func (s *Session) View(name string) ([]string, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	model := ctx.Model()
	exposed, err := model.ExposedElements(sym)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
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
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	if len(nested) > 0 {
		out = append(out, "  nested views")
		for _, view := range nested {
			out = append(out, "    "+s.viewElementLine(view))
		}
	}
	return out, nil
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
