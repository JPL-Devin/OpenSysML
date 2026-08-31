package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrUnroutableSend is returned when a `send … via p` reaches no end able to
// receive it: p is joined to nothing, or only to ends whose flow features carry
// outward. It lives here rather than in errors.go because routing is the only
// thing that raises it.
var ErrUnroutableSend = errors.New("send reaches no receiving port")

// ErrSendViaUnknownPort reports a routed send naming no sender port.
var ErrSendViaUnknownPort = errors.New("send via names no port of the sender")

// ErrSendPortTypeMismatch reports a typed receiving port rejecting a message.
var ErrSendPortTypeMismatch = errors.New("send message type is not carried by the receiving port")

// ErrUnreachableSendReceiver reports a routed receiver that cannot be resolved.
var ErrUnreachableSendReceiver = errors.New("send receiver is unreachable")

// UnknownSendPortError gives the routed send's invalid port and receiver.
type UnknownSendPortError struct {
	Port     string
	Receiver string
}

func (e *UnknownSendPortError) Error() string {
	return fmt.Sprintf("%s: port %q names no port of the sender for receiver %q",
		ErrSendViaUnknownPort, e.Port, e.Receiver)
}

func (e *UnknownSendPortError) Unwrap() error { return ErrSendViaUnknownPort }

// SendPortTypeMismatchError gives the routed send's incompatible type.
type SendPortTypeMismatchError struct {
	Port       string
	Receiver   string
	SignalType string
}

func (e *SendPortTypeMismatchError) Error() string {
	return fmt.Sprintf("%s: port %q carries no flow feature for message type %q to receiver %q",
		ErrSendPortTypeMismatch, e.Port, e.SignalType, e.Receiver)
}

func (e *SendPortTypeMismatchError) Unwrap() error { return ErrSendPortTypeMismatch }

// UnreachableSendReceiverError gives the routed send's unresolved receiver.
type UnreachableSendReceiverError struct {
	Port     string
	Receiver string
}

func (e *UnreachableSendReceiverError) Error() string {
	return fmt.Sprintf("%s: receiver %q is not reachable through port %q",
		ErrUnreachableSendReceiver, e.Receiver, e.Port)
}

func (e *UnreachableSendReceiverError) Unwrap() error { return ErrUnreachableSendReceiver }

// UnroutableSendError reports a send that could not be delivered, naming the
// port it was sent through and the ends joined to it that refused it, so the
// model can be corrected. An addressed send names a target rather than a port it
// routes through, so it reports the path that reached no port of any object.
type UnroutableSendError struct {
	Port     string   // the port or target the send named, as written
	Outbound []string // ends joined to Port that only carry outward
	Address  bool     // the send addressed a target rather than routing through a port
}

func (e *UnroutableSendError) Error() string {
	if e.Address {
		return fmt.Sprintf("%s: %q names no port of an object the sender can address", ErrUnroutableSend, e.Port)
	}
	if len(e.Outbound) == 0 {
		return fmt.Sprintf("%s: port %q is joined to no port that can receive it", ErrUnroutableSend, e.Port)
	}
	return fmt.Sprintf(
		"%s: port %q is joined only to outbound ends (%s)",
		ErrUnroutableSend, e.Port, strings.Join(e.Outbound, ", "),
	)
}

func (e *UnroutableSendError) Unwrap() error { return ErrUnroutableSend }

// ownerDelivery is where a connection of an object holding the sender delivers:
// the peer object reached and the port within it the message arrives at.
type ownerDelivery struct {
	object int64
	port   string
}

