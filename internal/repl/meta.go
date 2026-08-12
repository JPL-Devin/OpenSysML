package repl

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/export"
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

// parseArgs splits a command line into arguments, handling quoted strings.
// This allows file paths and expressions with spaces to be properly parsed.
// Example: `%load "path with spaces/file.sysml"` -> ["%load", "path with spaces/file.sysml"]
func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for _, r := range line {
		switch {
		case escaped:
			// Previous char was backslash - add this char literally
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			// Escape next character
			escaped = true
		case r == '"':
			// Toggle quote mode
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			// Whitespace outside quotes - end current arg
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			// Regular character - add to current arg
			current.WriteRune(r)
		}
	}

	// Add final argument if any
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

var helpText = []string{
	"%help               show this help",
	"%list               list current session declarations",
	"%clear              reset the session",
	"%load <file>        read a file and submit its contents",
	"%save <file>        write the session model to a file (.sysml notation or .ttl RDF)",
	"%verbosity [level]  show or set output level: quiet, normal or debug",
	"%trace [on|off]     show or set execution tracing (evaluation, calc, action and state steps)",
	"%quit               exit the REPL",
	"",
	"Runtime commands:",
	"%instantiate <name> create an instance of a part def",
	"%eval <expr>        evaluate an expression",
	"%slots <name>       show instance slots and values",
	"%instances          list all instantiated objects",
	"",
	"Behavioral commands:",
	"%calc <name> <args> invoke a calculation with arguments",
	"%constraint <name>  evaluate a constraint definition",
	"%requirement <name> evaluate a requirement definition",
	"",
	"Action debugging:",
	"%action <name>      start action executor debugging session",
	"%step               advance one token step",
	"%continue           run action to completion",
	"%tokens             show active tokens",
	"%break <node>       set breakpoint at node",
	"%stop               stop current debugging session",
	"",
	"State machine debugging:",
	"%state <name>       start state machine debugging session",
	"%events             show event queue",
	"%current            show current state and configuration",
	"%advance <time>     advance simulation time by <time> units, processing every event due",
}

// runMeta executes a meta command line. Returns lines to print, whether to quit,
// and an error only for unrecoverable I/O (unknown commands print guidance).
// RunMeta executes a meta-command (e.g., %eval, %load) and returns the output lines,
// a quit flag, and any error encountered.
func (s *Session) RunMeta(line string) (out []string, quit bool, err error) {
	out, quit, err = s.runMeta(line)
	return append(s.drainTrace(), out...), quit, err
}

func (s *Session) runMeta(line string) (out []string, quit bool, err error) {
	fields := parseArgs(strings.TrimSpace(line))
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
		return renderResult(s.Submit(string(data)), s.verbosity), false, nil
	case "%save":
		if len(fields) < 2 {
			return []string{"usage: %save <file.sysml|file.ttl>"}, false, nil
		}
		return s.doSave(fields[1])
	case "%verbosity":
		if len(fields) < 2 {
			return []string{fmt.Sprintf("verbosity: %s", s.verbosity)}, false, nil
		}
		v, verr := ParseVerbosity(fields[1])
		if verr != nil {
			return []string{"error: " + verr.Error()}, false, nil
		}
		s.SetVerbosity(v)
		return []string{fmt.Sprintf("verbosity: %s", v)}, false, nil
	case "%trace":
		if len(fields) >= 2 {
			switch fields[1] {
			case "on":
				s.SetTracing(true)
			case "off":
				s.SetTracing(false)
			default:
				return []string{fmt.Sprintf("error: unknown trace setting %q (want on or off)", fields[1])}, false, nil
			}
		}
		return []string{fmt.Sprintf("trace: %s", onOff(s.Tracing()))}, false, nil
	case "%quit", "%exit":
		return []string{"goodbye"}, true, nil
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
	case "%calc":
		if len(fields) < 2 {
			return []string{"usage: %calc <name> [args...]"}, false, nil
		}
		return s.doCalc(fields[1:])
	case "%constraint":
		if len(fields) < 2 {
			return []string{"usage: %constraint <name>"}, false, nil
		}
		return s.doConstraint(fields[1])
	case "%requirement":
		if len(fields) < 2 {
			return []string{"usage: %requirement <name>"}, false, nil
		}
		return s.doRequirement(fields[1])
	// Action debugging
	case "%action":
		if len(fields) < 2 {
			return []string{"usage: %action <name>"}, false, nil
		}
		return s.doAction(fields[1])
	case "%step":
		return s.doStep()
	case "%continue":
		return s.doContinue()
	case "%tokens":
		return s.doTokens()
	case "%break":
		if len(fields) < 2 {
			return []string{"usage: %break <node>"}, false, nil
		}
		return s.doBreak(fields[1])
	case "%stop":
		return s.doStop()
	// State machine debugging
	case "%state":
		if len(fields) < 2 {
			return []string{"usage: %state <name>"}, false, nil
		}
		return s.doStateMachine(fields[1])
	case "%events":
		return s.doEvents()
	case "%current":
		return s.doCurrent()
	case "%advance":
		if len(fields) < 2 {
			return []string{"usage: %advance <time>"}, false, nil
		}
		return s.doAdvance(fields[1])
	default:
		return []string{fmt.Sprintf("unknown command %q (try %%help)", fields[0])}, false, nil
	}
}

