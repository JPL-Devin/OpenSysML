package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Message is a signal instance in flight.
//
// SignalType names the message's type: the type the send statement named, or
// the scalar type of the value it evaluated to. An accept whose parameter is
// typed consumes only messages of that type, so two sends of different types
// reach different accepts regardless of the order they were posted in.
//
// Target names the receiving node of the sending behavior a `send m to r`
// addressed; a consumer accepts a message addressed to itself or to no one.
//
// Port names the port the message reached — the peer end a `via p` send routed
// to, or the port an addressed send resolved to — and only an accept on that
// port consumes it, keeping port-routed and addressed traffic separate.
//
// Object identifies the object the message reached, 0 for none, and Delivery
// what of that destination a consumer must satisfy to take the message.
type Message struct {
	SignalType string
	Target     string
	Port       string
	Object     int64
	Delivery   DeliveryKind
	Payload    map[string]Value
}

// DeliveryKind is what a message's destination resolved to, and so what a
// consumer must match: an unaddressed message resolved nothing and any consumer
// may take it, while every addressed or routed one names a destination in full.
type DeliveryKind uint8

const (
	// DeliverAnyone is a message no send addressed, such as one injected from
	// outside the model: it has no destination to hold a consumer to.
	DeliverAnyone DeliveryKind = iota
	// DeliverPort is the port of an object, reached by a connection or addressed.
	DeliverPort
	// DeliverPortReceiver is a receiver of an object reached through a port.
	DeliverPortReceiver
	// DeliverReceiver is the receiving node named within an object.
	DeliverReceiver
	// DeliverObject is an object itself, whichever of its consumers accepts.
	DeliverObject
)

// Call is the payload of an EventCall: the operation invoked and its arguments.
type Call struct {
	Operation string
	Args      map[string]Value
}

// PostMessage puts a message on the context-wide bus, where every executor
// sharing this context can see it. Actions and state machines communicate
// through this bus rather than through per-executor queues, so a message a
// state machine's entry action sends can be accepted by one of its transitions.
//
// A message posted with a destination but no Delivery — one injected from
// outside the model — is held to the destination it names.
func (ctx *Context) PostMessage(msg Message) {
	if msg.Delivery == DeliverAnyone {
		msg.Delivery = deliveryOf(msg)
	}
	ctx.messages = append(ctx.messages, msg)
}

// deliveryOf is what the fields of a message name as its destination, most
// specific first: only a message naming nothing is open to any consumer.
func deliveryOf(msg Message) DeliveryKind {
	switch {
	case msg.Port != "" && msg.Target != "":
		return DeliverPortReceiver
	case msg.Port != "":
		return DeliverPort
	case msg.Target != "":
		return DeliverReceiver
	case msg.Object != 0:
		return DeliverObject
	}
	return DeliverAnyone
}

// TakeMessage removes and returns the oldest message satisfying match. Messages
// that do not match keep their place in the queue, so a consumer looking for
// one type does not consume or reorder another's.
func (ctx *Context) TakeMessage(match func(Message) bool) (Message, bool) {
	for i, msg := range ctx.messages {
		if match(msg) {
			ctx.messages = append(ctx.messages[:i], ctx.messages[i+1:]...)
			return msg, true
		}
	}
	return Message{}, false
}

// PendingMessages returns the messages still in flight, oldest first.
func (ctx *Context) PendingMessages() []Message {
	out := make([]Message, len(ctx.messages))
	copy(out, ctx.messages)
	return out
}

// reaches reports whether a consumer named name, accepting on port and performed
// by object, may take this message: every part of the destination must hold, the
// ports always included, so port-routed and addressed traffic stay apart. A
// behavior no object performs has no identity to compare, so a message naming
// the receiver it is neither excludes it nor is excluded by it.
func (m Message) reaches(name, port string, object int64) bool {
	if m.Port != port {
		return false
	}
	switch m.Delivery {
	case DeliverPort, DeliverObject:
		return m.Object == object
	case DeliverPortReceiver:
		return m.Target == name && m.Object == object
	case DeliverReceiver:
		return m.Target == name && (m.Object == object || m.Object == 0 || object == 0)
	}
	return true
}

// objectID is the identity of an object, 0 for none.
func objectID(inst *Instance) int64 {
	if inst == nil {
		return 0
	}
	return inst.ID
}

