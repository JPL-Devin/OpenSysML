package repl

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/model"
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
	"%budget             show the bounds one run may spend, and the variable raising each",
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
	"%satisfy [name]     evaluate the satisfaction assertions of the model, or of one element",
	"",
	"Action debugging:",
	"%action <name> [<object>]  start action executor debugging session, performed by an object",
	"%step               advance one token step",
	"%continue           run action to completion",
	"%tokens             show active tokens",
	"%break <node>       set breakpoint at node",
	"%stop               stop current debugging session",
	"",
	"State machine debugging:",
	"%state <name> [<object>]   start state machine debugging session, performed by an object",
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
		data, rerr := os.ReadFile(expandHome(fields[1]))
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
	case "%budget":
		return s.doBudget(), false, nil
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
		name, argText := splitCalcArgs(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "%calc")))
		return s.doCalc(name, argText)
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
	case "%satisfy":
		return s.doSatisfy(fields[1:])
	// Action debugging
	case "%action":
		if len(fields) < 2 {
			return []string{"usage: %action <name> [<object>]"}, false, nil
		}
		return s.doAction(fields[1], fields[2:])
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
			return []string{"usage: %state <name> [<object>]"}, false, nil
		}
		return s.doStateMachine(fields[1], fields[2:])
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
	// ("%eval Demo::Vehicle::mass"). A name the session did not declare may
	// still be reachable through an import, which the expression path below
	// resolves, so a lookup failure is only reported if that path fails too.
	var (
		sym       *symbols.Symbol
		fqn       string
		lookupErr error
	)
	if isSymbolReference(expr) {
		sym, fqn, lookupErr = s.lookupSymbol(expr)
		// A name several declarations answer to is reported, never resolved to
		// whichever of them the prompt's scope happens to reach.
		var ambiguous *AmbiguousNameError
		if errors.As(lookupErr, &ambiguous) {
			return []string{"error: " + lookupErr.Error()}, false, nil
		}
	}
	if sym != nil {
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
		if lookupErr != nil {
			return []string{"error: " + lookupErr.Error()}, false, nil
		}
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

	// Evaluated in the namespace the session is working in, so a compound
	// expression names what a member written there would: the session's
	// features and the units its imports bring in.
	val, err := ctx.EvalWithScope(evalUsage.Value, s.promptScope(doc))
	if err != nil {
		if lookupErr != nil {
			return []string{"error: " + lookupErr.Error()}, false, nil
		}
		return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
	}

	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(val)),
	}, false, nil
}

// tryEvalLiteral attempts to evaluate standalone literal expressions.
func (s *Session) tryEvalLiteral(expr string) ([]string, bool) {
	// A name the session declares is answered by that declaration, so the empty
	// model this pass evaluates in must not answer for it: a library operation
	// reached by its unqualified name would otherwise stand in for a calc the
	// session wrote under the same name.
	if s.declaresANameIn(expr) {
		return nil, false
	}

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
	if err := ctx.SetBudgets(s.budgets); err != nil {
		return nil, false
	}

	val, err := ctx.Eval(usage.Value)
	if err != nil {
		// A failure the session's declarations could answer — a name or a unit
		// this empty model knows nothing of — is not the answer, so the
		// expression is evaluated again in the session. A failure that is the
		// answer to an expression of literals alone, such as an index that names
		// no position, is reported here instead of being hidden behind "no
		// declarations loaded".
		if isLiteralAnswerError(err) {
			return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, true
		}
		return nil, false
	}

	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(val)),
	}, true
}

// doBudget lists the bounds one run of this session may spend, each with the
// variable that raises it.
func (s *Session) doBudget() []string {
	b := s.Budgets()
	return []string{
		"budgets (each bounds one run, not the session):",
		fmt.Sprintf("  evaluation steps     %-10d %s", b.MaxSteps, runtime.MaxStepsEnvVar),
		fmt.Sprintf("  action steps         %-10d %s", b.MaxActionSteps, runtime.MaxActionStepsEnvVar),
		fmt.Sprintf("  state events         %-10d %s", b.MaxStateEvents, runtime.MaxStateEventsEnvVar),
		fmt.Sprintf("  do activity steps    %-10d %s", b.MaxDoSteps, runtime.MaxDoStepsEnvVar),
		fmt.Sprintf("  collection elements  %-10d %s", b.MaxElements, runtime.MaxElementsEnvVar),
	}
}