// doInstantiate creates an instance of a part def.
// doSave writes the session's model to path. The format follows the file
// extension: `.sysml`/`.kerml` writes the notation, `.ttl` writes RDF Turtle.
func (s *Session) doSave(path string) ([]string, bool, error) {
	src := s.joined()
	if strings.TrimSpace(src) == "" {
		return []string{"nothing to save: the session is empty"}, false, nil
	}
	format, err := export.FormatOfPath(path)
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	// Diagnostics are positions in the session buffer, not in the file about to
	// be written, so they are labelled as such.
	out, err := export.Convert(sessionOrigin, []byte(src), export.FormatSysML, format)
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return nil, false, fmt.Errorf("save %s: %w", path, err)
	}
	return []string{fmt.Sprintf("saved %d bytes of %s to %s", len(out), format, path)}, false, nil
}

func (s *Session) doInstantiate(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}

	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: instantiation failed: %v", err)}, false, nil
	}

	// Keyed by the resolved name, so %slots finds the instance whichever
	// spelling of the name created it.
	s.instances[fqn] = inst
	return []string{
		fmt.Sprintf("✓ Created instance of %s", fqn),
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

	// Try feature reference lookup, simple ("%eval x") or qualified
	// ("%eval Demo::Vehicle::mass").
	if isSymbolReference(expr) {
		sym, fqn, lerr := s.lookupSymbol(expr)
		if lerr != nil {
			return []string{"error: " + lerr.Error()}, false, nil
		}
		// An instantiated owner makes this a question about that object: read
		// the slot, which carries the value the instance actually holds.
		if inst, owner := s.owningInstance(fqn); inst != nil {
			if _, ok := inst.Slots[sym.Name]; ok {
				slot, err := inst.GetSlot(ctx, sym.Name)
				if err != nil {
					return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
				}
				return []string{
					fmt.Sprintf("✓ %s%s", expr, onInstance(inst, owner)),
					fmt.Sprintf("  = %s", formatSlot(slot)),
				}, false, nil
			}
		}
		usage, ok := sym.Decl.(*ast.Usage)
		if !ok || usage.Value == nil {
			return []string{fmt.Sprintf("error: %q has no value to evaluate", expr)}, false, nil
		}
		// Evaluate with the symbol's owner scope for proper name resolution
		val, err := ctx.EvalWithScope(usage.Value, sym.OwnerScope)
		if err != nil {
			return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
		}
		return []string{
			fmt.Sprintf("✓ %s", expr),
			fmt.Sprintf("  = %s", formatValue(val)),
		}, false, nil
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
		member := root.Members[i]
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		if usage, ok := member.(*ast.Usage); ok && usage.Value != nil {
			if usage.Ident.Name == "__eval__" || usage.Ident.ShortName == "__eval__" {
				evalUsage = usage
				break
			}
		}
	}

	if evalUsage == nil || evalUsage.Value == nil {
		return []string{"error: could not parse expression"}, false, nil
	}

	// Evaluated against the document scope, so a compound expression can name
	// the session's top-level features.
	val, err := ctx.EvalWithScope(evalUsage.Value, doc.Scope)
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

	member := root.Members[0]
	// Unwrap Membership if present
	if mem, ok := member.(*ast.Membership); ok {
		member = mem.Member
	}

	usage, ok := member.(*ast.Usage)
	if !ok || usage.Value == nil {
		return nil, false
	}

	// Use runtime context with empty model (no symbols needed for literals)
	emptyIdx := symbols.NewIndex()
	emptyModel := semantics.NewModel(resolve.New(emptyIdx))
	ctx := runtime.NewContext(emptyModel, resolve.New(emptyIdx), s.budgets.MaxSteps)

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

// isSymbolReference reports whether expr names a symbol — a single identifier
// or a qualified name like Demo::Vehicle::mass — rather than a compound
// expression.
func isSymbolReference(expr string) bool {
	expr = strings.TrimSpace(expr)
	if len(expr) == 0 {
		return false
	}
	// Simple heuristic: no spaces, operators, or parens
	for _, ch := range expr {
		if ch == ' ' || ch == '+' || ch == '-' || ch == '*' || ch == '/' ||
			ch == '(' || ch == ')' || ch == '.' {
			return false
		}
	}
	// A ':' is only meaningful here as part of the '::' qualifier separator.
	return !strings.Contains(strings.ReplaceAll(expr, "::", ""), ":")
}

// doSlots shows instance slots.
func (s *Session) doSlots(name string) ([]string, bool, error) {
	_, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	inst, ok := s.instances[fqn]
	if !ok {
		return []string{fmt.Sprintf("error: no instance of %q (use %%instantiate first)", fqn)}, false, nil
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}

	lines := []string{
		fmt.Sprintf("Instance: %s (ID: %d)", fqn, inst.ID),
		"Slots:",
	}
	w := &slotWalk{ctx: ctx, onPath: map[*symbols.Symbol]bool{inst.Type: true}, budget: maxSlotLines}
	return append(lines, w.lines(inst, "  ", 0)...), false, nil
}

const (
	// maxSlotDepth bounds how deep %slots expands nested objects.
	maxSlotDepth = 8
	// maxSlotLines bounds the listing as a whole, since nesting multiplies and
	// a slot is materialized by reading it: breadth costs objects, not just output.
	maxSlotLines = 200
)

// slotWalk expands an object graph for %slots under three bounds: onPath holds
// the types being expanded above the current one (a part containing its own
// kind materializes a fresh instance per descent, so instance identity cannot
// detect the cycle), depth, and a line budget shared across the listing.
type slotWalk struct {
	ctx    *runtime.Context
	onPath map[*symbols.Symbol]bool
	budget int
}

func (w *slotWalk) lines(inst *runtime.Instance, indent string, depth int) []string {
	features := w.ctx.FeaturesOf(inst.Type)
	if len(features) == 0 {
		return w.emit(nil, indent+"(no features)")
	}

	var lines []string
	for i := range features {
		if w.budget <= 0 {
			return append(lines, indent+"… (listing truncated)")
		}
		feat := &features[i]
		// A constraint or requirement the part carries has no value; what it has
		// is a verdict about this instance, which is the useful thing to show.
		if verdict, ok := featureVerdict(w.ctx, feat, inst); ok {
			lines = w.emit(lines, fmt.Sprintf("%s%s: %s", indent, feat.Name, verdict))
			continue
		}
		if held, elided := w.elided(feat, depth); elided {
			lines = w.emit(lines, fmt.Sprintf("%s%s : %s (not expanded: %s)", indent, feat.Name, held, elisionReason(depth)))
			continue
		}
		slot, err := inst.GetSlot(w.ctx, feat.Name)
		if err != nil {
			lines = w.emit(lines, fmt.Sprintf("%s%s: <error: %v>", indent, feat.Name, err))
			continue
		}
		lines = w.emit(lines, fmt.Sprintf("%s%s = %s", indent, feat.Name, formatSlot(slot)))
		for _, nested := range nestedInstances(w.ctx, slot) {
			if w.budget <= 0 {
				return append(lines, indent+"  … (listing truncated)")
			}
			w.onPath[nested.Type] = true
			lines = append(lines, w.lines(nested, indent+"  ", depth+1)...)
			delete(w.onPath, nested.Type)
		}
	}
	return lines
}

func (w *slotWalk) emit(lines []string, line string) []string {
	w.budget--
	return append(lines, line)
}

// elided reports whether expanding a feature would revisit a type already on
// the path or exceed the depth bound, naming the type it holds. Asked before
// the slot is read, since reading it materializes the object.
func (w *slotWalk) elided(feat *runtime.EffectiveFeature, depth int) (string, bool) {
	held := w.ctx.CompositeTypeOf(feat)
	if held == nil {
		return "", false
	}
	if depth >= maxSlotDepth || w.onPath[held] {
		return held.Name, true
	}
	return "", false
}

func elisionReason(depth int) string {
	if depth >= maxSlotDepth {
		return fmt.Sprintf("depth %d", maxSlotDepth)
	}
	return "contains its own kind"
}

// nestedInstances returns the instances a slot holds, whether it carries one
// value or a collection of them.
func nestedInstances(ctx *runtime.Context, slot *runtime.Slot) []*runtime.Instance {
	values := []runtime.Value{slot.Value}
	switch slot.Values.Kind {
	case runtime.ValSequence:
		values = slot.Values.Sequence.Elements()
	case runtime.ValSet:
		values = slot.Values.Set.Elements()
	}

	var out []*runtime.Instance
	for _, val := range values {
		if val.Kind != runtime.ValInstance {
			continue
		}
		if nested, ok := ctx.Instance(val.Instance); ok {
			out = append(out, nested)
		}
	}
	return out
}

// featureVerdict evaluates a constraint or requirement feature against the
// instance that carries it and renders the outcome for a slot listing.
// Reports false for a feature that holds a value rather than a verdict.
func featureVerdict(ctx *runtime.Context, feat *runtime.EffectiveFeature, inst *runtime.Instance) (string, bool) {
	if feat.Symbol == nil {
		return "", false
	}
	var (
		kind   string
		passed bool
		err    error
	)
	switch feat.Symbol.Kind {
	case symbols.SymbolConstraintUsage:
		kind = "constraint"
		passed, err = ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
	case symbols.SymbolRequirementUsage:
		kind = "requirement"
		passed, err = ctx.EvaluateRequirementOn(feat.Symbol, feat.DeclScope(), inst)
	default:
		return "", false
	}
	switch {
	case err != nil && !errors.Is(err, runtime.ErrViolated):
		return fmt.Sprintf("<%s: %v>", kind, err), true
	case err != nil || !passed:
		return fmt.Sprintf("<%s: violated>", kind), true
	default:
		return fmt.Sprintf("<%s: satisfied>", kind), true
	}
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

// formatSlot renders what a slot holds: a multi-valued feature keeps its
// contents in Values, leaving the scalar Value unset.
func formatSlot(slot *runtime.Slot) string {
	if slot.Values.Kind != runtime.ValInvalid {
		return formatValue(slot.Values)
	}
	return formatValue(slot.Value)
}

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
		return formatElements(val.Sequence.Elements())
	case runtime.ValSet:
		return fmt.Sprintf("Set{%d}", val.Set.Size())
	default:
		return "<unknown>"
	}
}

