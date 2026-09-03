package repl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// NoObjectError reports an object id the session holds no object under.
type NoObjectError struct {
	ID int64
	// Superseded records that the runtime still has the object, but no name of
	// the session reaches it any more: a later %instantiate took its name.
	Superseded bool
}

func (e *NoObjectError) Error() string {
	if e.Superseded {
		return fmt.Sprintf("no object #%d in this session: it was superseded, and nothing the session names reaches it", e.ID)
	}
	return fmt.Sprintf("no object #%d in this session: nothing materialized has that identity", e.ID)
}

// ObjectPathError reports a path that stops short of an object, naming the
// segment it stopped at.
type ObjectPathError struct {
	Path    string // the path, as the prompt prints names
	Segment string // the segment that reaches no object
	Reason  string
	// Err is what kept the segment's feature value from materializing, nil when
	// the segment reached a value that is no object or no feature at all.
	Err error
}

func (e *ObjectPathError) Error() string {
	return fmt.Sprintf("%s reaches no object at %q: %s", e.Path, e.Segment, e.Reason)
}

func (e *ObjectPathError) Unwrap() error { return e.Err }

// AmbiguousMachineError reports a state definition an object exhibits as the
// body of several usages, so naming the definition names no one machine.
type AmbiguousMachineError struct {
	Object  string // the object, as objectLabel prints it
	Machine string // the definition asked for, as the prompt prints names
	// Usages are the exhibited usages running it, in declaration order, as the
	// prompt prints names; "" for one without a name.
	Usages []string
}

func (e *AmbiguousMachineError) Error() string {
	names := make([]string, len(e.Usages))
	for i, u := range e.Usages {
		names[i] = u
		if u == "" {
			names[i] = "an unnamed one"
		}
	}
	return fmt.Sprintf("%s exhibits %q as %d machines, so naming the definition attaches to none of them: name the exhibited usage instead — %s",
		e.Object, e.Machine, len(e.Usages), strings.Join(names, " or "))
}

// ExhibitorsError reports a machine `%state <machine>` alone cannot attach to:
// no held object exhibits it, or several do, so no one running performance is meant.
type ExhibitorsError struct {
	Machine string      // the machine asked for, as the prompt prints names
	Type    string      // the type exhibiting it, as the prompt prints names
	Objects []ObjectRef // the held objects exhibiting it, in walk order; none when no object does
}

func (e *ExhibitorsError) Error() string {
	if len(e.Objects) == 0 {
		return fmt.Sprintf("no object of this session exhibits %q, which runs only on an object of %q: use %%instantiate to create one, then %%state <object> or %%state %s <object>",
			e.Machine, e.Type, e.Machine)
	}
	labels := make([]string, len(e.Objects))
	for i, o := range e.Objects {
		labels[i] = fmt.Sprintf("#%d", o.ID)
		if o.Name != "" {
			labels[i] = fmt.Sprintf("#%d of %q", o.ID, o.Name)
		}
	}
	return fmt.Sprintf("%d objects of this session exhibit %q (%s), so naming the machine alone attaches to none of them: use %%state <object> or %%state %s <object> to name one",
		len(e.Objects), e.Machine, strings.Join(labels, ", "), e.Machine)
}

// NotInstantiatedError reports a name no object is materialized under. When
// objects of the definition that name is typed by (or, for a definition, objects
// of usages typed by it) exist, it says so and names what to instantiate.
type NotInstantiatedError struct {
	Name string // the name asked for, as the prompt prints names
	// Definition is the definition the objects in Objects are of, "" when none.
	Definition string
	// Objects are the related objects, in name order.
	Objects []ObjectRef
	// UsageAsked reports that Name is a usage, so Objects are of its definition
	// rather than of the usage; otherwise Name is a definition Objects are typed by.
	UsageAsked bool
}

// ObjectRef names an object the session holds, by identity and by the name it is
// held under (as the prompt prints names) — "" for one only its identity reaches,
// such as a member of a multi-valued part.
type ObjectRef struct {
	ID   int64
	Name string
}

