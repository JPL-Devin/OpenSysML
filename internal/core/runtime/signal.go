package runtime

import (
	"fmt"
	"strconv"
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
//
// PortID identifies the port object the message reached, 0 where the run holds
// none. A binding connector makes a boundary port and an inner port one object,
// so an accept on either port takes a message that reached the other.
type Message struct {
	SignalType string
	// Signal is the definition SignalType resolved to when the send was built,
	// nil where the message's type is known only as a name. An accept matches it
	// by conformance, so a subtype message satisfies a supertype accept and
	// same-named definitions of different packages stay apart.
	Signal   *symbols.Symbol
	Target   string
	Port     string
	Object   int64
	PortID   int64
	Delivery DeliveryKind
	Payload  map[string]Value
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

// messageReaches reports whether a consumer named name, accepting on port and
// performed by self, may take message m: by the destination as written, or by
// the identity of the port object it reached — a port a binding connector
// joins to another is that other port, whichever object's feature names it.
// Resolving the consumer's port may materialize it, which can fail.
func (ctx *Context) messageReaches(m Message, name, port string, self *Instance) (bool, error) {
	if m.reaches(name, port, objectID(self)) {
		return true, nil
	}
	if m.PortID == 0 || port == "" {
		return false, nil
	}
	portID, err := ctx.portInstanceID(self, port)
	if err != nil {
		return false, err
	}
	if m.PortID != portID {
		return false, nil
	}
	switch m.Delivery {
	case DeliverPort:
		return true, nil
	case DeliverPortReceiver:
		return m.Target == name, nil
	}
	return false, nil
}

// portInstanceID is the identity of the port object a dotted port path of
// holder names, materializing it; 0 where the path names no port object.
func (ctx *Context) portInstanceID(holder *Instance, port string) (int64, error) {
	if holder == nil || port == "" {
		return 0, nil
	}
	current := holder
	segments := strings.Split(port, ".")
	for i, segment := range segments {
		fv, held := current.FeatureValues[segment]
		if !held || (i == len(segments)-1 && !isPortFeature(fv.Feature)) {
			return 0, nil
		}
		next, ok, err := ctx.fvObject(current, segment)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, nil
		}
		current = next
	}
	return current.ID, nil
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
	receiving, outbound, typeMismatch, err := ctx.connectedDeliveries(
		routable, self, send, msg, receiver != "",
	)
	if err != nil {
		return err
	}
	crossing, crossMismatch, err := ctx.ownerDeliveries(self, send, msg, receiver != "")
	if err != nil {
		return err
	}
	typeMismatch = typeMismatch || crossMismatch
	if len(receiving) == 0 && len(crossing) == 0 {
		if typeMismatch && receiver != "" {
			return &SendPortTypeMismatchError{
				Port: send.Target, Receiver: receiver, SignalType: msg.SignalType,
			}
		}
		return &UnroutableSendError{Port: send.Target, Outbound: outbound}
	}
	// A connection joins two objects, so each copy is held to the identity of the
	// object whose port the end resolved to rather than to the sender's. Two
	// destinations naming one port object (through a binding) get one copy.
	// Every destination is resolved before any copy is queued, so a failure
	// leaves nothing behind.
	posted := map[ownerDelivery]bool{}
	postedPorts := map[int64]bool{}
	var routed []Message
	for _, delivery := range append(receiving, crossing...) {
		if posted[delivery] {
			continue
		}
		posted[delivery] = true
		portID, err := ctx.portInstanceID(ctx.instances[delivery.object], delivery.port)
		if err != nil {
			return err
		}
		if portID != 0 {
			if postedPorts[portID] {
				continue
			}
			postedPorts[portID] = true
		}
		copied := msg
		copied.Target = receiver
		copied.Port = delivery.port
		copied.Object = delivery.object
		copied.PortID = portID
		copied.Delivery = DeliverPort
		if receiver != "" {
			copied.Delivery = DeliverPortReceiver
		}
		routed = append(routed, copied)
	}
	for _, m := range routed {
		ctx.PostMessage(m)
	}
	return nil
}

// resolveRoutedReceiver requires the named receiver to be an action or state
// reachable on the sending object before it can accept the routed message.
func (ctx *Context) resolveRoutedReceiver(send lower.Send, self *Instance) (messageAddress, error) {
	separator := "::"
	if send.TargetPath {
		separator = "."
	}
	segments := strings.Split(send.Target, separator)
	if !ctx.routedReceiverExists(send.Scope, segments, len(segments) > 1, self) {
		return messageAddress{}, fmt.Errorf("receiver %q is unresolved", send.Target)
	}
	addrs, err := ctx.resolveAddresses(send, self)
	if err != nil {
		return messageAddress{}, err
	}
	// A routed receiver is a node of the sending object, so of the addresses
	// resolved the one held to that object — or to none — is the receiver's.
	for _, addr := range addrs {
		if addr.Delivery == DeliverReceiver && (addr.Object == 0 || addr.Object == objectID(self)) {
			return addr, nil
		}
	}
	for _, addr := range addrs {
		if addr.Delivery == DeliverReceiver {
			return addr, nil
		}
	}
	return messageAddress{}, fmt.Errorf("receiver %q is not a receiving node", send.Target)
}

// routedReceiverExists prefers a directly declared receiving node over inherited
// feature names, then checks behavior features of the object being addressed.
func (ctx *Context) routedReceiverExists(scope *symbols.Scope, segments []string, path bool, self *Instance) bool {
	if len(segments) == 0 || segments[0] == "" {
		return false
	}
	if path {
		sym, ok := ctx.pathSymbol(scope, segments)
		return ok && isRoutedReceiverSymbol(sym)
	}
	name := segments[0]
	for current := scope; current != nil; {
		for _, sym := range symbols.PreferDeclared(current.LookupLocalAll(name)) {
			if isRoutedReceiverSymbol(sym) {
				return true
			}
		}
		parent := current.Parent()
		if parent == nil || !isRoutedReceiverSymbol(parent.Owner()) {
			break
		}
		current = parent
	}
	if self == nil {
		return false
	}
	for _, feature := range ctx.FeaturesOf(self.Type) {
		if feature.Name == name && isRoutedReceiverSymbol(feature.Symbol) {
			return true
		}
	}
	return false
}

// isRoutedReceiverSymbol accepts only actions and states as routed receivers,
// leaving other named members out of receiver address resolution.
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

// postTo delivers an addressed send to every object its target resolves to,
// one copy per address, each held to that object's own identity.
func (ctx *Context) postTo(msg Message, send lower.Send, self *Instance) error {
	addrs, err := ctx.resolveAddresses(send, self)
	if err != nil {
		return err
	}
	copies := make([]Message, 0, len(addrs))
	for _, addr := range addrs {
		copied := msg
		copied.Target, copied.Port, copied.Object = addr.Name, addr.Port, addr.Object
		copied.Delivery = addr.Delivery
		if addr.Delivery == DeliverPort {
			copied.PortID, err = ctx.portInstanceID(ctx.instances[addr.Object], addr.Port)
			if err != nil {
				return err
			}
		}
		copies = append(copies, copied)
	}
	for _, copied := range copies {
		ctx.PostMessage(copied)
	}
	return nil
}

// resolveAddresses answers what a `send m to t` addressed: the objects t
// belongs to and the port path within them, resolved through the instance
// graph — several where the target reaches through a multi-valued feature. A
// chain the graph does not reach is a port of the sender itself, and
// unroutable if it is neither; a name reaching neither is the receiving node
// of that name.
func (ctx *Context) resolveAddresses(send lower.Send, self *Instance) ([]messageAddress, error) {
	if send.Target == "" {
		// A send addressing no one is for the sending object, or for whoever accepts
		// it where no object sent it.
		if addr, ok := objectAddress(objectID(self)); ok {
			return []messageAddress{addr}, nil
		}
		return []messageAddress{{Delivery: DeliverAnyone}}, nil
	}
	if !send.TargetPath {
		return ctx.namedAddresses(send, self)
	}
	segments := strings.Split(send.Target, ".")
	addrs, err := ctx.featureAddresses(send.Scope, self, segments)
	if err != nil {
		return nil, err
	}
	if len(addrs) > 0 {
		return addrs, nil
	}
	if sym, ok := ctx.portSymbol(send.Scope, send.Target); ok &&
		sym.Kind == symbols.SymbolPortUsage && ctx.ownPortPath(send.Scope, segments) {
		if addr, ok := portAddress(send.Target, objectID(self)); ok {
			return []messageAddress{addr}, nil
		}
	}
	return nil, &UnroutableSendError{Port: send.Target, Address: true}
}

// namedAddresses resolves a target named rather than chained (`R`, `P::R`): an
// unqualified name is a feature, port or receiving node of the sending object,
// and a qualified one is the element its path names, never a same-named element
// of the sender.
func (ctx *Context) namedAddresses(send lower.Send, self *Instance) ([]messageAddress, error) {
	segments := strings.Split(send.Target, "::")
	name := segments[len(segments)-1]
	if len(segments) > 1 {
		return ctx.qualifiedAddresses(send, self, segments)
	}
	addrs, err := ctx.featureAddresses(send.Scope, self, segments)
	if err != nil {
		return nil, err
	}
	if len(addrs) > 0 {
		return addrs, nil
	}
	if sym, resolved := ctx.pathSymbol(send.Scope, segments); resolved &&
		sym.Kind == symbols.SymbolPortUsage {
		if addr, built := portAddress(name, objectID(self)); built {
			return []messageAddress{addr}, nil
		}
		return nil, &UnroutableSendError{Port: send.Target, Address: true}
	}
	if addr, built := receiverAddress(name, objectID(self)); built {
		return []messageAddress{addr}, nil
	}
	return nil, &UnroutableSendError{Port: send.Target, Address: true}
}

// qualifiedAddresses resolves a target naming a namespace path (`alpha::reader`,
// `P::Driver`) to the element that path names, never to a same-named feature of
// the sender: the qualifier chooses the object, so the address is the occurrence
// the path leads through, or unroutable where this run reaches none.
func (ctx *Context) qualifiedAddresses(send lower.Send, self *Instance, segments []string) ([]messageAddress, error) {
	addrs, err := ctx.featureAddresses(send.Scope, nil, segments)
	if err != nil {
		return nil, err
	}
	if len(addrs) > 0 {
		return addrs, nil
	}
	target, resolved := ctx.pathSymbol(send.Scope, segments)
	if !resolved {
		return nil, &UnroutableSendError{Port: send.Target, Address: true}
	}
	name := segments[len(segments)-1]
	// A path leading through no occurrence names an element of the sending
	// behavior's own namespace, where its bare name is that same element:
	// `P::Node::Machine` is the sender's machine, `P::alpha::inPort` is alpha's.
	if local, ok := ctx.pathSymbol(send.Scope, []string{name}); ok && local == target {
		addrs, err := ctx.featureAddresses(send.Scope, self, []string{name})
		if err != nil {
			return nil, err
		}
		if len(addrs) > 0 {
			return addrs, nil
		}
		if target.Kind == symbols.SymbolPortUsage {
			if addr, built := portAddress(name, objectID(self)); built {
				return []messageAddress{addr}, nil
			}
		} else if addr, built := receiverAddress(name, objectID(self)); built {
			return []messageAddress{addr}, nil
		}
	}
	// A receiver no object owns has no identity of its own; a sender that has one
	// cannot address it by name without reaching its own same-named element.
	if self == nil && target.Kind != symbols.SymbolPortUsage {
		if addr, built := receiverAddress(name, 0); built {
			return []messageAddress{addr}, nil
		}
	}
	return nil, &UnroutableSendError{Port: send.Target, Address: true}
}

// featureAddresses walks a target through the instance graph from the object
// its first segment belongs to. A segment held as a collection denotes every
// element it holds (KerML §7.3.4.6), so the walk carries a set of objects and
// the target resolves to one address per object reached, without duplicates.
// No address is reported where a segment names no feature, or one that is
// neither a port nor an occurrence to descend into. A failure to read an
// object of the graph is that failure, not a bad address.
func (ctx *Context) featureAddresses(scope *symbols.Scope, self *Instance, segments []string) ([]messageAddress, error) {
	owner, rest, ok, err := ctx.addressOwner(scope, self, segments)
	if err != nil || !ok {
		return nil, err
	}
	owners := []*Instance{owner}
	var out []messageAddress
	seen := map[messageAddress]bool{}
	add := func(addr messageAddress, built bool) {
		if built && !seen[addr] {
			seen[addr] = true
			out = append(out, addr)
		}
	}
	for i, segment := range rest {
		var next []*Instance
		for _, owner := range owners {
			fv, held := owner.FeatureValues[segment]
			if !held {
				continue
			}
			if isPortFeature(fv.Feature) {
				add(portAddress(strings.Join(rest[i:], "."), owner.ID))
				continue
			}
			// A behavior of an object is a receiving node of it, addressed by name.
			if i == len(rest)-1 && isBehaviorFeature(fv.Feature) {
				add(receiverAddress(segment, owner.ID))
				continue
			}
			held2, err := ctx.fvObjects(owner, segment)
			if err != nil {
				return nil, err
			}
			next = append(next, held2...)
		}
		owners = next
		if len(owners) == 0 {
			return out, nil
		}
	}
	for _, owner := range owners {
		add(objectAddress(owner.ID))
	}
	return out, nil
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

// fvObject reads the object a feature of inst holds as its scalar value,
// materializing it, and reports whether the feature holds one at all. A feature
// value that cannot be read is that failure rather than a feature holding no object.
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

// fvObjects reads every object a feature of inst holds, materializing it: the
// one its scalar value names, or each element of its collection in order.
func (ctx *Context) fvObjects(inst *Instance, name string) ([]*Instance, error) {
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil || fv == nil {
		return nil, err
	}
	var out []*Instance
	for _, v := range heldElements(fv.HeldValue()) {
		if v.Kind != ValInstance {
			continue
		}
		if held, ok := ctx.instances[v.Instance]; ok {
			out = append(out, held)
		}
	}
	return out, nil
}

// heldElements flattens a held value to the values it holds: the elements of a
// collection, or the value itself.
func heldElements(v Value) []Value {
	switch v.Kind {
	case ValSequence:
		if v.Sequence() == nil {
			return nil
		}
		return v.Sequence().Elements()
	case ValSet:
		if v.Set() == nil {
			return nil
		}
		return v.Set().Elements()
	}
	return []Value{v}
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

// messageMatches reports whether a message satisfies an accept whose parameter
// is typed as want, written in scope: the message's type must conform to the
// definition want resolves to, so a subtype message satisfies a supertype
// accept and same-named definitions of different packages stay apart. Where
// either side resolves to no symbol, the written names are compared instead.
func (ctx *Context) messageMatches(m Message, want *ast.QualifiedName, scope *symbols.Scope) bool {
	if want == nil || len(want.Parts) == 0 {
		return true
	}
	if m.Signal != nil && ctx.model != nil {
		if wantSym := ctx.resolveTypeRef(scope, want); wantSym != nil {
			return ctx.model.Conforms(m.Signal, wantSym)
		}
	}
	return m.carriesSignal(want.Parts[len(want.Parts)-1].Text)
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
//
// Names resolve in the send's declaring scope, which sees what a nested block
// imports; scope is the fallback where the lowered send records none.
func (e *EvalContext) buildMessage(scope *symbols.Scope, send lower.Send) (Message, error) {
	if send.Scope != nil {
		scope = send.Scope
	}
	target := send.Target
	if send.IsVia {
		target = send.Receiver
	}
	if sym, ok := e.namedType(scope, send.Message); ok {
		return Message{SignalType: sym.Name, Signal: sym, Target: target, Payload: map[string]Value{}}, nil
	}

	// `send Data(data) via p`, `send shutDown() to self`: the invoked name is
	// what the message is, whether it names a signal definition or an event
	// feature, and its arguments are the payload it carries. An accept matches
	// on that name, so building the message never evaluates the invocation as a
	// call — there is no function of that name to call.
	// A calculation of that name is called, though: what is sent is the value it
	// returns, not a message named after it.
	if invocation, ok := send.Message.(*ast.InvocationExpr); ok && !e.invokesCalc(scope, invocation) {
		return e.buildInvokedMessage(scope, invocation, target)
	}

	value, err := e.Eval(send.Message)
	if err != nil {
		return Message{}, fmt.Errorf("eval send message: %w", err)
	}
	signalType := valueTypeName(value)
	var signal *symbols.Symbol
	if signalType == "" && value.Kind == ValInstance {
		signal = e.ctx.objectSignalSymbol(value.Instance)
		if signal != nil {
			signalType = signal.Name
		}
	}
	if signalType == "" {
		return Message{}, fmt.Errorf("send: message of kind %v has no signal type", value.Kind)
	}
	return Message{
		SignalType: signalType,
		Signal:     signal,
		Target:     target,
		Payload:    map[string]Value{"value": value},
	}, nil
}

// acceptedValue is the value an accept binds its payload name to: the single
// value the message carries, or an occurrence of its signal built from the
// arguments the send named, so `accept p : Ping` sees a Ping object either way.
// The occurrence is cached on the message: a guard evaluated during transition
// selection and the firing that follows read the same object.
func (ctx *Context) acceptedValue(msg Message) (Value, error) {
	if value, ok := msg.Payload["value"]; ok {
		return value, nil
	}
	if msg.Signal == nil {
		return Value{}, fmt.Errorf("%w: %s carries no single value to bind",
			ErrNoValue, orAnonymousSignal(msg.SignalType))
	}
	value, err := ctx.materializeAccepted(msg)
	if err != nil {
		return Value{}, err
	}
	msg.Payload["value"] = value
	return value, nil
}

// materializeAccepted builds the occurrence a typed message binds as, leaving
// no instance behind when a payload argument does not fit it.
func (ctx *Context) materializeAccepted(msg Message) (Value, error) {
	mark := len(ctx.created)
	inst, err := ctx.materialize(msg.Signal, 0)
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return Value{}, fmt.Errorf("materialize accepted %s: %w", msg.SignalType, err)
	}
	features := ctx.FeaturesOf(msg.Signal)
	for name, value := range msg.Payload {
		target := name
		if _, held := inst.FeatureValues[name]; !held {
			n := positionalArg(name)
			if n == 0 || n > len(features) {
				ctx.abandonInstancesSince(mark)
				return Value{}, fmt.Errorf("accepted %s: %q names no feature it carries",
					msg.SignalType, name)
			}
			target = features[n-1].Name
		}
		if err := inst.SetFeatureValue(ctx, target, value); err != nil {
			ctx.abandonInstancesSince(mark)
			return Value{}, fmt.Errorf("accepted %s: %w", msg.SignalType, err)
		}
	}
	return Value{Kind: ValInstance, Instance: inst.ID}, nil
}

// positionalArg returns N for a payload entry named argN, or 0 for any other.
func positionalArg(name string) int {
	if !strings.HasPrefix(name, "arg") {
		return 0
	}
	n, err := strconv.Atoi(name[len("arg"):])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// invokesCalc reports whether an invocation calls a calculation — a calc
// declaration or a library function — rather than naming a signal to send.
func (e *EvalContext) invokesCalc(scope *symbols.Scope, invocation *ast.InvocationExpr) bool {
	if invocation.Type == nil {
		return false
	}
	if e.ctx == nil || e.ctx.resolver == nil || scope == nil {
		return false
	}
	sym, ok := e.ctx.resolver.ResolveQualified(scope, invocation.Type)
	if !ok {
		return false
	}
	return isCalcSymbol(sym)
}

// buildInvokedMessage builds the message of a send written as an invocation.
// The invoked name types the message; each argument is a value it carries,
// named where the send named it and by position where it did not. A single
// positional argument is also carried as `value`, which is what an accept binds
// its payload parameter to.
func (e *EvalContext) buildInvokedMessage(scope *symbols.Scope, invocation *ast.InvocationExpr, target string) (Message, error) {
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

	var signal *symbols.Symbol
	if scope != nil && e.ctx != nil && e.ctx.resolver != nil {
		if sym, ok := e.ctx.resolver.ResolveQualified(scope, invocation.Type); ok && isDefinitionSymbol(sym) {
			signal = sym
		}
	}
	return Message{SignalType: signalType, Signal: signal, Target: target, Payload: payload}, nil
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

// namedType reports the definition expr names when it names a type definition
// rather than denoting a value.
func (e *EvalContext) namedType(scope *symbols.Scope, expr ast.Node) (*symbols.Symbol, bool) {
	qname := ast.AsQualifiedName(expr)
	if qname == nil || scope == nil || e.ctx == nil || e.ctx.resolver == nil {
		return nil, false
	}
	sym, ok := e.ctx.resolver.ResolveQualified(scope, qname)
	if !ok || sym == nil {
		return nil, false
	}
	if !isDefinitionSymbol(sym) {
		return nil, false
	}
	return sym, true
}

// objectSignalSymbol is the definition an object sent as a message
// materializes, which is the type an accept of it matches by conformance.
func (ctx *Context) objectSignalSymbol(id int64) *symbols.Symbol {
	inst, ok := ctx.instances[id]
	if !ok || inst == nil || ctx.model == nil {
		return nil
	}
	if isDefinitionSymbol(inst.Type) {
		return inst.Type
	}
	for _, sup := range ctx.model.AllSupertypes(inst.Type) {
		if isDefinitionSymbol(sup) {
			return sup
		}
	}
	return nil
}

// isDefinitionSymbol reports whether a symbol declares a definition, not a usage.
func isDefinitionSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	_, isDef := sym.Decl.(*ast.Definition)
	return isDef
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