// postVia routes a message out of a sending port: every port joined to it that
// can receive the message gets a copy, which is the ends whose flow features
// carry inward after conjugation. A send that reaches none of them is delivered
// nowhere, which is a typed error rather than a message quietly dropped — the
// model asked for a delivery the connections it declares cannot make.
func (ctx *Context) postVia(conns []lower.Connection, msg Message, send lower.Send, self *Instance) error {
	if send.Receiver != "" && send.Scope != nil {
		sym, ok := ctx.portSymbol(send.Scope, send.Target)
		if !ok || sym == nil || sym.Kind != symbols.SymbolPortUsage ||
			!ctx.ownPortPath(send.Scope, strings.Split(send.Target, ".")) {
			return &UnknownSendPortError{Port: send.Target, Receiver: send.Receiver}
		}
	}
	receiver := send.Receiver
	if receiver != "" {
		receiverSend := send
		receiverSend.Target = send.Receiver
		receiverSend.TargetPath = send.ReceiverPath
		receiverSend.IsVia = false
		addr, err := ctx.resolveRoutedReceiver(receiverSend, self)
		if err != nil || (addr.Object != 0 && addr.Object != objectID(self)) {
			return &UnreachableSendReceiverError{Port: send.Target, Receiver: send.Receiver}
		}
		receiver = addr.Name
	}
	routable := ctx.realizedConnections(ctx.routableConnections(conns, self, send.Scope), self)
	var receiving, outbound []string
	var typeMismatch bool
	if receiver != "" {
		receiving, outbound, typeMismatch = ctx.receivingEndsForMessage(
			routable, send.Target, msg.SignalType,
		)
	} else {
		receiving, outbound = ctx.receivingEnds(routable, send.Target)
	}
	if len(receiving) == 0 {
		if typeMismatch && receiver != "" {
			return &SendPortTypeMismatchError{
				Port: send.Target, Receiver: receiver, SignalType: msg.SignalType,
			}
		}
		return &UnroutableSendError{Port: send.Target, Outbound: outbound}
	}
	for _, peer := range receiving {
		routed := msg
		routed.Target = receiver
		routed.Port = peer
		routed.Object = objectID(self)
		routed.Delivery = DeliverPort
		if receiver != "" {
			routed.Delivery = DeliverPortReceiver
		}
		ctx.PostMessage(routed)
	}
	return nil
}

func (ctx *Context) resolveRoutedReceiver(send lower.Send, self *Instance) (messageAddress, error) {
	separator := "::"
	if send.TargetPath {
		separator = "."
	}
	segments := strings.Split(send.Target, separator)
	sym, ok := ctx.pathSymbol(send.Scope, segments)
	if !ok || !isRoutedReceiverSymbol(sym) {
		return messageAddress{}, fmt.Errorf("receiver %q is unresolved", send.Target)
	}
	addr, err := ctx.resolveAddress(send, self)
	if err != nil || addr.Delivery != DeliverReceiver {
		if err == nil {
			err = fmt.Errorf("receiver %q is not a receiving node", send.Target)
		}
		return messageAddress{}, err
	}
	return addr, nil
}

func isRoutedReceiverSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolActionDef, symbols.SymbolActionUsage,
		symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	default:
		return false
	}
}

// messageAddress is where an addressed send delivers: what the address resolved
// to, and the object, port or receiving node naming it. Only the constructors
// below build one, so no address can name a destination in part.
type messageAddress struct {
	Delivery DeliveryKind
	Name     string
	Port     string
	Object   int64
}

// portAddress is a port of an object, refused where no port was resolved.
func portAddress(port string, object int64) (messageAddress, bool) {
	if port == "" {
		return messageAddress{}, false
	}
	return messageAddress{Delivery: DeliverPort, Port: port, Object: object}, true
}

// receiverAddress is a receiving node of an object, refused where it is unnamed.
func receiverAddress(name string, object int64) (messageAddress, bool) {
	if name == "" {
		return messageAddress{}, false
	}
	return messageAddress{Delivery: DeliverReceiver, Name: name, Object: object}, true
}

// objectAddress is an object itself, refused where no object was reached: a
// destination confined to object 0 would confine the message to nothing.
func objectAddress(object int64) (messageAddress, bool) {
	if object == 0 {
		return messageAddress{}, false
	}
	return messageAddress{Delivery: DeliverObject, Object: object}, true
}