// formatElements renders a collection's contents, since its size alone answers
// nothing about what the object holds.
func formatElements(elements []runtime.Value) string {
	parts := make([]string, len(elements))
	for i, el := range elements {
		parts[i] = formatValue(el)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// doCalc invokes a calculation with arguments.
func (s *Session) doCalc(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return []string{"usage: %calc <name> [args...]"}, false, nil
	}

	calcName := args[0]
	calcArgs := args[1:]

	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}

	sym, _, lerr := s.lookupSymbol(calcName)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}

	// Parse arguments as literal expressions (no session context needed)
	argValues := make([]runtime.Value, len(calcArgs))
	for i, argStr := range calcArgs {
		// Use empty context for literal evaluation
		emptyIdx := symbols.NewIndex()
		emptyModel := semantics.NewModel(resolve.New(emptyIdx))
		literalCtx := runtime.NewContext(emptyModel, resolve.New(emptyIdx), s.budgets.MaxSteps)

		// Parse as attribute inside a part (top-level attribute syntax not supported)
		src := fmt.Sprintf("part __dummy__ { attribute __arg__ = %s; }", argStr)
		p := parser.New(source.New("arg", []byte(src)))
		root := p.ParseFile()

		// Ignore parse diagnostics - literals might have unresolved types
		if len(root.Members) == 0 {
			return []string{fmt.Sprintf("error: failed to parse argument %q", argStr)}, false, nil
		}

		// Unwrap Membership if present
		member := root.Members[0]
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}

		// Extract attribute from part body
		partUsage, ok := member.(*ast.Usage)
		if !ok || partUsage.Kind != ast.UsagePart {
			return []string{fmt.Sprintf("error: argument %q: not a part usage", argStr)}, false, nil
		}

		if len(partUsage.Members) == 0 {
			return []string{fmt.Sprintf("error: argument %q: empty part body", argStr)}, false, nil
		}

		// Unwrap first member (attribute)
		attrMember := partUsage.Members[0]
		if attrMembership, ok := attrMember.(*ast.Membership); ok {
			attrMember = attrMembership.Member
		}

		usage, ok := attrMember.(*ast.Usage)
		if !ok {
			return []string{fmt.Sprintf("error: argument %q: first member not usage", argStr)}, false, nil
		}

		if usage.Value == nil {
			return []string{fmt.Sprintf("error: argument %q: usage has no value", argStr)}, false, nil
		}

		val, err := literalCtx.Eval(usage.Value)
		if err != nil {
			return []string{fmt.Sprintf("error: evaluation of argument %q failed: %v", argStr, err)}, false, nil
		}
		argValues[i] = val
	}

	// Invoke calculation via InvokeCalc
	result, err := ctx.InvokeCalc(sym, argValues, doc.Scope)
	if err != nil {
		return []string{fmt.Sprintf("error: calc invocation failed: %v", err)}, false, nil
	}

	return []string{
		fmt.Sprintf("✓ %s(%s)", calcName, strings.Join(calcArgs, ", ")),
		fmt.Sprintf("  = %s", formatValue(result)),
	}, false, nil
}

