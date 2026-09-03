package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ConnectorEnd is one end of a materialized connector: the name of the end
// feature it occupies, empty for an end the model leaves unnamed, and the value
// the end attaches to. The value is the connected feature itself — an object
// held at an end is the very object the connected feature holds, not a copy of
// it (KerML 1.0 §7.4.6) — so writing through one is read through the other.
type ConnectorEnd struct {
	Name  string
	Value Value
}

// participantEndName is the feature the ends of a connector occupy when they
// have no names of their own: `Links::Link::participant`, the ordered
// `[2..*]` feature every link's ends subset. A binary connector's `source` and
// `target` subset it in turn, so an object of a connector with any other arity
// holds its ends there.
const participantEndName = "participant"

// materializeConnectorFeatureValue fills a feature value that holds a connector usage: the object it
// denotes, with its ends attached to the features the `connect` clause names,
// resolved against the instance that owns the connector.
func (ctx *Context) materializeConnectorFeatureValue(owner *Instance, fv *FeatureValue, name string) error {
	if !fv.Feature.IsScalar() {
		return &ConnectorEndError{
			Connector: fmt.Sprintf("%s.%s", owner.Type.Name, name),
			End:       name,
			Location:  ctx.symbolLocation(fv.Feature.Symbol),
			Err:       errors.New("a connector of more than one object has no set of ends to attach"),
		}
	}
	kept, held := owner.keptConnectors[fv]
	conn, err := ctx.materializeConnectorAs(owner, fv.Feature.Symbol, ctx.connectorBaseOf(fv.Feature), kept)
	if err != nil {
		return err
	}
	if held {
		// A probe discards the object, so the identity is kept for the one materialized after it.
		ctx.noteProbeUndo(func() { owner.keepConnector(fv, kept) })
		delete(owner.keptConnectors, fv)
	}
	fv.Value = Value{Kind: ValInstance, Instance: conn.ID}
	fv.Materialized = true
	return nil
}

// connectorBaseOf returns the type an object of a connector usage is
// materialized from: the definition it names, or the usage itself when it names
// none — an implicitly typed `interface iface connect a.p to b.q` (SysML v2
// §8.3.13) specializes a library connector whose declaration is indexed without
// a body, so the object carries the usage's own features and its ends.
func (ctx *Context) connectorBaseOf(feat *EffectiveFeature) *symbols.Symbol {
	if base := ctx.CompositeTypeOf(feat); base != nil {
		return base
	}
	return feat.Symbol
}

// connectorEndFeatures returns the end features an object of the connector
// usage typeSym carries beyond the ones declared, whose names declared holds.
// The ends of an implicitly typed connector are declared by the library
// connector it specializes, whose declaration is indexed without a body, so
// they are answered from the ends the usage attaches: `source` and `target` for
// a binary connector, `participant` for any other arity.
func (ctx *Context) connectorEndFeatures(typeSym *symbols.Symbol, declared map[string]bool) []EffectiveFeature {
	ends := ctx.model.ConnectorEndAttachments(typeSym)
	if len(ends) == 0 {
		return nil
	}
	var out []EffectiveFeature
	for _, end := range ends {
		name, mult := end.Name, singleValue()
		if name == "" {
			name, mult = participantEndName, participants(len(ends))
		}
		if declared[name] {
			continue
		}
		declared[name] = true
		out = append(out, EffectiveFeature{
			Name:         name,
			Symbol:       end.EndFeature,
			OwnerType:    typeSym,
			Multiplicity: mult,
		})
	}
	return out
}

// materializeConnector builds the object the connector usage connSym denotes in
// the context of owner, the instance whose features its ends name. base is the
// type the object is materialized from.
func (ctx *Context) materializeConnector(owner *Instance, connSym, base *symbols.Symbol) (*Instance, error) {
	return ctx.materializeConnectorAs(owner, connSym, base, 0)
}