// postTo delivers an addressed send to the object its target resolves to,
// leaving the message for that object alone.
func (ctx *Context) postTo(msg Message, send lower.Send, self *Instance) error {
	addr, err := ctx.resolveAddress(send, self)
	if err != nil {
		return err
	}
	msg.Target, msg.Port, msg.Object = addr.Name, addr.Port, addr.Object
	msg.Delivery = addr.Delivery
	ctx.PostMessage(msg)
	return nil
}

// resolveAddress answers what a `send m to t` addressed: the object t belongs to
// and the port path within it, resolved through the instance graph. A chain the
// graph does not reach is a port of the sender itself, and unroutable if it is
// neither; a name reaching neither is the receiving node of that name.
func (ctx *Context) resolveAddress(send lower.Send, self *Instance) (messageAddress, error) {
	if send.Target == "" {
		// A send addressing no one is for the sending object, or for whoever accepts
		// it where no object sent it.
		if addr, ok := objectAddress(objectID(self)); ok {
			return addr, nil
		}
		return messageAddress{Delivery: DeliverAnyone}, nil
	}
	if !send.TargetPath {
		return ctx.namedAddress(send, self)
	}
	segments := strings.Split(send.Target, ".")
	addr, ok, err := ctx.featureAddress(send.Scope, self, segments)
	if err != nil {
		return messageAddress{}, err
	}
	if ok {
		return addr, nil
	}
	if sym, ok := ctx.portSymbol(send.Scope, send.Target); ok &&
		sym.Kind == symbols.SymbolPortUsage && ctx.ownPortPath(send.Scope, segments) {
		if addr, ok := portAddress(send.Target, objectID(self)); ok {
			return addr, nil
		}
	}
	return messageAddress{}, &UnroutableSendError{Port: send.Target, Address: true}
}

// namedAddress resolves a target named rather than chained (`R`, `P::R`): an
// unqualified name is a feature, port or receiving node of the sending object,
// and a qualified one is the element its path names, never a same-named element
// of the sender.
func (ctx *Context) namedAddress(send lower.Send, self *Instance) (messageAddress, error) {
	segments := strings.Split(send.Target, "::")
	name := segments[len(segments)-1]
	if len(segments) > 1 {
		return ctx.qualifiedAddress(send, self, segments)
	}
	addr, ok, err := ctx.featureAddress(send.Scope, self, segments)
	if err != nil {
		return messageAddress{}, err
	}
	if ok {
		return addr, nil
	}
	if sym, resolved := ctx.pathSymbol(send.Scope, segments); resolved &&
		sym.Kind == symbols.SymbolPortUsage {
		if addr, built := portAddress(name, objectID(self)); built {
			return addr, nil
		}
		return messageAddress{}, &UnroutableSendError{Port: send.Target, Address: true}
	}
	if addr, built := receiverAddress(name, objectID(self)); built {
		return addr, nil
	}
	return messageAddress{}, &UnroutableSendError{Port: send.Target, Address: true}
}

// qualifiedAddress resolves a target naming a namespace path (`alpha::reader`,
// `P::Driver`) to the element that path names, never to a same-named feature of
// the sender: the qualifier chooses the object, so the address is the occurrence
// the path leads through, or unroutable where this run reaches none.
func (ctx *Context) qualifiedAddress(send lower.Send, self *Instance, segments []string) (messageAddress, error) {
	addr, ok, err := ctx.featureAddress(send.Scope, nil, segments)
	if err != nil {
		return messageAddress{}, err
	}
	if ok {
		return addr, nil
	}
	target, resolved := ctx.pathSymbol(send.Scope, segments)
	if !resolved {
		return messageAddress{}, &UnroutableSendError{Port: send.Target, Address: true}
	}
	name := segments[len(segments)-1]
	// A path leading through no occurrence names an element of the sending
	// behavior's own namespace, where its bare name is that same element:
	// `P::Node::Machine` is the sender's machine, `P::alpha::inPort` is alpha's.
	if local, ok := ctx.pathSymbol(send.Scope, []string{name}); ok && local == target {
		addr, ok, err := ctx.featureAddress(send.Scope, self, []string{name})
		if err != nil {
			return messageAddress{}, err
		}
		if ok {
			return addr, nil
		}
		if target.Kind == symbols.SymbolPortUsage {
			if addr, built := portAddress(name, objectID(self)); built {
				return addr, nil
			}
		} else if addr, built := receiverAddress(name, objectID(self)); built {
			return addr, nil
		}
	}
	// A receiver no object owns has no identity of its own; a sender that has one
	// cannot address it by name without reaching its own same-named element.
	if self == nil && target.Kind != symbols.SymbolPortUsage {
		if addr, built := receiverAddress(name, 0); built {
			return addr, nil
		}
	}
	return messageAddress{}, &UnroutableSendError{Port: send.Target, Address: true}
}