// doConstraint evaluates a constraint definition.
func (s *Session) doConstraint(name string) ([]string, bool, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}

	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}

	// Evaluate against the instance that carries the constraint when one has
	// been created, so the verdict is about concrete values.
	inst, owner := s.owningInstance(fqn)
	scope := doc.Scope
	if inst != nil {
		scope = sym.OwnerScope
	}
	passed, err := ctx.EvaluateConstraintOn(sym, scope, inst)
	if err != nil || !passed {
		return []string{
			fmt.Sprintf("✗ Constraint %s failed%s", name, onInstance(inst, owner)),
			"  " + verdictDetail("Assertion", err),
		}, false, nil
	}

	return []string{
		fmt.Sprintf("✓ Constraint %s passed%s", name, onInstance(inst, owner)),
	}, false, nil
}

// verdictDetail explains a failed verdict: a condition that evaluated to false
// is the model's answer, not a malfunction, so it is not an error line. what
// names the kind of condition, e.g. "Assertion" or "Required condition".
func verdictDetail(what string, err error) string {
	var violation *runtime.ViolationError
	switch {
	case errors.As(err, &violation):
		return fmt.Sprintf("%s evaluated to false: %s", what, violation.Condition)
	case err == nil || errors.Is(err, runtime.ErrViolated):
		return what + " evaluated to false"
	}
	return fmt.Sprintf("Error: %v", err)
}