// declaresANameIn reports whether the session declares any name the expression
// uses, in the document root or in the namespace a prompt expression is
// evaluated in.
func (s *Session) declaresANameIn(expr string) bool {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return false
	}
	scopes := []*symbols.Scope{doc.Scope}
	if prompt := s.promptScope(doc); prompt != nil && prompt != doc.Scope {
		scopes = append(scopes, prompt)
	}
	src := source.New("literal", []byte(expr))
	lx := lexer.New(src)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind != lexer.Identifier && tok.Kind != lexer.UnrestrictedName {
			continue
		}
		for _, scope := range scopes {
			if sym, ok := scope.LookupLocal(src.Text(tok.Span)); ok && sym != nil {
				return true
			}
		}
	}
	return false
}

// isLiteralAnswerError reports whether err is what an expression of literals
// alone evaluates to, rather than a failure the declarations of a session could
// answer. An index outside a written sequence, a body called with arguments it
// declares no parameters for, or an operand of the wrong kind is the answer
// whatever is declared; an unresolved name or unit is not.
func isLiteralAnswerError(err error) bool {
	for _, answer := range []error{
		runtime.ErrIndexOutOfRange,
		runtime.ErrBodyArity,
		runtime.ErrTypeMismatch,
		runtime.ErrMultiplicityViolation,
		runtime.ErrStepLimitExceeded,
		runtime.ErrElementLimitExceeded,
	} {
		if errors.Is(err, answer) {
			return true
		}
	}
	return false
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
	connectors := w.connectors(inst, indent)
	if len(features) == 0 && len(connectors) == 0 {
		return w.emit(nil, indent+"(no features)")
	}

	// Connector lines already spent their share of the budget, so a truncated
	// listing still shows them rather than dropping what it charged for.
	truncated := func(lines []string, pad string) []string {
		return append(append(lines, connectors...), indent+pad+"… (listing truncated)")
	}

	var lines []string
	for i := range features {
		if w.budget <= 0 {
			return truncated(lines, "")
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
				return truncated(lines, "  ")
			}
			w.onPath[nested.Type] = true
			lines = append(lines, w.lines(nested, indent+"  ", depth+1)...)
			delete(w.onPath, nested.Type)
		}
	}
	return append(lines, connectors...)
}

// connectors lists the connectors the object owns that no feature names: an
// anonymous `connect a.p to b.q` relates the features at its ends whether or not
// it is named, and its ends are the only way to see what it relates.
func (w *slotWalk) connectors(inst *runtime.Instance, indent string) []string {
	conns, err := inst.OwnedConnectors(w.ctx)
	if err != nil {
		return w.emit(nil, fmt.Sprintf("%s(anonymous connector): <error: %v>", indent, err))
	}
	var lines []string
	for _, conn := range conns {
		lines = w.emit(lines, fmt.Sprintf("%s(anonymous %s) = %s", indent, connectorKeyword(conn),
			formatValue(runtime.Value{Kind: runtime.ValInstance, Instance: conn.ID})))
		for _, end := range conn.Ends {
			if w.budget <= 0 {
				return append(lines, indent+"  … (listing truncated)")
			}
			lines = w.emit(lines, fmt.Sprintf("%s  %s = %s", indent, endLabel(end), formatValue(end.Value)))
		}
	}
	return lines
}

// connectorKeyword names the kind of connector an object materializes, for a
// connector that has no name to show.
func connectorKeyword(conn *runtime.Instance) string {
	if usage, ok := conn.Type.Decl.(*ast.Usage); ok && usage.Keyword != "" {
		return usage.Keyword
	}
	return "connector"
}

