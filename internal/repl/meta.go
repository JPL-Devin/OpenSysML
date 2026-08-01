package repl

import (
	"fmt"
	"os"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// isMeta reports whether a trimmed input line is a meta command.
func isMeta(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "%")
}

var helpText = []string{
	"%help               show this help",
	"%list               list current session declarations",
	"%clear              reset the session",
	"%load <file>        read a file and submit its contents",
	"",
	"Runtime commands:",
	"%instantiate <name> create an instance of a part def",
	"%eval <expr>        evaluate an expression",
	"%slots <name>       show instance slots and values",
	"%instances          list all instantiated objects",
}

// runMeta executes a meta command line. Returns lines to print, whether to quit,
// and an error only for unrecoverable I/O (unknown commands print guidance).
func (s *Session) runMeta(line string) (out []string, quit bool, err error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return nil, false, nil
	}
	switch fields[0] {
	case "%help":
		return helpText, false, nil
	case "%list":
		decls := s.List()
		if len(decls) == 0 {
			return []string{"(empty session)"}, false, nil
		}
		return decls, false, nil
	case "%clear":
		s.Clear()
		return []string{"session cleared"}, false, nil
	case "%load":
		if len(fields) < 2 {
			return []string{"usage: %load <file>"}, false, nil
		}
		data, rerr := os.ReadFile(fields[1])
		if rerr != nil {
			return nil, false, fmt.Errorf("load %s: %w", fields[1], rerr)
		}
		r := s.Submit(string(data))
		return renderResult(r), false, nil
	case "%instantiate":
		if len(fields) < 2 {
			return []string{"usage: %instantiate <name>"}, false, nil
		}
		return s.doInstantiate(fields[1])
	case "%eval":
		if len(fields) < 2 {
			return []string{"usage: %eval <expression>"}, false, nil
		}
		expr := strings.TrimPrefix(line, "%eval")
		return s.doEval(strings.TrimSpace(expr))
	case "%slots":
		if len(fields) < 2 {
			return []string{"usage: %slots <name>"}, false, nil
		}
		return s.doSlots(fields[1])
	case "%instances":
		return s.doInstances()
	default:
		return []string{fmt.Sprintf("unknown command %q (try %%help)", fields[0])}, false, nil
	}
}

// doInstantiate creates an instance of a part def.
func (s *Session) doInstantiate(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(name)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: symbol %q not found", name)}, false, nil
	}
	
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: instantiation failed: %v", err)}, false, nil
	}
	
	s.instances[name] = inst
	return []string{
		fmt.Sprintf("✓ Created instance of %s", name),
		fmt.Sprintf("  ID: %d", inst.ID),
		fmt.Sprintf("  Use %%slots %s to inspect", name),
	}, false, nil
}

// doEval evaluates an expression.
func (s *Session) doEval(expr string) ([]string, bool, error) {
	// Try literal evaluation first (works even with empty session)
	literalResult, isLiteral := s.tryEvalLiteral(expr)
	if isLiteral {
		return literalResult, false, nil
	}
	
	// For feature references/complex expressions, need session context
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded (literals work, but feature references need declarations)"}, false, nil
	}
	
	// Create runtime context
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	
	// Try simple feature reference lookup (e.g., "%eval x")
	if isSimpleIdentifier(expr) {
		sym, ok := doc.Scope.LookupLocal(expr)
		if ok && sym != nil {
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
				val, err := ctx.Eval(usage.Value)
				if err != nil {
					return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
				}
				return []string{
					fmt.Sprintf("✓ %s", expr),
					fmt.Sprintf("  = %s", formatValue(val)),
				}, false, nil
			}
		}
		return []string{fmt.Sprintf("error: symbol %q not found", expr)}, false, nil
	}
	
	// Complex expression with feature refs - inject into session context
	tempSrc := s.joined() + fmt.Sprintf("\nattribute __eval__ = %s;", expr)
	p := parser.New(source.New("eval", []byte(tempSrc)))
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		lines := []string{"error: parse failed:"}
		for _, d := range p.Diagnostics {
			lines = append(lines, "  "+d.Message)
		}
		return lines, false, nil
	}
	
	// Find __eval__ attribute (should be last member)
	var evalUsage *ast.Usage
	for i := len(root.Members) - 1; i >= 0; i-- {
		if usage, ok := root.Members[i].(*ast.Usage); ok && usage.Value != nil {
			if usage.Ident.ShortName == "__eval__" {
				evalUsage = usage
				break
			}
		}
	}
	
	if evalUsage == nil || evalUsage.Value == nil {
		return []string{"error: could not parse expression"}, false, nil
	}
	
	val, err := ctx.Eval(evalUsage.Value)
	if err != nil {
		return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
	}
	
	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(val)),
	}, false, nil
}