// ownerDeliveries answers where a `send … via p` arrives through the connectors
// of the objects holding the sender: a part's port joined by its owner to a
// sibling's port reaches that sibling on the sibling's own identity, since the
// owner writes its ends as paths from itself (SysML v2 §7.16). The second result
// reports an end refusing the message because its inward features are typed for
// another, which decides the error a send delivered nowhere gives.
func (ctx *Context) ownerDeliveries(
	self *Instance, send lower.Send, signalType string, typed bool,
) ([]ownerDelivery, bool, error) {
	var out []ownerDelivery
	mismatch := false
	seen := map[ownerDelivery]bool{}
	path := ""
	for child := self; child != nil && child.owner != nil; child = child.owner {
		if child.ownerFeature == "" {
			break
		}
		if path == "" {
			path = child.ownerFeature
		} else {
			path = child.ownerFeature + "." + path
		}
		owner := child.owner
		sending := path + "." + send.Target
		conns := ctx.realizedConnections(ctx.objectConnections(owner.Type), owner)
		for _, conn := range conns {
			if !joins(conn.Ends, sending) {
				continue
			}
			for _, end := range conn.Ends {
				if end == sending {
					continue
				}
				accepts := true
				if typed {
					var refused bool
					accepts, refused = ctx.endReceivesMessage(conn.Scope, end, signalType)
					mismatch = mismatch || refused
				} else {
					accepts = ctx.endReceives(conn.Scope, end)
				}
				if !accepts {
					continue
				}
				addr, ok, err := ctx.featureAddress(conn.Scope, owner, strings.Split(end, "."))
				if err != nil {
					return nil, mismatch, err
				}
				if !ok || addr.Delivery != DeliverPort || addr.Object == 0 {
					continue
				}
				delivery := ownerDelivery{object: addr.Object, port: addr.Port}
				if seen[delivery] {
					continue
				}
				seen[delivery] = true
				out = append(out, delivery)
			}
		}
	}
	return out, mismatch, nil
}

// routableConnections are the connections a `send … via p` of a behavior can
// travel over: the ones the behavior's own body declares, and the ones declared
// by the part performing it, whose ports the send names. The performer is the
// object when one performs the behavior, and otherwise the part the behavior was
// declared in, which performs it by owning it (SysML v2 §7.16).
func (ctx *Context) routableConnections(own []lower.Connection, self *Instance, scope *symbols.Scope) []lower.Connection {
	performer := ctx.performerConnections(self, scope)
	if len(performer) == 0 {
		return own
	}
	out := make([]lower.Connection, 0, len(own)+len(performer))
	return append(append(out, own...), performer...)
}

// performerConnections returns the connections of the part performing a
// behavior: those of the object's type when an object performs it, and those of
// the enclosing part when none does, so a behavior declared in a part reaches
// that part's own ports either way.
func (ctx *Context) performerConnections(self *Instance, scope *symbols.Scope) []lower.Connection {
	if self != nil {
		return ctx.objectConnections(self.Type)
	}
	return ctx.objectConnections(enclosingPart(scope))
}

// enclosingPart returns the symbol of the nearest part-like declaration a
// behavior is nested in — the part that performs it — or nil when the behavior
// is not declared in one. Anything else it is nested in — a state, a region, a
// transition, another behavior, a package — is passed through.
func enclosingPart(scope *symbols.Scope) *symbols.Symbol {
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil {
			continue
		}
		switch decl := owner.Decl.(type) {
		case *ast.Usage:
			if structuralUsage(decl.Kind) {
				return owner
			}
		case *ast.Definition:
			if structuralDefinition(decl.Kind) {
				return owner
			}
		}
	}
	return nil
}

// structuralUsage reports whether a usage kind declares something that has ports
// and performs behaviors, rather than a behavior or a namespace.
func structuralUsage(k ast.UsageKind) bool {
	switch k {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence, ast.UsageIndividual,
		ast.UsagePort, ast.UsageInterface, ast.UsageConnection, ast.UsageStruct, ast.UsageClass:
		return true
	}
	return false
}

// structuralDefinition reports the same of a definition kind.
func structuralDefinition(k ast.DefinitionKind) bool {
	switch k {
	case ast.DefPart, ast.DefItem, ast.DefOccurrence, ast.DefIndividual,
		ast.DefPort, ast.DefInterface, ast.DefConnection, ast.DefStruct, ast.DefClass:
		return true
	}
	return false
}