// endLabel names one end of a connector, by position when the model gives the
// end no name of its own.
func endLabel(end runtime.ConnectorEnd) string {
	if end.Name != "" {
		return end.Name
	}
	return "end"
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
		// A variation is materialized from the variant it selects, so the depth
		// bound applies to it too.
		if w.ctx.IsVariationFeature(feat) && depth >= maxSlotDepth {
			return feat.Name, true
		}
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
		id, ok := val.Object()
		if !ok {
			continue
		}
		if nested, ok := ctx.Instance(id); ok {
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
		return formatConst(val.Const)
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
	case runtime.ValVariant:
		// A selected variation shows the variant chosen, and the object it
		// materialized when it has one.
		if val.Variant == nil {
			return "<unknown variant>"
		}
		if val.Instance != 0 {
			return fmt.Sprintf("%s (Instance ID: %d)", val.Variant.Name, val.Instance)
		}
		return val.Variant.Name
	case runtime.ValQuantity:
		// A magnitude is a number like any other in a result table, so it is
		// rendered as a bare one; the value itself keeps its full precision.
		return val.Quantity.TextWithMagnitude(formatConst(val.Quantity.Num))
	default:
		return "<unknown>"
	}
}

// formatConst renders a numeric constant for a result table: a Real to two
// decimals, which is the session's convention for a displayed number.
func formatConst(c semantics.Value) string {
	switch c.Kind {
	case semantics.ValInt:
		return fmt.Sprintf("%d", c.Int)
	case semantics.ValReal:
		return fmt.Sprintf("%.2f", c.Real)
	case semantics.ValBool:
		return fmt.Sprintf("%v", c.Bool)
	case semantics.ValInfinity:
		return "∞"
	default:
		return "<unknown const>"
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

// doCalc invokes a calculation with the arguments the command line states.
// argText is the raw argument list: a sequence of expressions, which the
// notation separates with commas and this prompt also accepts separated by
// whitespace alone. Named arguments (`v0 = ...`) are not supported here — the
// notation writes them inside an invocation's parentheses, which is a different
// production than an argument list at a prompt.
//
// A calc usage named without arguments binds its inputs from its own members,
// so it is evaluated as a usage and every output feature it computes is listed
// from that one run (SysML 7.17).
func (s *Session) doCalc(calcName, argText string) ([]string, bool, error) {
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

	if strings.TrimSpace(argText) == "" {
		if lines, handled := s.calcUsageOutputs(ctx, sym, calcName); handled {
			return lines, false, nil
		}
	}

	exprs, err := parseExprList(argText)
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}

	// Arguments are evaluated where the prompt evaluates any expression, so a
	// quantity argument names the units the session's imports bring in.
	scope := s.promptScope(doc)
	argValues := make([]runtime.Value, len(exprs))
	argTexts := make([]string, len(exprs))
	for i, arg := range exprs {
		val, err := ctx.EvalWithScope(arg.expr, scope)
		if err != nil {
			return []string{fmt.Sprintf("error: evaluation of argument %q failed: %v", arg.text, err)}, false, nil
		}
		argValues[i] = val
		argTexts[i] = arg.text
	}

	result, err := ctx.InvokeCalc(sym, argValues, scope)
	if err != nil {
		return []string{fmt.Sprintf("error: calc invocation failed: %v", err)}, false, nil
	}

	return []string{
		fmt.Sprintf("✓ %s(%s)", calcName, strings.Join(argTexts, ", ")),
		fmt.Sprintf("  = %s", formatValue(result)),
	}, false, nil
}

// calcUsageOutputs lists the outputs of a calc usage evaluated from its own
// member values. It reports handled=false when the name is not a calc usage, or
// is one that computes no output features, so those keep being invoked as
// calculations with an empty argument list.
func (s *Session) calcUsageOutputs(ctx *runtime.Context, sym *symbols.Symbol, calcName string) ([]string, bool) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageCalc {
		return nil, false
	}
	outputs, err := ctx.CalcUsageOutputs(sym, sym.OwnerScope, nil)
	if err != nil {
		return []string{"error: calc usage evaluation failed: " + err.Error()}, true
	}
	if len(outputs) == 0 {
		return nil, false
	}
	lines := make([]string, 0, len(outputs)+1)
	lines = append(lines, fmt.Sprintf("✓ %s", calcName))
	for _, out := range outputs {
		lines = append(lines, fmt.Sprintf("  %s = %s", out.Name, formatValue(out.Value)))
	}
	return lines, true
}

