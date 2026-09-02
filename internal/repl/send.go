package repl

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const sendUsage = "usage: %send <signal>[(<parameter>=<expression>, ...)] [to <object>]"

// signalKinds are the definitions a signal is declared as: what an accept may
// be typed by and a send may carry. A behavior definition is neither.
var signalKinds = []symbols.SymbolKind{
	symbols.SymbolAttributeDef,
	symbols.SymbolItemDef,
	symbols.SymbolOccurrenceDef,
	symbols.SymbolPartDef,
	symbols.SymbolIndividualDef,
	symbols.SymbolEnumerationDef,
	symbols.SymbolMetadataDef,
}

// sendRequest is a %send line taken apart: the signal, its arguments as
// written, and the object named after `to`, empty when none was.
type sendRequest struct {
	signal string
	args   []string
	target string
}

// parseSendLine takes apart what follows %send.
func parseSendLine(text string) (sendRequest, error) {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "(") || text == "to" || strings.HasPrefix(text, "to ") {
		return sendRequest{}, errors.New(sendUsage)
	}
	var req sendRequest
	end := indexOutsideName(text, "( \t")
	if end < 0 {
		end = len(text)
	}
	req.signal, text = text[:end], strings.TrimSpace(text[end:])

	if strings.HasPrefix(text, "(") {
		closing := closingParen(text)
		if closing < 0 {
			return sendRequest{}, fmt.Errorf("the argument list of %s is not closed: %s", req.signal, sendUsage)
		}
		for _, group := range splitTopLevel(text[1:closing]) {
			if arg := strings.Join(group, " "); arg != "" {
				req.args = append(req.args, arg)
			}
		}
		text = strings.TrimSpace(text[closing+1:])
	}
	if text == "" {
		return req, nil
	}
	target, ok := strings.CutPrefix(text, "to")
	if !ok || (target != "" && !strings.HasPrefix(target, " ") && !strings.HasPrefix(target, "\t")) {
		return sendRequest{}, fmt.Errorf("unexpected %q after the signal: %s", text, sendUsage)
	}
	req.target = strings.TrimSpace(target)
	if req.target == "" || indexOutsideName(req.target, " \t") >= 0 {
		return sendRequest{}, fmt.Errorf("`to` names one object: %s", sendUsage)
	}
	return req, nil
}

// quoteTracker follows the lexer's quoting through a prompt argument: a string
// or quoted name is opaque to a scanner, its escaped characters included.
type quoteTracker struct {
	quote   rune
	escaped bool
}

// inside consumes r, reporting whether it is part of a string or quoted name.
func (q *quoteTracker) inside(r rune) bool {
	switch {
	case q.escaped:
		q.escaped = false
	case q.quote != 0:
		if r == '\\' {
			q.escaped = true
		} else if r == q.quote {
			q.quote = 0
		}
	case r == '"' || r == '\'':
		q.quote = r
	default:
		return false
	}
	return true
}