// onInstance renders the " (on <owner> ID: n)" suffix that marks a result as
// being about one object rather than about declared defaults.
func onInstance(inst *runtime.Instance, owner string) string {
	if inst == nil {
		return ""
	}
	return fmt.Sprintf(" (on %s ID: %d)", owner, inst.ID)
}

// doRequirement evaluates a requirement definition.
func (s *Session) doRequirement(name string) ([]string, bool, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}

	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}

	inst, owner := s.owningInstance(fqn)
	scope := doc.Scope
	if inst != nil {
		scope = sym.OwnerScope
	}
	passed, err := ctx.EvaluateRequirementOn(sym, scope, inst)
	if err != nil || !passed {
		return []string{
			fmt.Sprintf("✗ Requirement %s failed%s", name, onInstance(inst, owner)),
			"  " + verdictDetail("Required condition", err),
		}, false, nil
	}

	return []string{
		fmt.Sprintf("✓ Requirement %s satisfied%s", name, onInstance(inst, owner)),
	}, false, nil
}

// --- Action Debugging Commands ---

// doAction starts an action executor debugging session.
func (s *Session) doAction(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}

	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	if sym.Kind != symbols.SymbolActionUsage && sym.Kind != symbols.SymbolActionDef {
		return []string{fmt.Sprintf("error: %q is not an action", name)}, false, nil
	}

	// Create executor
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: failed to create executor: %v", err)}, false, nil
	}
	exec.SetTrace(s.trace)

	// Store session
	s.actionExec = &actionSession{
		name:     name,
		rootName: rootNameOf(fqn, name),
		symbol:   sym,
		executor: exec,
	}

	// Display initial state
	tokens := exec.Tokens()
	return []string{
		fmt.Sprintf("✓ Started action executor for %q", name),
		fmt.Sprintf("  State: %s", exec.State()),
		fmt.Sprintf("  Tokens: %d", len(tokens)),
		"",
		"Use %step to advance, %tokens to inspect, %continue to run to completion",
	}, false, nil
}

