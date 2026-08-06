package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ErrNoMatchingMessage reports that an accept action found no message it could
// consume. It is a suspension condition in SysML, but this runtime executes a
// single behavior to completion, so nothing can arrive later.
var ErrNoMatchingMessage = errors.New("no matching message")

// Message is a signal instance in flight.
//
// SignalType names the message's type: the type the send statement named, or
// the scalar type of the value it evaluated to. An accept whose parameter is
// typed consumes only messages of that type, so two sends of different types
// reach different accepts regardless of the order they were posted in.
//
// Target names the element the send addressed (`send m to r` / `send m via p`).
// A consumer accepts a message addressed to itself or to no one; an empty
// Target is a broadcast.
type Message struct {
	SignalType string
	Target     string
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
func (e *EvalContext) buildMessage(scope *symbols.Scope, send lower.Send) (Message, error) {
	if typeName, ok := e.namedType(scope, send.Message); ok {
		return Message{SignalType: typeName, Target: send.Target, Payload: map[string]Value{}}, nil
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
		Target:     send.Target,
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