// closingParen indexes the parenthesis closing the one text opens with, -1 when
// none does; parentheses inside a string or quoted name do not count.
func closingParen(text string) int {
	depth, q := 0, quoteTracker{}
	for i, r := range text {
		switch {
		case q.inside(r):
		case r == '(':
			depth++
		case r == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// signalTarget is where a %send delivers: the object (nil for a machine no
// object performs), the machines that may accept it, and its name in the report.
type signalTarget struct {
	object   *runtime.Instance
	machines []*runtime.StateExecutor
	label    string
}

// doSend injects a signal into an object's machine through the runtime's own
// message bus, so the debugger delivers it as it would one an action sent.
func (s *Session) doSend(text string) ([]string, bool, error) {
	lines, err := s.sendSignal(text)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// sendSignal resolves the signal and its destination, refuses what no machine
// there would accept, and posts the rest.
func (s *Session) sendSignal(text string) ([]string, error) {
	req, err := parseSendLine(text)
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	target, err := s.signalTarget(req.target)
	if err != nil {
		return nil, err
	}
	msg, typed, err := s.signalMessage(ctx, req, target)
	if err != nil {
		return nil, err
	}
	accepting := acceptingMachines(target.machines, msg)
	if len(accepting) == 0 {
		return nil, fmt.Errorf("%s accepts no signal %s now: %s", target.label, msg.SignalType, machineStates(target.machines))
	}
	ctx.PostMessage(msg)

	out := []string{fmt.Sprintf("✓ Sent %s to %s", signalText(msg), target.label)}
	if !typed {
		out = append(out, fmt.Sprintf("  No declaration types %s, so the signal is matched by name alone", msg.SignalType))
	}
	out = append(out, "  Accepted by "+machineStates(accepting))
	if s.stateExec != nil && s.stateExec.executor.AcceptsMessage(msg) {
		out = append(out, "", "Use %step or %advance <time> to dispatch it")
	}
	return out, nil
}

// signalTarget resolves the object a %send names, or the object the debugged
// machine is performed by when it names none.
func (s *Session) signalTarget(name string) (signalTarget, error) {
	if name == "" {
		if s.stateExec == nil {
			return signalTarget{}, fmt.Errorf("%w, so there is no object to send to: name one with `to <object>`", s.noStateSessionErr())
		}
		exec := s.stateExec.executor
		label := fmt.Sprintf("state machine %q", s.stateExec.name)
		if self := exec.Performer(); self != nil {
			label = fmt.Sprintf("object #%d of %q", self.ID, notationName(s.stateExec.selfFQN))
		}
		return signalTarget{object: exec.Performer(), machines: []*runtime.StateExecutor{exec}, label: label}, nil
	}

	inst, fqn := s.objectNamed(name)
	if inst == nil {
		_, resolved, lerr := s.lookupSymbol(name)
		if lerr != nil {
			return signalTarget{}, lerr
		}
		if inst, fqn = s.objectNamed(resolved); inst == nil {
			return signalTarget{}, fmt.Errorf("no instance of %q (use %%instantiate first)", notationName(resolved))
		}
	}
	label := fmt.Sprintf("object #%d of %q", inst.ID, notationName(fqn))
	machines := s.machinesOf(inst)
	if len(machines) == 0 {
		return signalTarget{}, fmt.Errorf("%s runs no state machine, so nothing there accepts a signal (%%state <machine> <object> starts one)", label)
	}
	return signalTarget{object: inst, machines: machines, label: label}, nil
}

// machinesOf lists the machines an object runs: those it exhibits, and the one
// a %state session performs on its behalf.
func (s *Session) machinesOf(inst *runtime.Instance) []*runtime.StateExecutor {
	var machines []*runtime.StateExecutor
	for _, b := range inst.Behaviors() {
		if b.State != nil {
			machines = append(machines, b.State)
		}
	}
	if s.stateExec != nil {
		exec := s.stateExec.executor
		if exec.Performer() == inst && !slices.Contains(machines, exec) {
			machines = append(machines, exec)
		}
	}
	return machines
}

// signalMessage builds the message to post, typed by the definition the name
// resolves to; a bare name no declaration types is matched by name, as `accept go` is.
func (s *Session) signalMessage(ctx *runtime.Context, req sendRequest, target signalTarget) (runtime.Message, bool, error) {
	sym, _, lerr := s.lookupSymbolOfKinds(req.signal, signalKinds...)
	if lerr != nil {
		if !errors.Is(lerr, runtime.ErrUnresolvedReference) {
			return runtime.Message{}, false, lerr
		}
		named := runtime.NamedSignalMessage(nameText(req.signal), target.object)
		if len(req.args) > 0 || len(acceptingMachines(target.machines, named)) == 0 {
			return runtime.Message{}, false, lerr
		}
		return named, false, nil
	}
	if !slices.Contains(signalKinds, sym.Kind) {
		return runtime.Message{}, false, fmt.Errorf("%s is a %s, not a signal definition", req.signal, sym.Notation())
	}
	if err := checkArgumentNames(req.args); err != nil {
		return runtime.Message{}, false, err
	}
	bound, err := s.operationArguments(ctx, req.args)
	if err != nil {
		return runtime.Message{}, false, err
	}
	msg, err := ctx.SignalMessage(sym, bound, target.object)
	if err != nil {
		return runtime.Message{}, false, err
	}
	return msg, true, nil
}

// checkArgumentNames refuses an argument list that binds one parameter twice,
// which a map of bindings would otherwise silently reduce to the last.
func checkArgumentNames(args []string) error {
	seen := make(map[string]bool, len(args))
	for _, arg := range args {
		param, _, ok := strings.Cut(arg, "=")
		param = strings.TrimSpace(param)
		if !ok || param == "" {
			return fmt.Errorf("argument %q is not written as <parameter>=<expression>", arg)
		}
		if seen[param] {
			return fmt.Errorf("argument %s is given twice", param)
		}
		seen[param] = true
	}
	return nil
}

// acceptingMachines keeps the machines whose active configuration accepts msg.
func acceptingMachines(machines []*runtime.StateExecutor, msg runtime.Message) []*runtime.StateExecutor {
	var out []*runtime.StateExecutor
	for _, m := range machines {
		if m.AcceptsMessage(msg) {
			out = append(out, m)
		}
	}
	return out
}

// machineStates names each machine with the state it is in.
func machineStates(machines []*runtime.StateExecutor) string {
	parts := make([]string, 0, len(machines))
	for _, m := range machines {
		parts = append(parts, fmt.Sprintf("state machine %q in state %s", machineName(m), currentStateName(m)))
	}
	return strings.Join(parts, ", ")
}

// machineName names a machine the way the notation declares it.
func machineName(exec *runtime.StateExecutor) string {
	if sym := exec.StateMachineSymbol(); sym != nil && sym.Name != "" {
		return sym.Name
	}
	return "<anonymous>"
}

// signalText writes a message as the send that posts it: the signal with its
// payload, parameter by parameter.
func signalText(msg runtime.Message) string {
	if len(msg.Payload) == 0 {
		return msg.SignalType
	}
	names := make([]string, 0, len(msg.Payload))
	for name := range msg.Payload {
		names = append(names, name)
	}
	sort.Strings(names)
	args := make([]string, 0, len(names))
	for _, name := range names {
		args = append(args, name+"="+runtime.FormatValue(msg.Payload[name]))
	}
	return msg.SignalType + "(" + strings.Join(args, ", ") + ")"
}