// doStep advances the action executor one step.
func (s *Session) doStep() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}

	exec := s.actionExec.executor

	// Check if already completed
	if exec.State() == runtime.StateCompleted {
		return []string{"✓ Action already completed"}, false, nil
	}

	// Step
	err := exec.Step()
	if err != nil {
		return []string{fmt.Sprintf("error: step failed: %v", err)}, false, nil
	}

	// Display state
	tokens := exec.Tokens()
	out := []string{
		"✓ Step complete",
		fmt.Sprintf("  State: %s", exec.State()),
		fmt.Sprintf("  Tokens: %d", len(tokens)),
	}

	if exec.State() == runtime.StateCompleted {
		results := exec.Results()
		out = append(out, "", "✓ Action completed")
		if len(results) > 0 {
			out = append(out, "  Results:")
			for k, v := range results {
				out = append(out, fmt.Sprintf("    %s = %s", k, formatValue(v)))
			}
		}
	}

	return out, false, nil
}

// doContinue runs the action to completion.
func (s *Session) doContinue() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}

	exec := s.actionExec.executor

	// Check if already completed
	if exec.State() == runtime.StateCompleted {
		return []string{"✓ Action already completed"}, false, nil
	}

	// Run to completion, or to the first breakpoint hit
	err := exec.RunToCompletion()
	if err != nil {
		return []string{fmt.Sprintf("error: execution failed: %v", err)}, false, nil
	}

	if node := exec.PausedAt(); node != "" {
		out := []string{
			fmt.Sprintf("⏸ Paused at breakpoint %q", node),
			fmt.Sprintf("  State: %s", exec.State()),
			fmt.Sprintf("  Tokens: %d", len(exec.Tokens())),
			"",
			"Use %tokens to inspect, %step or %continue to resume",
		}
		return out, false, nil
	}

	// Display results
	results := exec.Results()
	out := []string{
		"✓ Action completed",
		fmt.Sprintf("  Final state: %s", exec.State()),
	}

	if len(results) > 0 {
		out = append(out, "  Results:")
		for k, v := range results {
			out = append(out, fmt.Sprintf("    %s = %s", k, formatValue(v)))
		}
	}

	return out, false, nil
}

// doTokens displays active tokens.
func (s *Session) doTokens() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}

	exec := s.actionExec.executor
	tokens := exec.Tokens()

	if len(tokens) == 0 {
		return []string{"No active tokens"}, false, nil
	}

	out := []string{fmt.Sprintf("Active tokens (%d):", len(tokens))}
	for _, tok := range tokens {
		locName := runtime.ActionNodeName(tok.Location)
		if locName == "" {
			locName = anonymousNodeLabel(tok.Location)
		}

		out = append(out, fmt.Sprintf("  Token %d @ %s", tok.ID, locName))
		if len(tok.Data) > 0 {
			for k, v := range tok.Data {
				out = append(out, fmt.Sprintf("    %s = %s", k, formatValue(v)))
			}
		}
	}

	return out, false, nil
}