// objectConnections returns the lowered connections an object of typeSym owns:
// the ones its type declares and the ones it inherits, most specific first. They
// are lowered once per type.
func (ctx *Context) objectConnections(typeSym *symbols.Symbol) []lower.Connection {
	if typeSym == nil {
		return nil
	}
	if conns, ok := ctx.objectConns[typeSym]; ok {
		return conns
	}
	conns := []lower.Connection{}
	for _, decl := range append([]*symbols.Symbol{typeSym}, ctx.model.AllSupertypes(typeSym)...) {
		conns = append(conns, lower.ToObjectConnections(decl.Decl, declScope(decl))...)
	}
	ctx.objectConns[typeSym] = conns
	return conns
}

// realizedConnections drops the connections a variation offers but the owner
// routing through them did not select: a `variant interface`'s connection joins
// its ends only where that variant is the one its variation is bound to
// (SysML v2 §7.20). Connections belonging to no variation are always realized.
func (ctx *Context) realizedConnections(conns []lower.Connection, self *Instance) []lower.Connection {
	out := make([]lower.Connection, 0, len(conns))
	for _, conn := range conns {
		if conn.Variation != "" && ctx.selectedVariant(conn, self) != conn.Variant {
			continue
		}
		out = append(out, conn)
	}
	return out
}

// selectedVariant answers what the owner of conn bound its variation to. A
// connection an object declares is governed by that object's own selection, so
// two objects of a type each route the variant they selected; a connection the
// behavior declares is governed by the selection made without an object.
func (ctx *Context) selectedVariant(conn lower.Connection, self *Instance) string {
	if conn.Owner == lower.OwnerObject && self != nil {
		return ctx.heldVariant(self, conn.Variation)
	}
	if variant, ok := ctx.selectedVariants[variantSelection{variation: conn.Variation}]; ok {
		return variant
	}
	return ""
}

// heldVariant reads the variant an object's variation feature value holds, which is that
// object's own selection whether or not it was read before the message was sent.
func (ctx *Context) heldVariant(inst *Instance, variation string) string {
	fv, err := inst.GetFeatureValue(ctx, variation)
	if err != nil || fv == nil {
		return ""
	}
	if fv.Value.Kind != ValVariant || fv.Value.Variant == nil {
		return ""
	}
	return fv.Value.Variant.Name
}

// receivingEnds sorts the ends joined to sendingPort into the ones a message can
// arrive at and the ones it cannot: a connector joins its ends without a
// direction of its own, so what a message may traverse is decided by the flow
// features of the port each end names, conjugated where the port's type is
// (SysML v2 §7.15). Each list is in declaration order and without duplicates.
func (ctx *Context) receivingEnds(conns []lower.Connection, sendingPort string) (receiving, outbound []string) {
	if sendingPort == "" {
		return nil, nil
	}
	seen := map[string]bool{sendingPort: true}
	for _, conn := range conns {
		if !joins(conn.Ends, sendingPort) {
			continue
		}
		for _, end := range conn.Ends {
			if seen[end] {
				continue
			}
			seen[end] = true
			if ctx.endReceives(conn.Scope, end) {
				receiving = append(receiving, end)
				continue
			}
			outbound = append(outbound, end)
		}
	}
	return receiving, outbound
}

// receivingEndsForMessage finds connected receiving ends and distinguishes
// typed message mismatches from ends that are outbound or otherwise unroutable.
func (ctx *Context) receivingEndsForMessage(
	conns []lower.Connection, sendingPort, signalType string,
) (receiving, outbound []string, typeMismatch bool) {
	if sendingPort == "" {
		return nil, nil, false
	}
	seen := map[string]bool{sendingPort: true}
	for _, conn := range conns {
		if !joins(conn.Ends, sendingPort) {
			continue
		}
		for _, end := range conn.Ends {
			if seen[end] {
				continue
			}
			seen[end] = true
			accepts, mismatch := ctx.endReceivesMessage(conn.Scope, end, signalType)
			if accepts {
				receiving = append(receiving, end)
			} else if mismatch {
				typeMismatch = true
			} else {
				outbound = append(outbound, end)
			}
		}
	}
	return receiving, outbound, typeMismatch
}

