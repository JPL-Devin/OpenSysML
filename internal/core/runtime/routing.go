package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ErrUnroutableSend is returned when a `send … via p` reaches no end able to
// receive it: p is joined to nothing, or only to ends whose flow features carry
// outward. It lives here rather than in errors.go because routing is the only
// thing that raises it.
var ErrUnroutableSend = errors.New("send reaches no receiving port")

// UnroutableSendError reports a send that could not be delivered, naming the
// port it was sent through and the ends joined to it that refused it, so the
// model can be corrected.
type UnroutableSendError struct {
	Port     string   // the port the send named, as written
	Outbound []string // ends joined to Port that only carry outward
}

func (e *UnroutableSendError) Error() string {
	if len(e.Outbound) == 0 {
		return fmt.Sprintf("%s: port %q is joined to no port that can receive it", ErrUnroutableSend, e.Port)
	}
	return fmt.Sprintf(
		"%s: port %q is joined only to outbound ends (%s)",
		ErrUnroutableSend, e.Port, strings.Join(e.Outbound, ", "),
	)
}

func (e *UnroutableSendError) Unwrap() error { return ErrUnroutableSend }

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
// is not declared in one. A behavior nested in another behavior is reached
// through its own body, so the search passes through it.
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
		default:
			return nil
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

// heldVariant reads the variant an object's variation slot holds, which is that
// object's own selection whether or not it was read before the message was sent.
func (ctx *Context) heldVariant(inst *Instance, variation string) string {
	slot, err := inst.GetSlot(ctx, variation)
	if err != nil || slot == nil {
		return ""
	}
	if slot.Value.Kind != ValVariant || slot.Value.Variant == nil {
		return ""
	}
	return slot.Value.Variant.Name
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

// portSymbol resolves the path an end names — `p` or a nested `p.q` — to the
// port it declares, each segment after the first being a member of the one
// before it.
func (ctx *Context) portSymbol(scope *symbols.Scope, path string) (*symbols.Symbol, bool) {
	if scope == nil || ctx.resolver == nil || path == "" {
		return nil, false
	}
	segments := strings.Split(path, ".")
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