// featureAddress walks a target through the instance graph from the object its
// first segment belongs to, reporting no address where a segment names no
// feature, or one that is neither a port nor an occurrence to descend into. A
// failure to read an object of the graph is that failure, not a bad address.
func (ctx *Context) featureAddress(scope *symbols.Scope, self *Instance, segments []string) (messageAddress, bool, error) {
	owner, rest, ok, err := ctx.addressOwner(scope, self, segments)
	if err != nil || !ok {
		return messageAddress{}, false, err
	}
	for i, segment := range rest {
		fv, held := owner.FeatureValues[segment]
		if !held {
			return messageAddress{}, false, nil
		}
		if isPortFeature(fv.Feature) {
			addr, built := portAddress(strings.Join(rest[i:], "."), owner.ID)
			return addr, built, nil
		}
		// A behavior of an object is a receiving node of it, addressed by name.
		if i == len(rest)-1 && isBehaviorFeature(fv.Feature) {
			addr, built := receiverAddress(segment, owner.ID)
			return addr, built, nil
		}
		owner, ok, err = ctx.fvObject(owner, segment)
		if err != nil || !ok {
			return messageAddress{}, false, err
		}
	}
	addr, built := objectAddress(owner.ID)
	return addr, built, nil
}

// addressOwner answers which object a target's leading segments belong to: the
// sending object, or one holding it, where the first names a feature of it, else the occurrence
// the shortest prefix names in the send's scope — a prefix rather than one name,
// since a namespace qualifies the occurrence in `P::alpha.inPort`.
func (ctx *Context) addressOwner(scope *symbols.Scope, self *Instance, segments []string) (*Instance, []string, bool, error) {
	// A name is a feature of the sending object, or of an object holding it: a
	// nested object addresses a sibling through the object they belong to.
	for up := self; up != nil; up = up.owner {
		if fv, held := up.FeatureValues[segments[0]]; held && ctx.namesFeature(scope, up, fv, segments[0]) {
			return up, segments, true, nil
		}
	}
	if scope == nil || ctx.resolver == nil {
		return nil, nil, false, nil
	}
	for n := 1; n <= len(segments); n++ {
		sym, ok := ctx.pathSymbol(scope, segments[:n])
		if !ok || !isOccurrenceUsage(sym) || !ctx.occursOnce(sym) {
			continue
		}
		if self != nil && self.Type == sym {
			return self, segments[n:], true, nil
		}
		// A target that names an object this run cannot build fails as that, rather
		// than being reported as an address naming nothing.
		inst, err := ctx.occurrenceOf(sym)
		if err != nil {
			return nil, nil, false, err
		}
		return inst, segments[n:], true, nil
	}
	return nil, nil, false, nil
}

// namesFeature reports whether a feature value of the sending object is what a name in
// the send's scope denotes: a nearer declaration, such as a node of the sending
// behavior, shadows the object's feature as name resolution has it.
func (ctx *Context) namesFeature(scope *symbols.Scope, self *Instance, fv *FeatureValue, name string) bool {
	sym, ok := ctx.pathSymbol(scope, []string{name})
	if !ok || (fv.Feature != nil && fv.Feature.Symbol == sym) {
		return true
	}
	for _, feat := range ctx.FeaturesOf(self.Type) {
		if feat.Symbol == sym {
			return true
		}
	}
	return false
}

// ownPortPath reports whether a port path names a port of the sending behavior
// itself. A path led by a namespace or by an occurrence names a port of another
// object, which is unroutable where the instance graph did not reach it.
func (ctx *Context) ownPortPath(scope *symbols.Scope, segments []string) bool {
	sym, ok := ctx.pathSymbol(scope, segments[:1])
	if !ok || isOccurrenceUsage(sym) {
		return false
	}
	return sym.Kind != symbols.SymbolPackage && sym.Kind != symbols.SymbolNamespace
}