// endReceives reports whether a message can arrive at the port an end names: one
// of its flow features carries inward, after conjugation. A port declaring no
// flow features constrains nothing, so it receives whatever reaches it — as does
// an end naming something this run cannot resolve, which routing is not the
// place to report.
func (ctx *Context) endReceives(scope *symbols.Scope, end string) bool {
	sym, ok := ctx.portSymbol(scope, end)
	if !ok {
		return true
	}
	features := ctx.model.PortFeatures(sym)
	if len(features) == 0 {
		return true
	}
	for _, feature := range features {
		if feature.Direction != ast.DirOut {
			return true
		}
	}
	return false
}

// endReceivesMessage classifies a port for this message, reporting a mismatch
// only when typed inward features reject it; conformance is accepted either way.
func (ctx *Context) endReceivesMessage(scope *symbols.Scope, end, signalType string) (bool, bool) {
	sym, ok := ctx.portSymbol(scope, end)
	if !ok {
		return true, false
	}
	features := ctx.model.PortFeatures(sym)
	if len(features) == 0 {
		return true, false
	}
	inward := false
	for _, feature := range features {
		if feature.Direction != ast.DirOut {
			inward = true
		}
	}
	if !inward {
		return false, false
	}
	messageSym := ctx.resolveType(scope, signalType)
	if messageSym == nil {
		return true, false
	}
	typed := false
	for _, feature := range features {
		if feature.Direction == ast.DirOut {
			continue
		}
		typeSym := ctx.extractType(feature.Symbol)
		if typeSym == nil {
			continue
		}
		typed = true
		if ctx.model.Conforms(typeSym, messageSym) || ctx.model.Conforms(messageSym, typeSym) {
			return true, false
		}
	}
	if typed {
		return false, true
	}
	return true, false
}

// resolveType resolves a routed message type through the sender's scope,
// returning nil when the type is unavailable for conservative routing.
func (ctx *Context) resolveType(scope *symbols.Scope, name string) *symbols.Symbol {
	if ctx.resolver == nil || name == "" {
		return nil
	}
	parts := strings.Split(name, "::")
	qn := &ast.QualifiedName{Parts: make([]ast.NameSegment, len(parts))}
	for i, part := range parts {
		qn.Parts[i] = ast.NameSegment{Text: part}
	}
	sym, ok := ctx.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return nil
	}
	if canonical, ok := ctx.resolver.ResolveAliasTarget(sym); ok {
		return canonical
	}
	return sym
}

// portSymbol resolves the path an end names — `p` or a nested `p.q` — to the
// port it declares, each segment after the first being a member of the one
// before it.
func (ctx *Context) portSymbol(scope *symbols.Scope, path string) (*symbols.Symbol, bool) {
	return ctx.pathSymbol(scope, strings.Split(path, "."))
}

// pathSymbol resolves the first segment in scope and every later one as a member
// of the one before it, whichever separator the segments were written with.
func (ctx *Context) pathSymbol(scope *symbols.Scope, segments []string) (*symbols.Symbol, bool) {
	if scope == nil || ctx.resolver == nil || len(segments) == 0 || segments[0] == "" {
		return nil, false
	}
	sym, ok := ctx.resolver.LookupName(scope, segments[0])
	for _, segment := range segments[1:] {
		if !ok || sym == nil {
			return nil, false
		}
		sym, ok = ctx.model.LookupMember(sym, segment)
	}
	if !ok || sym == nil {
		return nil, false
	}
	return sym, true
}

// joins reports whether a connection has an end naming want.
func joins(ends []string, want string) bool {
	for _, end := range ends {
		if end == want {
			return true
		}
	}
	return false
}