// anonymousNodeLabel describes a node that declares no name, by kind.
func anonymousNodeLabel(node ast.Node) string {
	switch node.(type) {
	case *ast.InitialNode:
		return "<initial>"
	case *ast.FinalNode:
		return "<final>"
	case *ast.ForkNode:
		return "<fork>"
	case *ast.JoinNode:
		return "<join>"
	case *ast.MergeNode:
		return "<merge>"
	case *ast.DecisionNode:
		return "<decision>"
	case *ast.ActionExecutionNode:
		return "<action>"
	case nil:
		return "<none>"
	default:
		return "<anonymous>"
	}
}

// doBreak sets a breakpoint at a named node of the running action.
func (s *Session) doBreak(nodeName string) ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}

	exec := s.actionExec.executor
	names := exec.NodeNames()
	if !slices.Contains(names, nodeName) {
		out := []string{fmt.Sprintf("error: action %q has no node named %q", s.actionExec.name, nodeName)}
		if len(names) > 0 {
			out = append(out, "  nodes: "+strings.Join(names, ", "))
		}
		return out, false, nil
	}
	exec.SetBreakpoint(nodeName)

	return []string{
		fmt.Sprintf("✓ Breakpoint set at node %q", nodeName),
		"  %continue runs until a token reaches it",
	}, false, nil
}

// doStop stops the current debugging session.
func (s *Session) doStop() ([]string, bool, error) {
	if s.actionExec == nil && s.stateExec == nil {
		return []string{"error: no active debugging session"}, false, nil
	}

	sessionName := ""
	if s.actionExec != nil {
		sessionName = s.actionExec.name
		s.actionExec = nil
	} else if s.stateExec != nil {
		sessionName = s.stateExec.name
		s.stateExec = nil
	}

	return []string{fmt.Sprintf("✓ Stopped debugging session for %q", sessionName)}, false, nil
}

// --- State Machine Debugging Commands ---

// doStateMachine starts a state machine executor debugging session.
func (s *Session) doStateMachine(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}

	sym, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		return []string{"error: " + lerr.Error()}, false, nil
	}

	if sym.Kind != symbols.SymbolStateDef && sym.Kind != symbols.SymbolStateUsage {
		return []string{fmt.Sprintf("error: %q is not a state machine", name)}, false, nil
	}

	// Create executor
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: failed to create executor: %v", err)}, false, nil
	}
	exec.SetTrace(s.trace)

	// Store session
	s.stateExec = &stateSession{
		name:     name,
		rootName: rootNameOf(fqn, name),
		symbol:   sym,
		executor: exec,
		now:      exec.CurrentTime(),
	}

	return []string{
		fmt.Sprintf("✓ Started state machine executor for %q", name),
		fmt.Sprintf("  Current state: %s", currentStateName(exec)),
		fmt.Sprintf("  Time: %.2f", exec.CurrentTime()),
		fmt.Sprintf("  Events: %d", exec.EventQueue().Len()),
		"",
		"Use %events to see queue, %current for state, %advance <time> to step",
	}, false, nil
}

// doEvents displays the event queue.
func (s *Session) doEvents() ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{"error: no active state machine session (use %state <name> first)"}, false, nil
	}

	exec := s.stateExec.executor
	queue := exec.EventQueue()

	if queue.Len() == 0 {
		return []string{"Event queue empty"}, false, nil
	}

	// Note: EventQueue doesn't expose events directly, so just show count
	return []string{
		fmt.Sprintf("Event queue: %d events", queue.Len()),
		"Use %advance <time> to process next event",
	}, false, nil
}

// doCurrent shows current state and configuration.
func (s *Session) doCurrent() ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{"error: no active state machine session (use %state <name> first)"}, false, nil
	}

	exec := s.stateExec.executor
	stateStack := exec.StateStack()
	stateData := exec.StateData()

	out := []string{
		fmt.Sprintf("Current state: %s", currentStateName(exec)),
		fmt.Sprintf("Time: %.2f", s.stateExec.now),
		fmt.Sprintf("Last event at: %.2f", exec.CurrentTime()),
		fmt.Sprintf("Execution state: %s", exec.State()),
	}

	if len(stateStack) > 1 {
		out = append(out, "", "State stack (active configuration):")
		for i, stateNode := range stateStack {
			if stateNode.Name != "" {
				out = append(out, fmt.Sprintf("  %d. %s", i, stateNode.Name))
			}
		}
	}

	if len(stateData) > 0 {
		out = append(out, "", "State data:")
		for k, v := range stateData {
			out = append(out, fmt.Sprintf("  %s = %s", k, formatValue(v)))
		}
	}

	return out, false, nil
}