// featureValueObject reads the object a feature of inst holds, materializing it, and
// reports whether the feature holds one at all. A feature value that cannot be read is
// that failure rather than a feature holding no object.
func (ctx *Context) fvObject(inst *Instance, name string) (*Instance, bool, error) {
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if fv == nil || fv.Value.Kind != ValInstance {
		return nil, false, nil
	}
	held, ok := ctx.instances[fv.Value.Instance]
	return held, ok, nil
}

// isPortFeature reports whether a feature is a port, where an address stops: the
// rest of its path is the port path within the object.
func isPortFeature(feature *EffectiveFeature) bool {
	return feature != nil && feature.Symbol != nil && feature.Symbol.Kind == symbols.SymbolPortUsage
}

// isBehaviorFeature reports whether a feature is a behavior an object performs,
// which receives by name rather than being an object of its own.
func isBehaviorFeature(feature *EffectiveFeature) bool {
	if feature == nil || feature.Symbol == nil {
		return false
	}
	switch feature.Symbol.Kind {
	case symbols.SymbolActionUsage, symbols.SymbolStateUsage:
		return true
	}
	return false
}

// post delivers a built message the way the send addressed it: routed through
// the connections of the sending port, or straight onto the bus. self is the
// object performing the behavior that sent it, nil for a behavior no object
// performs.
func (ctx *Context) post(conns []lower.Connection, msg Message, send lower.Send, self *Instance) error {
	if send.IsVia {
		return ctx.postVia(conns, msg, send, self)
	}
	return ctx.postTo(msg, send, self)
}

// carriesSignal reports whether this message satisfies an accept of signalType.
// An empty signalType accepts any message: the model asked for no type.
func (m Message) carriesSignal(signalType string) bool {
	return signalType == "" || signalType == m.SignalType
}

// buildMessage evaluates a send statement into a message.
//
// A send whose message names a type sends that type with no payload
// (`send Ping to m`, where `item def Ping;`) — the notation this subset offers
// for a signal that carries nothing. Any other message expression is evaluated,
// and the message is typed by the value's type, carrying it as `value`.
//
// A via send keeps a receiver target when one was stated; postVia fills in the
// reached port and final delivery kind.
func (e *EvalContext) buildMessage(scope *symbols.Scope, send lower.Send) (Message, error) {
	target := send.Target
	if send.IsVia {
		target = send.Receiver
	}
	if typeName, ok := e.namedType(scope, send.Message); ok {
		return Message{SignalType: typeName, Target: target, Payload: map[string]Value{}}, nil
	}

	// `send Data(data) via p`, `send shutDown() to self`: the invoked name is
	// what the message is, whether it names a signal definition or an event
	// feature, and its arguments are the payload it carries. An accept matches
	// on that name, so building the message never evaluates the invocation as a
	// call — there is no function of that name to call.
	// A calculation of that name is called, though: what is sent is the value it
	// returns, not a message named after it.
	if invocation, ok := send.Message.(*ast.InvocationExpr); ok && !e.invokesCalc(scope, invocation) {
		return e.buildInvokedMessage(invocation, target)
	}

	value, err := e.Eval(send.Message)
	if err != nil {
		return Message{}, fmt.Errorf("eval send message: %w", err)
	}
	signalType := valueTypeName(value)
	if signalType == "" {
		return Message{}, fmt.Errorf("send: message of kind %v has no signal type", value.Kind)
	}
	return Message{
		SignalType: signalType,
		Target:     target,
		Payload:    map[string]Value{"value": value},
	}, nil
}

// invokesCalc reports whether an invocation calls a calculation — a calc
// declaration or a library function — rather than naming a signal to send.
func (e *EvalContext) invokesCalc(scope *symbols.Scope, invocation *ast.InvocationExpr) bool {
	if invocation.Type == nil {
		return false
	}
	if _, isBuiltin := builtins[qualifiedNameToString(invocation.Type)]; isBuiltin {
		return true
	}
	if e.ctx == nil || e.ctx.resolver == nil || scope == nil {
		return false
	}
	sym, ok := e.ctx.resolver.ResolveQualified(scope, invocation.Type)
	if !ok || sym == nil {
		return false
	}
	return isCalcDecl(sym.Decl)
}