func (e *NotInstantiatedError) Error() string {
	if len(e.Objects) == 0 {
		return fmt.Sprintf("no instance of %q (use %%instantiate first)", e.Name)
	}
	labels := make([]string, len(e.Objects))
	names := make([]string, len(e.Objects))
	for i, o := range e.Objects {
		labels[i] = fmt.Sprintf("#%d", o.ID)
		names[i] = labels[i]
		if o.Name != "" {
			labels[i] = fmt.Sprintf("#%d of %q", o.ID, o.Name)
			names[i] = o.Name
		}
	}
	objects, is := "object "+labels[0]+" is", "it"
	if len(labels) > 1 {
		objects, is = "objects "+strings.Join(labels, ", ")+" are", "one of them"
	}
	if e.UsageAsked {
		return fmt.Sprintf("no instance of the usage %q: %s of its definition %q, not of the usage — use %%instantiate %s to create the usage's object, or name %s to address %s",
			e.Name, objects, e.Definition, e.Name, strings.Join(names, " or "), is)
	}
	return fmt.Sprintf("no instance of the definition %q itself: %s typed by it — name %s to address %s, or use %%instantiate %s to create an object of the definition",
		e.Name, objects, strings.Join(names, " or "), is, e.Name)
}

// objectRef resolves an object argument of a command: an object id the prompt
// printed (#3), a name an object is materialized under or reached through
// (S1::driver::r), or a feature path from such a name (driver.r.motor). The
// second result is the name the object is held under, "" for one no name reaches.
func (s *Session) objectRef(arg string) (*runtime.Instance, string, error) {
	if id, ok := objectID(arg); ok {
		return s.objectByID(id)
	}
	root, segments, ok := objectPath(arg)
	if !ok {
		root, segments = arg, nil
	}
	sym, fqn, lerr := s.lookupSymbol(root)
	if lerr != nil {
		return nil, "", lerr
	}
	inst, name, err := s.objectDenoted(root, fqn)
	if err != nil {
		return nil, "", err
	}
	if inst == nil {
		return nil, "", s.notInstantiated(sym, fqn)
	}
	return s.walkObjectPath(inst, name, strings.TrimSpace(arg), segments)
}

// objectDenoted is objectAt for a name that resolved to fqn: a chain through a
// feature (S1::driver::r) resolves to where the feature is declared
// (S1::Driver::r), so the name as typed is walked first, and the resolved one
// only when what was typed reaches nothing.
func (s *Session) objectDenoted(typed, fqn string) (*runtime.Instance, string, error) {
	if plain, ok := s.plainName(typed); ok && plain != fqn {
		inst, name, err := s.objectAt(plain)
		if err != nil || inst != nil {
			return inst, name, err
		}
	}
	return s.objectAt(fqn)
}

// heldObject returns the object the session holds under a name objectRef
// reported: an object id, or a qualified name reaching one.
func (s *Session) heldObject(name string) (*runtime.Instance, bool) {
	if id, ok := objectID(name); ok {
		inst, _, err := s.objectByID(id)
		return inst, err == nil
	}
	inst, _, err := s.objectAt(name)
	return inst, err == nil && inst != nil
}

// objectID reads the `#<n>` spelling the prompt prints object identities in.
func objectID(arg string) (int64, bool) {
	digits, ok := strings.CutPrefix(arg, "#")
	if !ok || digits == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(digits, 10, 64)
	return id, err == nil
}

// objectPath reads an object argument as a feature chain: the root name and the
// features walked from it. Text that is no such chain reports false.
func objectPath(text string) (string, []string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil, false
	}
	p := parser.New(source.New("object", []byte(text)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return "", nil, false
	}
	var segments []string
	for {
		chain, ok := expr.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		member := qualifiedText(chain.Member)
		if member == "" {
			return "", nil, false
		}
		segments = append([]string{member}, segments...)
		expr = chain.Operand
	}
	ref, ok := expr.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ref.Name.Global {
		return "", nil, false
	}
	root := qualifiedText(ref.Name)
	if root == "" {
		return "", nil, false
	}
	return root, segments, true
}