// currentStateName renders the machine's active configuration: the active
// state's name, or one name per orthogonal region.
func currentStateName(exec *runtime.StateExecutor) string {
	active := exec.ActiveStates()
	if len(active) == 0 {
		return "<none>"
	}
	names := make([]string, 0, len(active))
	for _, state := range active {
		if state.Name != "" {
			names = append(names, state.Name)
		} else {
			names = append(names, "<anonymous>")
		}
	}
	return strings.Join(names, " | ")
}

// parseDuration reads the argument of %advance: a number of time units, with
// an optional trailing "s".
func parseDuration(arg string) (float64, error) {
	text := strings.TrimSuffix(strings.TrimSpace(arg), "s")
	d, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q (expected a number of time units, e.g. 30 or 30s)", arg)
	}
	if math.IsNaN(d) || math.IsInf(d, 0) {
		return 0, fmt.Errorf("invalid time %q (expected a finite number of time units, e.g. 30 or 30s)", arg)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid time %q (must not be negative)", arg)
	}
	return d, nil
}

// doAdvance advances simulation time by the given duration, processing every
// event scheduled at or before the deadline.
func (s *Session) doAdvance(timeStr string) ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{"error: no active state machine session (use %state <name> first)"}, false, nil
	}

	duration, err := parseDuration(timeStr)
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}

	exec := s.stateExec.executor
	deadline := s.stateExec.now + duration

	// A state's do behavior is work too: the machine can have none queued yet
	// still have somewhere to go, and its completion transition is queued once
	// the behavior ends.
	if !exec.HasPendingWork() {
		s.stateExec.now = deadline
		return []string{fmt.Sprintf("No pending work - simulation time is now %.2f", deadline)}, false, nil
	}

	// Bound the drain by the session's own budgets, so a machine that keeps
	// queueing work cannot hang the REPL and the way to raise the bound is the
	// same one the executors report.
	maxEvents, maxDoActions := s.budgets.MaxStateEvents, s.budgets.MaxDoSteps
	var processed, doActions int64
	for exec.HasPendingWork() && exec.State() == runtime.StateRunning &&
		processed < maxEvents && doActions < maxDoActions {
		if queue := exec.EventQueue(); queue.Len() == 0 || queue.Peek().Timestamp > deadline {
			// Nothing to dispatch within the deadline, but a do behavior with
			// actions left is due now, so run it and count it as do work.
			if !exec.HasPendingDoWork() {
				break
			}
			ran, err := exec.RunDoRound()
			if err != nil {
				return []string{fmt.Sprintf("error: do behavior failed: %v", err)}, false, nil
			}
			if ran == 0 {
				break
			}
			doActions += int64(ran)
			continue
		}
		if err := exec.ProcessNextEvent(); err != nil {
			return []string{fmt.Sprintf("error: event processing failed: %v", err)}, false, nil
		}
		processed++
	}
	s.stateExec.now = math.Max(deadline, exec.CurrentTime())

	out := []string{
		fmt.Sprintf("✓ Advanced to %.2f (%d event(s) processed)", s.stateExec.now, processed),
		fmt.Sprintf("  Current state: %s", currentStateName(exec)),
		fmt.Sprintf("  Last event at: %.2f", exec.CurrentTime()),
		fmt.Sprintf("  Remaining events: %d", exec.EventQueue().Len()),
	}

	if doActions > 0 {
		out = append(out, fmt.Sprintf("  Do behavior actions run: %d", doActions))
	}

	// A drain the bound cut short has work left, so say so rather than let it
	// read as a machine that settled.
	switch {
	case processed >= maxEvents:
		out = append(out, fmt.Sprintf("  Stopped at the event budget (%d events; raise %s to allow more)",
			maxEvents, runtime.MaxStateEventsEnvVar))
	case doActions >= maxDoActions:
		out = append(out, fmt.Sprintf("  Stopped at the do activity budget (%d steps; raise %s to allow more)",
			maxDoActions, runtime.MaxDoStepsEnvVar))
	}

	if exec.State() == runtime.StateCompleted {
		out = append(out, "", "✓ State machine completed (final state reached)")
	}

	return out, false, nil
}