// buildInvokedMessage builds the message of a send written as an invocation.
// The invoked name types the message; each argument is a value it carries,
// named where the send named it and by position where it did not. A single
// positional argument is also carried as `value`, which is what an accept binds
// its payload parameter to.
func (e *EvalContext) buildInvokedMessage(invocation *ast.InvocationExpr, target string) (Message, error) {
	signalType := ast.SimpleName(invocation.Type)
	if signalType == "" {
		return Message{}, fmt.Errorf("send: the message names no signal")
	}
	if invocation.Operand != nil {
		return Message{}, fmt.Errorf("send %s: a message is not sent through a receiver", signalType)
	}

	payload := make(map[string]Value, len(invocation.Args)+len(invocation.NamedArgs))
	for i, arg := range invocation.Args {
		value, err := e.Eval(arg)
		if err != nil {
			return Message{}, fmt.Errorf("eval argument %d of send %s: %w", i+1, signalType, err)
		}
		payload[fmt.Sprintf("arg%d", i+1)] = value
		if len(invocation.Args) == 1 && len(invocation.NamedArgs) == 0 {
			payload["value"] = value
		}
	}
	for _, arg := range invocation.NamedArgs {
		name := ast.SimpleName(arg.Name)
		if name == "" {
			return Message{}, fmt.Errorf("send %s: an argument is named by nothing", signalType)
		}
		value, err := e.Eval(arg.Value)
		if err != nil {
			return Message{}, fmt.Errorf("eval argument %s of send %s: %w", name, signalType, err)
		}
		payload[name] = value
	}

	return Message{SignalType: signalType, Target: target, Payload: payload}, nil
}

// triggerName describes a transition's trigger for traces. Traces are compared
// against goldens, so the text has to be stable: printing the trigger node
// itself emits a pointer address.
func triggerName(trigger ast.Node) string {
	switch t := trigger.(type) {
	case nil:
		return ""
	case *ast.AcceptEvent:
		return "accept " + orAny(ast.SimpleName(t.SignalType))
	case *ast.CallEvent:
		return "call " + orAny(ast.SimpleName(t.Operation))
	case *ast.TimeEvent:
		return "time"
	case *ast.ChangeEvent:
		return "change"
	default:
		return fmt.Sprintf("%T", trigger)
	}
}

// triggerDescription describes the event an accept waits for in the notation it
// was written in, which is what an error about it, or a view of a suspended run,
// has to name. The expression a trigger waits on is named when it is a name;
// there is no printer for an arbitrary one, so the keyword alone stands for it.
func triggerDescription(trigger ast.Node) string {
	switch t := trigger.(type) {
	case *ast.TimeEvent:
		keyword := "after"
		if t.Absolute {
			keyword = "at"
		}
		return joinWords("accept", keyword, ast.SimpleName(t.Duration))
	case *ast.ChangeEvent:
		return joinWords("accept", "when", ast.SimpleName(t.Condition))
	default:
		return triggerName(trigger)
	}
}

// joinWords joins the words of a description, dropping the ones that are empty.
func joinWords(words ...string) string {
	kept := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

// viaSuffix describes the port an accept waits on, for an error message, or
// nothing when it waits on none.
func viaSuffix(port string) string {
	if port == "" {
		return ""
	}
	return " via " + port
}

// orAny names a type or operation, or reports that any is accepted when the
// model named none.
func orAny(name string) string {
	if name == "" {
		return "any"
	}
	return name
}

// namedType reports the type name when expr names a type definition rather than
// denoting a value.
func (e *EvalContext) namedType(scope *symbols.Scope, expr ast.Node) (string, bool) {
	qname := ast.AsQualifiedName(expr)
	if qname == nil || scope == nil || e.ctx == nil || e.ctx.resolver == nil {
		return "", false
	}
	sym, ok := e.ctx.resolver.ResolveQualified(scope, qname)
	if !ok || sym == nil {
		return "", false
	}
	if _, isDef := sym.Decl.(*ast.Definition); !isDef {
		return "", false
	}
	return sym.Name, true
}

// valueTypeName names the type of a value, as a send statement's signal type.
func valueTypeName(v Value) string {
	switch v.Kind {
	case ValString:
		return "String"
	case ValConst:
		switch v.Const.Kind {
		case semantics.ValInt:
			return "Integer"
		case semantics.ValReal:
			return "Real"
		case semantics.ValBool:
			return "Boolean"
		}
	}
	return ""
}