// qualifiedText spells a qualified name as the index registers it, "" for one
// with an empty segment.
func qualifiedText(qn *ast.QualifiedName) string {
	if qn == nil || len(qn.Parts) == 0 {
		return ""
	}
	segments := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		if part.Text == "" {
			return ""
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::")
}

// objectByID returns the object of an identity, under the name the session
// reaches it by — "" for one no session object holds.
func (s *Session) objectByID(id int64) (*runtime.Instance, string, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	inst := s.heldByID(ctx, id)
	if inst == nil {
		_, retained := ctx.Instance(id)
		return nil, "", &NoObjectError{ID: id, Superseded: retained}
	}
	return inst, s.nameOf(inst), nil
}

// heldByID finds an object among those the session holds: the named roots and
// what their materialized features hold. An object the runtime retains after a
// second %instantiate took its name is not among them.
func (s *Session) heldByID(ctx *runtime.Context, id int64) *runtime.Instance {
	var found *runtime.Instance
	s.walkHeldObjects(ctx, func(cur carrier) bool {
		if cur.inst.ID == id {
			found = cur.inst
		}
		return found == nil
	})
	return found
}

// nameOf is the qualified name the session reaches an object by: the name it is
// materialized under, or the owner's name and the feature holding it alone. An
// object no session object holds, or one held only among others of a
// multi-valued feature, has none.
func (s *Session) nameOf(inst *runtime.Instance) string {
	var path []string
	for depth := 0; inst != nil && depth <= maxFeatureValueDepth; depth++ {
		if name := s.instanceName(inst); name != "" {
			return strings.Join(append([]string{name}, path...), "::")
		}
		owner, _ := inst.Owner()
		if owner == nil {
			return ""
		}
		feature := holdingAlone(owner, inst.ID)
		if feature == "" {
			return ""
		}
		path = append([]string{feature}, path...)
		inst = owner
	}
	return ""
}

// holdingAlone is the first feature of owner, by name, holding the object id as
// its one value, so owner's name and the feature reach that object and no
// other; "" when only a multi-valued feature holds it.
func holdingAlone(owner *runtime.Instance, id int64) string {
	feature := ""
	for name, fv := range owner.FeatureValues {
		if fv == nil || fv.Values.Kind != runtime.ValInvalid || (feature != "" && name > feature) {
			continue
		}
		if held, ok := fv.Value.Object(); ok && held == id {
			feature = name
		}
	}
	return feature
}

// objectAt is objectNamed with the reason a name reaches no object: nil with no
// error when nothing is materialized under any prefix of it.
func (s *Session) objectAt(fqn string) (*runtime.Instance, string, error) {
	if fqn == "" {
		return nil, "", nil
	}
	segments := strings.Split(fqn, "::")
	for i := len(segments); i > 0; i-- {
		key := strings.Join(segments[:i], "::")
		inst, ok := s.instances[key]
		if !ok {
			continue
		}
		return s.walkObjectPath(inst, key, notationName(fqn), segments[i:])
	}
	return nil, "", nil
}

// walkObjectPath follows a chain of feature values from inst, reporting the
// segment of path (the chain as typed) that reaches no object. name is what inst
// is reached by, extended as the walk descends.
func (s *Session) walkObjectPath(inst *runtime.Instance, name, path string, segments []string) (*runtime.Instance, string, error) {
	if len(segments) == 0 {
		return inst, name, nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	for _, seg := range segments {
		fail := func(reason string) error {
			return &ObjectPathError{Path: path, Segment: seg, Reason: reason}
		}
		if _, declared := inst.FeatureValues[seg]; !declared {
			return nil, "", fail(fmt.Sprintf("%s has no feature %q", objectLabel(inst, name), seg))
		}
		fv, serr := inst.GetFeatureValue(ctx, seg)
		if serr != nil {
			return nil, "", &ObjectPathError{Path: path, Segment: seg, Err: serr,
				Reason: fmt.Sprintf("feature %q of object #%d could not be materialized: %v", seg, inst.ID, serr)}
		}
		if fv == nil {
			return nil, "", fail(fmt.Sprintf("%s has no feature %q", objectLabel(inst, name), seg))
		}
		held := fv.HeldValue()
		id, isObject := held.Object()
		if !isObject {
			if held.Kind == runtime.ValInvalid {
				return nil, "", fail(fmt.Sprintf("feature %q of object #%d holds no value", seg, inst.ID))
			}
			return nil, "", fail(fmt.Sprintf("feature %q of object #%d holds %s, which is not an object", seg, inst.ID, formatValue(ctx, held)))
		}
		child, ok := ctx.Instance(id)
		if !ok {
			return nil, "", fail(fmt.Sprintf("feature %q of object #%d holds object #%d, which this session no longer has", seg, inst.ID, id))
		}
		inst, name = child, name+"::"+seg
	}
	return inst, name, nil
}

// notInstantiated reports that nothing is materialized under fqn, and — when
// objects of the definition the name is typed by, or typed by the definition it
// names, exist — what to instantiate instead.
func (s *Session) notInstantiated(sym *symbols.Symbol, fqn string) error {
	e := &NotInstantiatedError{Name: notationName(fqn)}
	if sym == nil || sym.Decl == nil || len(s.instances) == 0 {
		return e
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return e
	}
	model := ctx.Model()
	var definition *symbols.Symbol
	switch sym.Decl.(type) {
	case *ast.Usage:
		e.UsageAsked = true
		for _, sup := range model.AllSupertypes(sym) {
			if _, isDef := sup.Decl.(*ast.Definition); isDef {
				definition = sup
				break
			}
		}
	case *ast.Definition:
		definition = sym
	}
	if definition == nil || definition.Decl == nil {
		return e
	}
	e.Definition = notationName(s.fqnOf(definition))
	// An error names what exists; it materializes nothing to find it.
	s.walkHeldObjects(ctx, func(cur carrier) bool {
		if carriesDeclaration(model, cur.inst.Type, definition.Decl) {
			ref := ObjectRef{ID: cur.inst.ID}
			if _, byIdentity := objectID(cur.name); !byIdentity {
				ref.Name = notationName(cur.name)
			}
			e.Objects = append(e.Objects, ref)
		}
		return true
	})
	return e
}

// exhibitor is a held object running a machine, with the machines of it that run
// under one declaration.
type exhibitor struct {
	carrier
	machines []*runtime.ObjectBehavior
}

// exhibitorsOf finds the held objects exhibiting sym's machine, in name order.
// Nothing is materialized: an object not yet held runs no machine to attach to.
func (s *Session) exhibitorsOf(ctx *runtime.Context, sym *symbols.Symbol) []exhibitor {
	var found []exhibitor
	s.walkHeldObjects(ctx, func(cur carrier) bool {
		if machines := cur.inst.ExhibitedStatesOf(sym); len(machines) > 0 {
			found = append(found, exhibitor{carrier: cur, machines: machines})
		}
		return true
	})
	return found
}

// exhibitorsError reports sym's machine as one `%state <machine>` alone cannot
// attach to, naming the objects exhibiting it.
func (s *Session) exhibitorsError(name string, sym *symbols.Symbol, exhibitors []exhibitor) error {
	e := &ExhibitorsError{Machine: name}
	if sym.OwnerScope != nil {
		if declaring := sym.OwnerScope.Owner(); declaring != nil {
			e.Type = notationName(s.fqnOf(declaring))
		}
	}
	for _, ex := range exhibitors {
		ref := ObjectRef{ID: ex.inst.ID}
		if _, byIdentity := objectID(ex.name); !byIdentity {
			ref.Name = notationName(ex.name)
		}
		e.Objects = append(e.Objects, ref)
	}
	return e
}

// ambiguousMachine reports a definition self exhibits as the body of several
// machines, naming the usages that would address one.
func (s *Session) ambiguousMachine(name string, self *runtime.Instance, selfFQN string, exhibited []*runtime.ObjectBehavior) error {
	e := &AmbiguousMachineError{Object: objectLabel(self, selfFQN), Machine: name}
	for _, b := range exhibited {
		usage := ""
		if member := b.Member(); member != nil && member.Name != "" {
			usage = notationName(s.fqnOf(member))
		}
		e.Usages = append(e.Usages, usage)
	}
	return e
}

// fqnOf is the qualified name the index registers a symbol under, its own name
// when the index registers none.
func (s *Session) fqnOf(sym *symbols.Symbol) string {
	if idx := s.browseIndex(); idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}
