package runtime

import (
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// routableConnections are the connections a `send … via p` of a behavior can
// travel over: the ones the behavior's own body declares, and the ones declared
// by the object performing it, whose ports the send names when the behavior is
// performed by an object.
func (ctx *Context) routableConnections(own []lower.Connection, self *Instance) []lower.Connection {
	if self == nil {
		return own
	}
	objects := ctx.objectConnections(self.Type)
	if len(objects) == 0 {
		return own
	}
	out := make([]lower.Connection, 0, len(own)+len(objects))
	return append(append(out, own...), objects...)
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
		conns = append(conns, lower.ToObjectConnections(decl.Decl)...)
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