// tryEvalLiteral attempts to evaluate standalone literal expressions.
func (s *Session) tryEvalLiteral(expr string) ([]string, bool) {
	// Parse as standalone attribute
	src := fmt.Sprintf("attribute __lit__ = %s;", expr)
	p := parser.New(source.New("literal", []byte(src)))
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 || len(root.Members) == 0 {
		return nil, false
	}
	
	usage, ok := root.Members[0].(*ast.Usage)
	if !ok || usage.Value == nil {
		return nil, false
	}
	
	// Use runtime context with empty model (no symbols needed for literals)
	emptyIdx := symbols.NewIndex()
	emptyModel := semantics.NewModel(resolve.New(emptyIdx))
	ctx := runtime.NewContext(emptyModel, resolve.New(emptyIdx), 100000)
	
	val, err := ctx.Eval(usage.Value)
	if err != nil {
		// Not evaluable as literal (needs session symbols)
		return nil, false
	}
	
	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(val)),
	}, true
}


// isSimpleIdentifier checks if string is a single identifier (no operators/spaces).
func isSimpleIdentifier(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Simple heuristic: no spaces, operators, or parens
	for _, ch := range s {
		if ch == ' ' || ch == '+' || ch == '-' || ch == '*' || ch == '/' || 
		   ch == '(' || ch == ')' || ch == '.' {
			return false
		}
	}
	return true
}

// doSlots shows instance slots.
func (s *Session) doSlots(name string) ([]string, bool, error) {
	inst, ok := s.instances[name]
	if !ok {
		return []string{fmt.Sprintf("error: no instance named %q (use %%instantiate first)", name)}, false, nil
	}
	
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	
	lines := []string{
		fmt.Sprintf("Instance: %s (ID: %d)", name, inst.ID),
		"Slots:",
	}
	
	// Get effective features to know slot names
	features := ctx.FeaturesOf(inst.Type)
	if len(features) == 0 {
		lines = append(lines, "  (no features)")
		return lines, false, nil
	}
	
	for _, feat := range features {
		slot, err := inst.GetSlot(ctx, feat.Name)
		if err != nil {
			lines = append(lines, fmt.Sprintf("  %s: <error: %v>", feat.Name, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s = %s", feat.Name, formatValue(slot.Value)))
	}
	
	return lines, false, nil
}

// doInstances lists all instantiated objects.
func (s *Session) doInstances() ([]string, bool, error) {
	if len(s.instances) == 0 {
		return []string{"(no instances created)"}, false, nil
	}
	
	lines := []string{"Instances:"}
	for name, inst := range s.instances {
		lines = append(lines, fmt.Sprintf("  %s (ID: %d)", name, inst.ID))
	}
	return lines, false, nil
}

// formatValue renders a runtime value for display.
func formatValue(val runtime.Value) string {
	switch val.Kind {
	case runtime.ValConst:
		switch val.Const.Kind {
		case semantics.ValInt:
			return fmt.Sprintf("%d", val.Const.Int)
		case semantics.ValReal:
			return fmt.Sprintf("%.2f", val.Const.Real)
		case semantics.ValBool:
			return fmt.Sprintf("%v", val.Const.Bool)
		case semantics.ValInfinity:
			return "∞"
		default:
			return "<unknown const>"
		}
	case runtime.ValNull:
		return "null"
	case runtime.ValString:
		return fmt.Sprintf("%q", val.Str)
	case runtime.ValInstance:
		return fmt.Sprintf("Instance(ID: %d)", val.Instance)
	case runtime.ValSequence:
		return fmt.Sprintf("Sequence[%d]", val.Sequence.Size())
	case runtime.ValSet:
		return fmt.Sprintf("Set{%d}", val.Set.Size())
	default:
		return "<unknown>"
	}
}