// materializeConnectorAs materializes a connector under the given identity, 0 for
// the next one the context hands out.
func (ctx *Context) materializeConnectorAs(owner *Instance, connSym, base *symbols.Symbol, id int64) (*Instance, error) {
	ends := ctx.model.ConnectorEndAttachments(connSym)
	if len(ends) == 0 {
		return nil, fmt.Errorf("%w: %s declares no end to attach", ErrConnectorEnd, connectorName(connSym))
	}

	// An end may name the connector itself, or another connector that names this
	// one back, which would attach ends forever.
	ownerID := int64(0)
	if owner != nil {
		ownerID = owner.ID
	}
	key := connectorRef{owner: ownerID, connector: connSym}
	if ctx.materializingConnectors[key] {
		return nil, fmt.Errorf("%w: connector %s attaches to itself", ErrCyclicFeatureValue, connectorName(connSym))
	}
	ctx.materializingConnectors[key] = true
	defer delete(ctx.materializingConnectors, key)

	// The behaviors start once every end is attached: a connector an end of which
	// cannot attach leaves nothing behind and touches no other object, so its
	// identity stays free for another attempt.
	mark := len(ctx.created)
	inst, err := ctx.materializeOwnedBy(base, id, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}

	var unnamed []Value
	for _, end := range ends {
		val, err := ctx.attachConnectorEnd(owner, connSym, end)
		if err != nil {
			ctx.abandonInstancesSince(mark)
			return nil, err
		}
		inst.Ends = append(inst.Ends, ConnectorEnd{Name: end.Name, Value: val})
		if end.Name == "" {
			unnamed = append(unnamed, val)
			continue
		}
		ctx.bindEndFeatureValue(inst, end, val)
	}
	if len(unnamed) > 0 {
		ctx.bindParticipants(inst, inst.Ends)
	}
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
	return inst, nil
}

// attachConnectorEnd evaluates what one end attaches to against the instance
// owning the connector, so the end holds the connected feature of that very
// object. An end naming nothing reachable is reported with its location: a
// connector that cannot be attached is no connector.
func (ctx *Context) attachConnectorEnd(owner *Instance, connSym *symbols.Symbol, end semantics.ConnectorEndAttachment) (Value, error) {
	if end.Attachment == nil {
		return Value{}, ctx.connectorEndError(connSym, end, errors.New("names no feature"))
	}
	if owner == nil {
		return Value{}, ctx.connectorEndError(connSym, end, errors.New("no object owns the connector"))
	}
	scope := connSym.OwnerScope
	if scope == nil {
		scope = owner.Type.OwnerScope
	}
	ec := NewEvalContextIn(ctx, scope, owner)
	defer ec.beginStep()()
	val, err := ec.Eval(end.Attachment)
	if err != nil {
		return Value{}, ctx.connectorEndError(connSym, end, err)
	}
	if val.Kind == ValInvalid {
		return Value{}, ctx.connectorEndError(connSym, end, errors.New("holds no value"))
	}
	return val, nil
}

// bindEndFeatureValue writes an attached end into the feature value named after the end feature,
// adding the feature value when the object carries none: the ends of an implicitly typed
// connector are declared by a library connector, indexed without its body.
func (ctx *Context) bindEndFeatureValue(inst *Instance, end semantics.ConnectorEndAttachment, val Value) {
	fv, ok := inst.FeatureValues[end.Name]
	if !ok {
		fv = &FeatureValue{Feature: &EffectiveFeature{
			Name:         end.Name,
			Symbol:       end.EndFeature,
			OwnerType:    inst.Type,
			Multiplicity: singleValue(),
		}}
		inst.FeatureValues[end.Name] = fv
	}
	ctx.noteProbeWrite(fv)
	fv.Value = val
	fv.Values = Value{}
	fv.Materialized = true
}

// bindParticipants writes every end into the participant feature value, in declaration
// order, for a connector whose ends have no names of their own: that feature is
// where a link holds the things it relates.
func (ctx *Context) bindParticipants(inst *Instance, ends []ConnectorEnd) {
	seq := NewSequence()
	for _, end := range ends {
		seq.Append(end.Value)
	}
	fv, ok := inst.FeatureValues[participantEndName]
	if !ok {
		fv = &FeatureValue{Feature: &EffectiveFeature{
			Name:         participantEndName,
			OwnerType:    inst.Type,
			Multiplicity: participants(len(ends)),
		}}
		inst.FeatureValues[participantEndName] = fv
	}
	ctx.noteProbeWrite(fv)
	fv.Value = Value{}
	fv.Values = NewSequenceValue(seq)
	fv.Materialized = true
}

// singleValue is the multiplicity of a feature holding one value, which an end
// of a connector does.
func singleValue() semantics.Range {
	return semantics.Range{
		Lower: semantics.Bound{Value: 1, Known: true},
		Upper: semantics.Bound{Value: 1, Known: true},
	}
}

// participants is the multiplicity of the participant feature of a connector
// with n ends: the ends it has, and no upper bound, as the library declares.
func participants(n int) semantics.Range {
	return semantics.Range{
		Lower: semantics.Bound{Value: int64(n), Known: true},
		Upper: semantics.Bound{Known: true, Infinite: true},
	}
}

// OwnedConnectors returns the connectors the instance owns that no feature names —
// an anonymous `connect a.p to b.q` member — materializing each once, in
// declaration order. A named connector is reached through its feature value instead.
func (inst *Instance) OwnedConnectors(ctx *Context) ([]*Instance, error) {
	defer ctx.beginRun()()
	members := ctx.anonymousConnectors(inst.Type)
	inst.holdAnonymous(ctx, len(members))
	for i, member := range members {
		if inst.anonymous[i] != 0 {
			continue
		}
		if _, err := inst.materializeAnonymousConnector(ctx, i, member); err != nil {
			return nil, err
		}
	}
	return inst.anonymousConnectors(ctx)
}

