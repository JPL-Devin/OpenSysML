package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Message is a signal instance in flight.
//
// SignalType names the message's type: the type the send statement named, or
// the scalar type of the value it evaluated to. An accept whose parameter is
// typed consumes only messages of that type, so two sends of different types
// reach different accepts regardless of the order they were posted in.
//
// Target names the receiver a `send m to r` addressed. A consumer accepts a
// message addressed to itself or to no one; an empty Target is a broadcast.
//
// Port names the port a `send m via p` was routed to — the peer end of p, not p
// itself, since that is where the message arrived. Only an accept on that port
// consumes it, so port-routed traffic and addressed traffic stay separate.
type Message struct {
	SignalType string
	Target     string
	Port       string
	Payload    map[string]Value
}

// Call is the payload of an EventCall: the operation invoked and its arguments.
type Call struct {
	Operation string
	Args      map[string]Value
}

// PostMessage puts a message on the context-wide bus, where every executor
// sharing this context can see it. Actions and state machines communicate
// through this bus rather than through per-executor queues, so a message a
// state machine's entry action sends can be accepted by one of its transitions.
func (ctx *Context) PostMessage(msg Message) {
	ctx.messages = append(ctx.messages, msg)
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

// addressedTo reports whether a consumer named name may take this message.
func (m Message) addressedTo(name string) bool {
	return m.Target == "" || m.Target == name
}

// arrivedAt reports whether a consumer listening on port may take this message.
// The two sides must agree: a message routed through a port is only for an
// accept on that port, and an accept on a port takes nothing else — otherwise a
// broadcast would be consumed by whichever accept ran first, regardless of the
// connections the model declared.
func (m Message) arrivedAt(port string) bool {
	return m.Port == port
}

// postVia routes a message out of a sending port: every port connected to it
// receives a copy, since a connection joins ends without a direction. A port
// with no connections reaches no one, which is not an error — the message is
// simply never delivered, and an accept waiting for it stays suspended until
// the run gives up with ErrAcceptDeadlock.
func (ctx *Context) postVia(conns []lower.Connection, msg Message, sendingPort string) {
	for _, peer := range lower.PeerPorts(conns, sendingPort) {
		routed := msg
		routed.Target = ""
		routed.Port = peer
		ctx.PostMessage(routed)
	}
}

// post delivers a built message the way the send addressed it: routed through
// the connections of the sending port, or straight onto the bus.
func (ctx *Context) post(conns []lower.Connection, msg Message, send lower.Send) {
	if send.IsVia {
		ctx.postVia(conns, msg, send.Target)
		return
	}
	ctx.PostMessage(msg)
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
// A `via` send addresses a port rather than a receiver, so the built message
// carries no Target: postVia fills in the port the message reaches.
func (e *EvalContext) buildMessage(scope *symbols.Scope, send lower.Send) (Message, error) {
	target := send.Target
	if send.IsVia {
		target = ""
	}
	if typeName, ok := e.namedType(scope, send.Message); ok {
		return Message{SignalType: typeName, Target: target, Payload: map[string]Value{}}, nil
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