// splitCalcArgs splits `%calc`'s tail into the calc's name and its argument
// list, accepting the invocation form `Fall(a, b)` as well as a bare name
// followed by arguments.
func splitCalcArgs(tail string) (name, argText string) {
	cut := strings.IndexAny(tail, " \t(")
	if cut < 0 {
		return tail, ""
	}
	name, rest := tail[:cut], strings.TrimSpace(tail[cut:])
	if closesAtEnd(rest) {
		return name, rest[1 : len(rest)-1]
	}
	return name, rest
}

// closesAtEnd reports whether text is one parenthesized group: its opening
// parenthesis is closed by its last character, as an invocation's argument list
// is and a sequence of parenthesized arguments is not.
func closesAtEnd(text string) bool {
	if !strings.HasPrefix(text, "(") || !strings.HasSuffix(text, ")") {
		return false
	}
	depth := 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(text)-1
			}
		}
	}
	return false
}

// argExpr is one parsed argument: its expression and the text it was written as.
type argExpr struct {
	expr ast.Node
	text string
}

// parseExprList parses an argument list as expressions, so an argument that
// contains spaces — a quantity, a parenthesized subexpression, a nested
// invocation — survives as the one expression it is.
func parseExprList(text string) ([]argExpr, error) {
	var out []argExpr
	for _, arg := range splitArgs(text) {
		if isNamedArgument(arg) {
			return nil, fmt.Errorf("named arguments are not supported here; pass arguments positionally")
		}
		expr, err := parseWholeExpr(arg)
		if err != nil {
			return nil, err
		}
		out = append(out, argExpr{expr: expr, text: arg})
	}
	return out, nil
}

// splitArgs cuts an argument list into one text per argument. A comma separates
// arguments, and so does whitespace — except where the fragment after it
// continues the expression before it, as a quantity's unit does.
func splitArgs(text string) []string {
	var args []string
	for _, group := range splitTopLevel(text) {
		var buf string
		for _, frag := range group {
			switch {
			case buf == "":
				buf = frag
			case continuesExpr(buf, frag):
				buf += " " + frag
			default:
				args = append(args, buf)
				buf = frag
			}
		}
		if buf != "" {
			args = append(args, buf)
		}
	}
	return args
}

// continuesExpr reports whether frag continues the expression buf holds rather
// than starting the next argument: a unit or index bracket does, as does a
// fragment that is no expression on its own or that follows an unfinished one
// (`5 - 3` is one argument, `5 -3` is two).
func continuesExpr(buf, frag string) bool {
	if strings.HasPrefix(frag, "[") || strings.HasPrefix(frag, "#") {
		return true
	}
	if _, err := parseWholeExpr(frag); err != nil {
		return true
	}
	_, err := parseWholeExpr(buf)
	return err != nil
}

// splitTopLevel splits text at commas outside any bracket or string, each group
// as the whitespace-separated fragments it is written in.
func splitTopLevel(text string) [][]string {
	var groups [][]string
	var frags []string
	var frag strings.Builder
	depth, quoted := 0, false

	flushFrag := func() {
		if frag.Len() > 0 {
			frags = append(frags, frag.String())
			frag.Reset()
		}
	}
	for _, r := range text {
		switch {
		case quoted:
			if r == '"' {
				quoted = false
			}
		case r == '"':
			quoted = true
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth == 0 && r == ',':
			flushFrag()
			groups = append(groups, frags)
			frags = nil
			continue
		case depth == 0 && (r == ' ' || r == '\t'):
			flushFrag()
			continue
		}
		frag.WriteRune(r)
	}
	flushFrag()
	return append(groups, frags)
}