// materializeAnonymousConnector materializes the instance's i-th anonymous
// connector, member, under the identity a carry-over kept for it, if any.
func (inst *Instance) materializeAnonymousConnector(ctx *Context, i int, member *symbols.Symbol) (*Instance, error) {
	kept := inst.keptIdentity(i)
	conn, err := ctx.materializeConnectorAs(inst, member, member, kept)
	if err != nil {
		return nil, err
	}
	// A probe discards the object, so the identity is kept for the one materialized after it.
	ctx.noteProbeUndo(func() {
		if i < len(inst.anonymous) {
			inst.anonymous[i] = 0
		}
		if i < len(inst.keptAnonymous) {
			inst.keptAnonymous[i] = kept
		}
	})
	inst.anonymous[i] = conn.ID
	if i < len(inst.keptAnonymous) {
		inst.keptAnonymous[i] = 0
	}
	return conn, nil
}

// MaterializedConnectors returns the anonymous connectors the instance has
// already materialized, none when they were never asked for; it materializes nothing.
func (inst *Instance) MaterializedConnectors(ctx *Context) []*Instance {
	if inst.anonymous == nil {
		return nil
	}
	conns, _ := inst.anonymousConnectors(ctx)
	return conns
}

// KeptConnectorIDs returns the identities of the connectors a carry-over set
// aside for the instance to materialize again, anonymous and named, ascending.
func (inst *Instance) KeptConnectorIDs() []int64 {
	ids := make([]int64, 0, len(inst.keptAnonymous)+len(inst.keptConnectors))
	for _, id := range inst.keptAnonymous {
		if id != 0 {
			ids = append(ids, id)
		}
	}
	for _, id := range inst.keptConnectors {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// RestoreConnector materializes again the connector a carry-over set aside under id,
// which takes that identity back; nil when the instance kept no such identity.
func (inst *Instance) RestoreConnector(ctx *Context, id int64) (*Instance, error) {
	if id == 0 {
		return nil, nil
	}
	if i := slices.Index(inst.keptAnonymous, id); i >= 0 {
		return inst.restoreAnonymousConnector(ctx, i)
	}
	for name, fv := range inst.FeatureValues {
		if inst.keptConnectors[fv] != id {
			continue
		}
		if _, err := inst.GetFeatureValue(ctx, name); err != nil {
			return nil, err
		}
		conn, _ := ctx.Instance(id)
		return conn, nil
	}
	return nil, nil
}

// restoreAnonymousConnector materializes the instance's i-th anonymous connector
// alone, leaving its siblings and the identities kept for them as they are; nil
// when the declarations as they are now have no i-th one.
func (inst *Instance) restoreAnonymousConnector(ctx *Context, i int) (*Instance, error) {
	defer ctx.beginRun()()
	members := ctx.anonymousConnectors(inst.Type)
	if i >= len(members) {
		return nil, nil
	}
	inst.holdAnonymous(ctx, len(members))
	return inst.materializeAnonymousConnector(ctx, i, members[i])
}

// holdAnonymous gives the instance a slot per anonymous connector declared, n of
// them, 0 standing for one not materialized yet. A probe leaves the slots as found.
func (inst *Instance) holdAnonymous(ctx *Context, n int) {
	if inst.anonymous != nil && len(inst.anonymous) >= n {
		return
	}
	prior := inst.anonymous
	ctx.noteProbeUndo(func() { inst.anonymous = prior })
	inst.anonymous = make([]int64, n)
	copy(inst.anonymous, prior)
}

// keepAnonymous sets aside, for a carry-over, the identities of the anonymous
// connectors materialized here, alongside any kept earlier and not taken back yet.
func (inst *Instance) keepAnonymous() {
	for i, id := range inst.anonymous {
		if id == 0 {
			continue
		}
		for len(inst.keptAnonymous) <= i {
			inst.keptAnonymous = append(inst.keptAnonymous, 0)
		}
		inst.keptAnonymous[i] = id
	}
	inst.anonymous = nil
}

// keptIdentity returns the identity the instance's i-th anonymous connector had
// before a carry-over, 0 when it had none.
func (inst *Instance) keptIdentity(i int) int64 {
	if i >= len(inst.keptAnonymous) {
		return 0
	}
	return inst.keptAnonymous[i]
}

// anonymousConnectors returns the objects the instance's anonymous connectors
// materialized to, skipping those not materialized yet and dropping any the
// context no longer holds.
func (inst *Instance) anonymousConnectors(ctx *Context) ([]*Instance, error) {
	out := make([]*Instance, 0, len(inst.anonymous))
	for _, id := range inst.anonymous {
		if id == 0 {
			continue
		}
		if conn, ok := ctx.Instance(id); ok {
			out = append(out, conn)
		}
	}
	return out, nil
}

// anonymousConnectors returns the connector usages an object of typeSym owns
// that declare no name, which are members of it all the same: `connect a.p to
// b.q;` joins its ends whether or not it is named. A usage is instantiated from
// what types it, so the declarations searched are typeSym's own and those of the
// types it specializes, most specific first.
func (ctx *Context) anonymousConnectors(typeSym *symbols.Symbol) []*symbols.Symbol {
	if typeSym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, decl := range append([]*symbols.Symbol{typeSym}, ctx.model.AllSupertypes(typeSym)...) {
		// A library supertype states the metamodel frame every element
		// specializes, not the model's own connections.
		if decl != typeSym && ctx.libraryDeclared(decl) {
			continue
		}
		for _, member := range declMembers(decl.Decl) {
			usage, ok := member.(*ast.Usage)
			if !ok || usage.Ident.Name != "" || usage.Ident.ShortName != "" {
				continue
			}
			if len(usage.ConnectorEnds) < 2 {
				continue
			}
			// A succession or transition carries ends too, and relates its ends in
			// time rather than joining them, so it is no connector to materialize.
			sym := anonymousConnectorSymbol(decl, usage)
			if !ctx.model.IsConnectorUsage(sym) {
				continue
			}
			out = append(out, sym)
		}
	}
	return out
}

// anonymousConnectorSymbol returns a symbol standing for an anonymous connector
// declared in typeSym's body. An unnamed declaration is registered under no
// name, so the symbol is built here — it carries the declaration, the scope it
// was written in and the file it came from, which is what resolving its ends and
// reporting them needs.
func anonymousConnectorSymbol(typeSym *symbols.Symbol, usage *ast.Usage) *symbols.Symbol {
	kind := symbols.SymbolConnectionUsage
	switch usage.Kind {
	case ast.UsageInterface:
		kind = symbols.SymbolInterfaceUsage
	case ast.UsageAllocation:
		kind = symbols.SymbolAllocationUsage
	}
	return &symbols.Symbol{
		Kind:       kind,
		Decl:       usage,
		OwnerScope: typeSym.Scope,
		DocName:    typeSym.DocName,
		DeclSpan:   usage.Span(),
	}
}

// connectorName names a connector for an error message: its declared name, or
// the kind of connector it is when it declares none.
func connectorName(sym *symbols.Symbol) string {
	if sym == nil {
		return "connector"
	}
	if sym.Name != "" {
		return sym.Name
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Keyword != "" {
		return "anonymous " + usage.Keyword
	}
	return "anonymous connector"
}

// ConnectorEndError reports a connector end that cannot be attached to what it
// names, carrying where the end was written so the model can be corrected.
type ConnectorEndError struct {
	Connector string // the connector as declared
	End       string // the feature the end names, as written
	Location  string // file and position of the end
	Err       error  // why the end could not be attached
}

func (e *ConnectorEndError) Error() string {
	where := ""
	if e.Location != "" {
		where = " at " + e.Location
	}
	return fmt.Sprintf("%s: %s end %q%s: %v", ErrConnectorEnd, e.Connector, e.End, where, e.Err)
}

func (e *ConnectorEndError) Unwrap() error { return e.Err }

// endText renders the feature an end names as it was written, for a message.
func endText(node ast.Node) string {
	if chain, ok := node.(*ast.FeatureChainExpr); ok {
		return endText(chain.Operand) + "." + ast.SimpleName(chain.Member)
	}
	qn := ast.AsQualifiedName(node)
	if qn == nil {
		return ""
	}
	parts := make([]string, len(qn.Parts))
	for i, part := range qn.Parts {
		parts[i] = part.Text
	}
	return strings.Join(parts, ".")
}

// Is reports that this error is an ErrConnectorEnd, so a caller can test for
// the condition without knowing which end of which connector failed.
func (e *ConnectorEndError) Is(target error) bool { return target == ErrConnectorEnd }

// connectorEndError builds the diagnostic for an end that cannot be attached.
func (ctx *Context) connectorEndError(connSym *symbols.Symbol, end semantics.ConnectorEndAttachment, cause error) error {
	written := endText(end.Attachment)
	if written == "" {
		written = end.Name
	}
	file := ""
	if connSym != nil {
		file = connSym.DocName
	}
	span := source.Span{}
	if end.End != nil {
		span = end.End.Span()
	}
	return &ConnectorEndError{
		Connector: connectorName(connSym),
		End:       written,
		Location:  ctx.sourceLocation(file, span),
		Err:       cause,
	}
}