// isNamedArgument reports whether text binds a name (`v0 = 1.0`), which is a
// production of an invocation rather than of an argument list at the prompt. The
// binding is the argument's own: an `=` nested in a call, in a bracket or in a
// string belongs to that expression.
func isNamedArgument(text string) bool {
	depth, quoted := 0, false
	for i, r := range text {
		switch {
		case quoted:
			if r == '"' {
				quoted = false
			}
		case r == '"':
			quoted = true
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth == 0 && r == '=' && i > 0 && i+1 < len(text):
			if text[i+1] == '=' || strings.ContainsRune("=<>!+-*/", rune(text[i-1])) {
				continue
			}
			return isIdentifier(strings.TrimSpace(text[:i]))
		}
	}
	return false
}

// isIdentifier reports whether text is one bare name, the only thing a named
// argument's left side can be.
func isIdentifier(text string) bool {
	if text == "" {
		return false
	}
	for i, r := range text {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

// parseWholeExpr parses text as one complete expression, reporting text the
// expression parser does not consume in full.
func parseWholeExpr(text string) (ast.Node, error) {
	p := parser.New(source.New("arg", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil {
		return nil, fmt.Errorf("failed to parse argument %q", text)
	}
	if _, bad := expr.(*ast.ErrorNode); bad || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return nil, fmt.Errorf("failed to parse argument %q", text)
	}
	return expr, nil
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
	passed, err := ctx.EvaluateConstraintOn(sym, declaringScope(sym, doc.Scope), inst)
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

// promptScope is the namespace a prompt expression is evaluated in: the last
// namespace the session declared, whose imports are then visible to it exactly
// as they are to a member written there (KerML 8.2.3.5.3). A session that
// declared no namespace evaluates at the document root.
func (s *Session) promptScope(doc *model.Document) *symbols.Scope {
	if doc == nil || doc.Scope == nil || doc.AST == nil {
		return nil
	}
	for i := len(doc.AST.Members) - 1; i >= 0; i-- {
		member := doc.AST.Members[i]
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		var ident ast.Identification
		switch n := member.(type) {
		case *ast.Package:
			ident = n.Ident
		case *ast.Namespace:
			ident = n.Ident
		default:
			continue
		}
		name := ident.Name
		if name == "" {
			name = ident.ShortName
		}
		if sym, ok := doc.Scope.LookupLocal(name); ok && sym != nil && sym.Scope != nil {
			return sym.Scope
		}
	}
	return doc.Scope
}

// declaringScope returns the scope an element's conditions were written in,
// which is what their names — a member of the enclosing package, a measurement
// unit an import brought in — resolve against. The document root reaches only
// what the root itself declares, so it is a fallback for a symbol carrying no
// declaring scope rather than the scope to evaluate in.
func declaringScope(sym *symbols.Symbol, root *symbols.Scope) *symbols.Scope {
	if sym != nil && sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return root
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
	passed, err := ctx.EvaluateRequirementOn(sym, declaringScope(sym, doc.Scope), inst)
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

// doSatisfy evaluates satisfaction assertions: every one the model states, or,
// given a name, the ones the named element states — or that element itself, when
// it is a named satisfaction assertion. The usual `assert satisfy r by p;` is
// anonymous, so the element stating it is how a user reaches it.
func (s *Session) doSatisfy(args []string) ([]string, bool, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}

	scope := doc.Scope
	where := "the session"
	if len(args) > 0 {
		sym, fqn, lerr := s.lookupSymbol(args[0])
		if lerr != nil {
			return []string{"error: " + lerr.Error()}, false, nil
		}
		if a, aerr := ctx.SatisfyAssertionOf(sym); aerr == nil {
			return s.satisfyVerdict(ctx, a), false, nil
		}
		if sym.Scope == nil {
			return []string{fmt.Sprintf("error: %s states no satisfaction assertion", args[0])}, false, nil
		}
		scope, where = sym.Scope, fqn
	}

	assertions := ctx.SatisfyAssertionsIn(scope)
	if len(assertions) == 0 {
		return []string{fmt.Sprintf("no satisfaction assertion in %s", where)}, false, nil
	}
	var out []string
	for _, a := range assertions {
		out = append(out, s.satisfyVerdict(ctx, a)...)
	}
	return out, false, nil
}

// satisfyVerdict renders the verdict of one satisfaction assertion, evaluated
// against an object of its subject: the one the session already created for that
// subject, so a `%instantiate` before it is what the verdict is about.
func (s *Session) satisfyVerdict(ctx *runtime.Context, a *runtime.SatisfyAssertion) []string {
	subject, owner := s.subjectInstance(a)
	if subject == nil && a.Subject != nil {
		// No object of the subject exists yet, so the verdict is about a fresh
		// one, created here rather than inside the evaluation so it can be named.
		if inst, serr := ctx.SatisfySubject(a); serr == nil {
			subject, owner = inst, s.subjectName(a)
			// Kept like %instantiate would, so a repeated %satisfy is about the
			// same object rather than another copy of it.
			s.instances[owner] = inst
		}
	}
	holds, err := ctx.EvaluateSatisfactionOn(a, subject)
	if err != nil || !holds {
		return []string{
			fmt.Sprintf("✗ %s fails%s", a.Text(), onInstance(subject, owner)),
			"  " + verdictDetail("Required condition", err),
		}
	}
	return []string{fmt.Sprintf("✓ %s holds%s", a.Text(), onInstance(subject, owner))}
}

// subjectInstance returns the object the session has already created for an
// assertion's subject, with the name it was created under, or nil for none.
func (s *Session) subjectInstance(a *runtime.SatisfyAssertion) (*runtime.Instance, string) {
	name := s.subjectName(a)
	if inst, ok := s.instances[name]; ok {
		return inst, name
	}
	return nil, ""
}

// subjectName is the name an assertion's subject is known by: its
// fully-qualified name, or the reference as written when it resolves to nothing.
func (s *Session) subjectName(a *runtime.SatisfyAssertion) string {
	if idx := s.symbolIndex(); idx != nil && a.Subject != nil {
		if fqn := idx.GetFQN(a.Subject); fqn != "" {
			return fqn
		}
	}
	return a.SubjectRef
}

// performingObject resolves the object a debugging session's behavior is
// performed by: its connections route what the behavior sends. No argument
// performs the behavior outside any object.
func (s *Session) performingObject(args []string) (*runtime.Instance, string) {
	if len(args) == 0 {
		return nil, ""
	}
	_, fqn, lerr := s.lookupSymbol(args[0])
	if lerr != nil {
		return nil, "error: " + lerr.Error()
	}
	inst, ok := s.instances[fqn]
	if !ok {
		return nil, fmt.Sprintf("error: no instance of %q (use %%instantiate first)", fqn)
	}
	return inst, ""
}

// --- Action Debugging Commands ---

// doAction starts an action executor debugging session.
func (s *Session) doAction(name string, performer []string) ([]string, bool, error) {
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

	self, msg := s.performingObject(performer)
	if msg != "" {
		return []string{msg}, false, nil
	}

	// Create executor
	exec, err := ctx.CreateActionExecutorFor(sym, self)
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
func (s *Session) doStateMachine(name string, performer []string) ([]string, bool, error) {
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

	self, msg := s.performingObject(performer)
	if msg != "" {
		return []string{msg}, false, nil
	}

	// Create executor
	exec, err := ctx.CreateStateExecutorFor(sym, self)
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
		// A signal in flight is due now, whatever the deadline: dispatching it is
		// the step RunToCompletion would take here.
		if exec.EventQueue().Len() == 0 && exec.HasPendingSignal() {
			if err := exec.ProcessNextEvent(); err != nil {
				return []string{fmt.Sprintf("error: event processing failed: %v", err)}, false, nil
			}
			processed++
			continue
		}
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
