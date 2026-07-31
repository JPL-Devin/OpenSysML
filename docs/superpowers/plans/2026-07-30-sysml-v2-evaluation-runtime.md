# SysML v2 Evaluation Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Tiers 1–3 of the SysML v2 evaluation runtime: feature flattening, instance model, and expression evaluator.

**Architecture:** New `internal/core/runtime/` package consuming `semantics.Model`. Tier 1 = effective feature lists (type shape). Tier 2 = instance model with lazy slot materialization. Tier 3 = evaluator over instance model with frame stack, KerML builtin functions, step counter.

**Tech Stack:** Go 1.25.10, existing `internal/core/{ast,symbols,semantics}` packages, standard library only.

---

## File Structure

**Create:**
- `internal/core/runtime/context.go` — Context, ID allocator, instance registry, memoization
- `internal/core/runtime/value.go` — Value, Sequence, Set types
- `internal/core/runtime/errors.go` — Runtime error types
- `internal/core/runtime/shape.go` — EffectiveFeature, FeaturesOf (Tier 1)
- `internal/core/runtime/instance.go` — Instance, Slot, Instantiate (Tier 2)
- `internal/core/runtime/eval.go` — EvalContext, Eval dispatcher (Tier 3)
- `internal/core/runtime/builtins.go` — KerML builtin registry + implementations
- `internal/core/runtime/value_equality.go` — Value equality/hashing for Set
- `internal/core/runtime/doc.go` — Package documentation
- `internal/core/runtime/shape_test.go` — Tier 1 tests
- `internal/core/runtime/instance_test.go` — Tier 2 tests
- `internal/core/runtime/eval_test.go` — Tier 3 tests
- `internal/core/runtime/builtins_test.go` — Builtin tests
- `internal/core/runtime/runtime_integration_test.go` — End-to-end tests

---

### Task 1: Scaffold Runtime Package + Error Types

**Files:**
- Create: `internal/core/runtime/doc.go`
- Create: `internal/core/runtime/errors.go`

- [ ] **Step 1: Create package directory**

```bash
mkdir -p internal/core/runtime
```

- [ ] **Step 2: Write package documentation (doc.go)**

```go
// Package runtime provides the SysML v2 execution runtime: expression
// evaluation, instance materialization, and KerML operator library.
//
// This package implements Tiers 1–3 of the SysML v2 runtime:
//   - Tier 1: Feature flattening (effective-feature lists per type)
//   - Tier 2: Instance model (lazy slot materialization, multiplicity-driven collections)
//   - Tier 3: Expression evaluator (literals, operators, feature access, calc invocation, KerML builtins)
//
// Key types:
//   - Context: Runtime execution context (ID allocator, instance registry, memoization)
//   - Value: Runtime-evaluable value (int/real/bool/string/null/instance/Sequence/Set)
//   - EffectiveFeature: One entry in a type's effective feature list (Tier 1 schema)
//   - Instance: Runtime-materialized object with typed slots
//   - EvalContext: Lexical environment for evaluation (frame stack)
//
// Integration:
//   - Consumes semantics.Model (inherits features, multiplicity, constant folding)
//   - Gates on pass-validated models (LevelConstraint success)
//   - One Context per workspace session (LSP/REPL lifetime)
//
// Behavioral simulation (actions, state machines) is out of scope (future Tiers 4–5).
package runtime
```

- [ ] **Step 3: Define runtime error types (errors.go)**

```go
package runtime

import (
	"errors"
	"fmt"
)

var (
	// ErrStepLimitExceeded is returned when the evaluation step counter exceeds maxSteps.
	ErrStepLimitExceeded = errors.New("evaluation step limit exceeded")

	// ErrUnresolvedReference is returned when a feature reference cannot be resolved.
	ErrUnresolvedReference = errors.New("unresolved reference")

	// ErrTypeMismatch is returned when an operation receives a value of unexpected type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrMultiplicityViolation is returned when a slot access/assignment violates multiplicity bounds.
	ErrMultiplicityViolation = errors.New("multiplicity violation")

	// ErrUninitializedSlot is returned when accessing a slot that has no value and no default.
	ErrUninitializedSlot = errors.New("uninitialized slot")
)

// EvalError wraps an evaluation error with source context.
type EvalError struct {
	Msg string
	Err error
}

func (e *EvalError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *EvalError) Unwrap() error {
	return e.Err
}
```

- [ ] **Step 4: Verify directory structure**

Run: `ls -la internal/core/runtime/`

Expected: `doc.go` and `errors.go` exist

- [ ] **Step 5: Commit**

```bash
git add internal/core/runtime/doc.go internal/core/runtime/errors.go
git commit -m "feat(runtime): scaffold package + error types"
```

---

### Task 2: Value Model (value.go)

**Files:**
- Create: `internal/core/runtime/value.go`

- [ ] **Step 1: Write failing test (value_test.go scaffold)**

```go
package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func TestValueConstWrapping(t *testing.T) {
	// Test that runtime.Value correctly wraps semantics.Value
	semVal := semantics.Value{Kind: semantics.ValInt, Int: 42}
	v := Value{Kind: ValConst, Const: semVal}
	
	if v.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", v.Kind)
	}
	if v.Const.Int != 42 {
		t.Errorf("expected Int=42, got %d", v.Const.Int)
	}
}

func TestSequenceOperations(t *testing.T) {
	seq := NewSequence()
	v1 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	v2 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}}
	
	seq.Append(v1)
	seq.Append(v2)
	
	if seq.Size() != 2 {
		t.Errorf("expected size 2, got %d", seq.Size())
	}
	
	elem, err := seq.At(0)
	if err != nil || elem.Const.Int != 1 {
		t.Errorf("expected elem[0]=1, got %v, err=%v", elem, err)
	}
}

func TestSetOperations(t *testing.T) {
	set := NewSet()
	v1 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	v2 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}} // duplicate
	
	set.Add(v1)
	set.Add(v2) // should not increase size
	
	if set.Size() != 1 {
		t.Errorf("expected size 1 (dedupe), got %d", set.Size())
	}
	
	if !set.Contains(v1) {
		t.Error("expected set to contain v1")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core/runtime/... -run TestValue`

Expected: FAIL with "undefined: Value"

- [ ] **Step 3: Implement Value types (value.go)**

```go
package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// ValueKind distinguishes runtime value types.
type ValueKind int

const (
	ValInvalid ValueKind = iota
	ValConst             // wraps semantics.Value (int/real/bool/infinity)
	ValNull
	ValString
	ValInstance
	ValSequence
	ValSet
)

// Value is a runtime-evaluable value.
type Value struct {
	Kind     ValueKind
	Const    semantics.Value // ValConst: reuse static evaluator
	Str      string          // ValString
	Instance int64           // ValInstance: instance ID
	Sequence *Sequence       // ValSequence
	Set      *Set            // ValSet
}

// Sequence is an ordered collection (slice-backed).
type Sequence struct {
	elements []Value
}

// NewSequence creates an empty Sequence.
func NewSequence() *Sequence {
	return &Sequence{elements: make([]Value, 0)}
}

// Append adds a value to the end of the sequence.
func (s *Sequence) Append(val Value) {
	s.elements = append(s.elements, val)
}

// At returns the element at the given index (0-based).
func (s *Sequence) At(index int) (Value, error) {
	if index < 0 || index >= len(s.elements) {
		return Value{}, fmt.Errorf("index %d out of range [0, %d)", index, len(s.elements))
	}
	return s.elements[index], nil
}

// Size returns the number of elements.
func (s *Sequence) Size() int {
	return len(s.elements)
}

// Elements returns the underlying slice (for iteration).
func (s *Sequence) Elements() []Value {
	return s.elements
}

// Set is an unordered unique collection (map-backed, using valueKey for equality).
type Set struct {
	elements map[valueKey]Value
}

// NewSet creates an empty Set.
func NewSet() *Set {
	return &Set{elements: make(map[valueKey]Value)}
}

// Add inserts a value into the set (deduplicates by valueKey).
func (s *Set) Add(val Value) {
	key := valueKey(val)
	s.elements[key] = val
}

// Contains checks if the value is in the set.
func (s *Set) Contains(val Value) bool {
	_, ok := s.elements[valueKey(val)]
	return ok
}

// Size returns the number of unique elements.
func (s *Set) Size() int {
	return len(s.elements)
}

// Elements returns all elements in arbitrary order.
func (s *Set) Elements() []Value {
	result := make([]Value, 0, len(s.elements))
	for _, v := range s.elements {
		result = append(result, v)
	}
	return result
}
```

- [ ] **Step 4: Stub valueKey (value_equality.go minimal)**

```go
package runtime

import (
	"hash/fnv"
)

// valueKey is a comparable projection of Value for use as map key.
type valueKey struct {
	kind    ValueKind
	intVal  int64
	realVal float64
	boolVal bool
	infVal  bool
	strVal  string
	instID  int64
	colHash uint64
}

// valueKey extracts a comparable key from a Value.
func valueKey(v Value) valueKey {
	key := valueKey{kind: v.Kind}
	switch v.Kind {
	case ValConst:
		switch v.Const.Kind {
		case 1: // semantics.ValInt
			key.intVal = v.Const.Int
		case 2: // semantics.ValReal
			key.realVal = v.Const.Real
		case 3: // semantics.ValBool
			key.boolVal = v.Const.Bool
		case 4: // semantics.ValInfinity
			key.infVal = true
		}
	case ValString:
		key.strVal = v.Str
	case ValInstance:
		key.instID = v.Instance
	case ValSequence:
		key.colHash = hashSequence(v.Sequence)
	case ValSet:
		key.colHash = hashSet(v.Set)
	}
	return key
}

// hashSequence computes a content-based hash for a Sequence.
func hashSequence(seq *Sequence) uint64 {
	h := fnv.New64a()
	for _, elem := range seq.elements {
		k := valueKey(elem)
		h.Write([]byte{byte(k.kind)})
		// Simplified: hash intVal, strVal, instID (full implementation in later task)
		if k.intVal != 0 {
			h.Write([]byte{byte(k.intVal), byte(k.intVal >> 8)})
		}
	}
	return h.Sum64()
}

// hashSet computes a content-based hash for a Set (order-invariant).
func hashSet(set *Set) uint64 {
	// Sum hashes of elements (order-invariant)
	var sum uint64
	for k := range set.elements {
		sum += uint64(k.intVal) // Simplified
	}
	return sum
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v ./internal/core/runtime/... -run TestValue`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/core/runtime/value.go internal/core/runtime/value_equality.go internal/core/runtime/value_test.go
git commit -m "feat(runtime): Value model + Sequence/Set types"
```

---

### Task 3: Context + ID Allocator (context.go foundation)

**Files:**
- Create: `internal/core/runtime/context.go`

- [ ] **Step 1: Write failing test (context_test.go)**

```go
package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

func TestContextIDAllocation(t *testing.T) {
	model := &semantics.Model{} // minimal mock
	ctx := NewContext(model, 100000)
	
	id1 := ctx.allocateID()
	id2 := ctx.allocateID()
	
	if id1 == id2 {
		t.Error("expected unique IDs, got duplicates")
	}
	if id1 != 1 || id2 != 2 {
		t.Errorf("expected sequential IDs 1,2; got %d,%d", id1, id2)
	}
}

func TestContextStepCounter(t *testing.T) {
	model := &semantics.Model{}
	ctx := NewContext(model, 10)
	
	for i := 0; i < 10; i++ {
		if err := ctx.incrementStep(); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	
	// 11th step should error
	if err := ctx.incrementStep(); err == nil {
		t.Error("expected step limit error, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core/runtime/... -run TestContext`

Expected: FAIL with "undefined: NewContext"

- [ ] **Step 3: Implement Context (context.go)**

```go
package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Context carries runtime execution state. One per workspace session.
type Context struct {
	model     *semantics.Model
	nextID    int64
	steps     int64
	maxSteps  int64
	instances map[int64]*Instance
	features  map[*symbols.Symbol][]EffectiveFeature
}

// NewContext creates a runtime context backed by the given semantic model.
// maxSteps sets the runaway guard (step counter limit).
func NewContext(model *semantics.Model, maxSteps int64) *Context {
	return &Context{
		model:     model,
		nextID:    1, // IDs start at 1 (0 = invalid)
		steps:     0,
		maxSteps:  maxSteps,
		instances: make(map[int64]*Instance),
		features:  make(map[*symbols.Symbol][]EffectiveFeature),
	}
}

// allocateID returns the next instance ID and increments the counter.
func (ctx *Context) allocateID() int64 {
	id := ctx.nextID
	ctx.nextID++
	return id
}

// incrementStep increments the step counter and returns ErrStepLimitExceeded if limit reached.
func (ctx *Context) incrementStep() error {
	ctx.steps++
	if ctx.steps >= ctx.maxSteps {
		return fmt.Errorf("%w (%d steps)", ErrStepLimitExceeded, ctx.maxSteps)
	}
	return nil
}

// getInstance retrieves an instance by ID.
func (ctx *Context) getInstance(id int64) (*Instance, bool) {
	inst, ok := ctx.instances[id]
	return inst, ok
}

// registerInstance stores an instance in the registry.
func (ctx *Context) registerInstance(inst *Instance) {
	ctx.instances[inst.ID] = inst
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/core/runtime/... -run TestContext`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/runtime/context.go internal/core/runtime/context_test.go
git commit -m "feat(runtime): Context with ID allocator + step counter"
```

---

### Task 4: Tier 1 — Feature Flattening (shape.go + shape_test.go)

**Files:**
- Create: `internal/core/runtime/shape.go`
- Create: `internal/core/runtime/shape_test.go`

- [ ] **Step 1: Write failing test for FeaturesOf (shape_test.go)**

```go
package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestFeaturesOf_SimpleDef(t *testing.T) {
	src := `
		part def Wheel {
			attribute diameter: Real;
			attribute width: Real;
		}
	`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	// Resolve Wheel symbol
	wheelSym := resolveSymbol(t, root, "Wheel")
	
	features := ctx.FeaturesOf(wheelSym)
	
	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	
	// Check feature names
	names := []string{features[0].Feature.Name, features[1].Feature.Name}
	expectedNames := []string{"diameter", "width"}
	if !equalSlices(names, expectedNames) {
		t.Errorf("expected %v, got %v", expectedNames, names)
	}
}

func parseAndBuildModel(t *testing.T, src string) (*semantics.Model, *ast.RootNamespace) {
	t.Helper()
	srcFile := source.New("test.sysml", []byte(src))
	p := parser.New(srcFile)
	root, err := p.ParseFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	
	idx := symbols.NewIndex()
	idx.IndexDocument("test.sysml", root)
	
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return model, root
}

func resolveSymbol(t *testing.T, root *ast.RootNamespace, name string) *symbols.Symbol {
	t.Helper()
	// Walk root scope to find symbol by name
	scope := root.Scope // assume Index attached scope
	sym := scope.LookupLocal(name)
	if sym == nil {
		t.Fatalf("symbol %q not found", name)
	}
	return sym
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core/runtime/... -run TestFeaturesOf`

Expected: FAIL with "undefined: FeaturesOf"

- [ ] **Step 3: Implement EffectiveFeature type (shape.go)**

```go
package runtime

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// EffectiveFeature is one entry in a type's effective feature list (Tier 1 schema).
type EffectiveFeature struct {
	Feature      *symbols.Symbol  // feature symbol
	Type         *symbols.Symbol  // resolved type (via typing relationship), nil if untyped
	Multiplicity semantics.Range  // extracted multiplicity, zero if none
	DefaultValue ast.Node         // Usage.Value expression, nil if no default
}
```

- [ ] **Step 4: Implement FeaturesOf (shape.go)**

```go
// FeaturesOf returns the effective feature list for a type symbol: all visible
// features (own + inherited with masking), in stable order (local first, then
// supertypes breadth-first). Result is memoized in ctx.features.
func (ctx *Context) FeaturesOf(sym *symbols.Symbol) []EffectiveFeature {
	// Check cache
	if cached, ok := ctx.features[sym]; ok {
		return cached
	}
	
	// Get all members (dedupe short+primary aliases by pointer)
	members := ctx.model.MembersOf(sym)
	seenPtrs := make(map[*symbols.Symbol]bool)
	var uniqueMembers []*symbols.Symbol
	for _, m := range members {
		if !seenPtrs[m] {
			seenPtrs[m] = true
			uniqueMembers = append(uniqueMembers, m)
		}
	}
	
	// Build EffectiveFeature for each member
	result := make([]EffectiveFeature, 0, len(uniqueMembers))
	for _, member := range uniqueMembers {
		feat := EffectiveFeature{
			Feature: member,
		}
		
		// Extract type (via typing relationship)
		feat.Type = ctx.extractType(member)
		
		// Extract multiplicity
		mult, ok := ctx.model.MultiplicityOf(member)
		if ok {
			feat.Multiplicity = mult
		}
		
		// Extract default value
		if usage, ok := member.Decl.(*ast.Usage); ok {
			feat.DefaultValue = usage.Value
		}
		
		result = append(result, feat)
	}
	
	// Memoize
	ctx.features[sym] = result
	return result
}

// extractType resolves the type of a feature via its typing relationship.
func (ctx *Context) extractType(member *symbols.Symbol) *symbols.Symbol {
	// Walk relationships to find RelTyping
	rels := relationshipsOf(member.Decl)
	for _, rel := range rels {
		if rel.Kind == RelTyping {
			// Resolve target
			typeSym, ok := ctx.model.Resolver().ResolveQualified(member.OwnerScope, rel.Target)
			if ok {
				return typeSym
			}
		}
	}
	return nil
}

// relationshipsOf extracts relationship edges from a declaration (copied from passes/constraint.go).
type generalizationKind int

const (
	RelSpecializes generalizationKind = iota
	RelSubsets
	RelRedefines
	RelTyping
)

type relationship struct {
	Kind   generalizationKind
	Target *ast.QualifiedName
}

func relationshipsOf(decl ast.Node) []relationship {
	switch d := decl.(type) {
	case *ast.Definition:
		return extractRelationships(d.Relationships)
	case *ast.Usage:
		return extractRelationships(d.Relationships)
	default:
		return nil
	}
}

func extractRelationships(rels []*ast.Relationship) []relationship {
	var result []relationship
	for _, r := range rels {
		var kind generalizationKind
		switch r.Kind {
		case ast.RelSpecializes:
			kind = RelSpecializes
		case ast.RelSubsets:
			kind = RelSubsets
		case ast.RelRedefines:
			kind = RelRedefines
		case ast.RelTyping:
			kind = RelTyping
		default:
			continue
		}
		for _, target := range r.Targets {
			result = append(result, relationship{Kind: kind, Target: target})
		}
	}
	return result
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v ./internal/core/runtime/... -run TestFeaturesOf`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/core/runtime/shape.go internal/core/runtime/shape_test.go
git commit -m "feat(runtime): Tier 1 feature flattening (FeaturesOf)"
```

---

### Task 5: Tier 2 Foundation — Instance + Slot Types (instance.go scaffold)

**Files:**
- Create: `internal/core/runtime/instance.go`

- [ ] **Step 1: Define Instance and Slot types (instance.go)**

```go
package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Instance is a runtime-materialized object (Tier 2).
type Instance struct {
	ID    int64              // unique identity
	Type  *symbols.Symbol    // the def/usage symbol this instantiates
	Slots map[string]*Slot   // feature name → slot
}

// Slot holds the runtime value(s) for one feature.
type Slot struct {
	Feature      *EffectiveFeature
	Value        Value   // scalar slot (multiplicity [1])
	Values       Value   // collection slot (Sequence or Set)
	Materialized bool    // lazy flag: has this slot been instantiated?
}

// GetSlot retrieves the slot for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet.
func (inst *Instance) GetSlot(ctx *Context, name string) (*Slot, error) {
	slot, ok := inst.Slots[name]
	if !ok {
		return nil, fmt.Errorf("slot %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}
	
	// Lazy materialization deferred to Task 6
	return slot, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/core/runtime/`

Expected: SUCCESS (no errors)

- [ ] **Step 3: Commit**

```bash
git add internal/core/runtime/instance.go
git commit -m "feat(runtime): Tier 2 Instance + Slot types (scaffold)"
```

---

### Task 6: Tier 2 — Instantiate + Lazy Slot Materialization (instance.go logic + tests)

**Files:**
- Modify: `internal/core/runtime/instance.go`
- Create: `internal/core/runtime/instance_test.go`

- [ ] **Step 1: Write failing test for Instantiate (instance_test.go)**

```go
package runtime

import (
	"testing"
)

func TestInstantiate_SimplePartDef(t *testing.T) {
	src := `
		part def Wheel {
			attribute diameter: Real = 0.5;
		}
	`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	wheelSym := resolveSymbol(t, root, "Wheel")
	
	inst, err := ctx.Instantiate(wheelSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	
	if inst.ID != 1 {
		t.Errorf("expected ID=1, got %d", inst.ID)
	}
	
	if len(inst.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(inst.Slots))
	}
	
	diameterSlot, ok := inst.Slots["diameter"]
	if !ok {
		t.Fatal("expected 'diameter' slot")
	}
	
	// Check default value evaluated
	if diameterSlot.Value.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", diameterSlot.Value.Kind)
	}
	if diameterSlot.Value.Const.Real != 0.5 {
		t.Errorf("expected Real=0.5, got %f", diameterSlot.Value.Const.Real)
	}
}

func TestInstantiate_IDAllocation(t *testing.T) {
	src := `part def A {}`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	aSym := resolveSymbol(t, root, "A")
	
	inst1, _ := ctx.Instantiate(aSym)
	inst2, _ := ctx.Instantiate(aSym)
	
	if inst1.ID == inst2.ID {
		t.Error("expected unique IDs")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core/runtime/... -run TestInstantiate`

Expected: FAIL with "undefined: Instantiate"

- [ ] **Step 3: Implement Instantiate (instance.go)**

```go
// Instantiate materializes an instance of the given usage/definition symbol.
// Allocates ID, creates slots per FeaturesOf(sym), evaluates default values,
// leaves composite features lazy. Returns the instance or an error.
func (ctx *Context) Instantiate(sym *symbols.Symbol) (*Instance, error) {
	// Allocate ID
	id := ctx.allocateID()
	
	// Create instance
	inst := &Instance{
		ID:    id,
		Type:  sym,
		Slots: make(map[string]*Slot),
	}
	
	// Get effective features
	features := ctx.FeaturesOf(sym)
	
	// Create slot for each feature
	for i := range features {
		feat := &features[i]
		slot := &Slot{
			Feature:      feat,
			Materialized: false,
		}
		
		// Evaluate default value if present and scalar
		if feat.DefaultValue != nil && feat.Multiplicity.Upper.Value <= 1 {
			// Use semantics.Eval for constant defaults (Tier 3 will use full evaluator)
			if semVal, ok := ctx.model.Eval(feat.DefaultValue); ok {
				slot.Value = Value{Kind: ValConst, Const: semVal}
				slot.Materialized = true
			}
		}
		
		inst.Slots[feat.Feature.Name] = slot
	}
	
	// Register instance
	ctx.registerInstance(inst)
	
	return inst, nil
}
```

- [ ] **Step 4: Implement lazy materialization in GetSlot (instance.go)**

```go
// GetSlot retrieves the slot for the named feature, materializing it lazily
// if it's a composite feature that hasn't been accessed yet.
func (inst *Instance) GetSlot(ctx *Context, name string) (*Slot, error) {
	slot, ok := inst.Slots[name]
	if !ok {
		return nil, fmt.Errorf("slot %q not found in instance %d (type %s)", name, inst.ID, inst.Type.Name)
	}
	
	// If already materialized, return
	if slot.Materialized {
		return slot, nil
	}
	
	// Lazy instantiation: if feature is composite (has a type that's a part/item def)
	if slot.Feature.Type != nil {
		// Check multiplicity
		mult := slot.Feature.Multiplicity
		if mult.Upper.Value == 1 {
			// Scalar: instantiate one
			childInst, err := ctx.Instantiate(slot.Feature.Type)
			if err != nil {
				return nil, err
			}
			slot.Value = Value{Kind: ValInstance, Instance: childInst.ID}
		} else {
			// Collection: instantiate up to lower bound (or 0 if unbounded)
			count := int(mult.Lower.Value)
			if count < 0 {
				count = 0
			}
			
			// Determine collection type (Sequence vs Set)
			seq := NewSequence()
			for i := 0; i < count; i++ {
				childInst, err := ctx.Instantiate(slot.Feature.Type)
				if err != nil {
					return nil, err
				}
				seq.Append(Value{Kind: ValInstance, Instance: childInst.ID})
			}
			slot.Values = Value{Kind: ValSequence, Sequence: seq}
		}
		slot.Materialized = true
	}
	
	return slot, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v ./internal/core/runtime/... -run TestInstantiate`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/core/runtime/instance.go internal/core/runtime/instance_test.go
git commit -m "feat(runtime): Tier 2 Instantiate + lazy slot materialization"
```

---

### Task 7: Tier 3 Foundation — EvalContext + Eval Scaffolding (eval.go scaffold)

**Files:**
- Create: `internal/core/runtime/eval.go`

- [ ] **Step 1: Define EvalContext and frame stack (eval.go)**

```go
package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
	ctx    *Context            // runtime context
	frames []map[string]Value  // stack of local bindings (innermost = frames[len-1])
}

// NewEvalContext creates an evaluation context with an empty frame stack.
func NewEvalContext(ctx *Context) *EvalContext {
	return &EvalContext{
		ctx:    ctx,
		frames: nil,
	}
}

// Push adds a new frame to the stack (on calc invocation, lambda entry).
func (ec *EvalContext) Push(bindings map[string]Value) {
	ec.frames = append(ec.frames, bindings)
}

// Pop removes the top frame from the stack (on return, lambda exit).
func (ec *EvalContext) Pop() {
	if len(ec.frames) > 0 {
		ec.frames = ec.frames[:len(ec.frames)-1]
	}
}

// Lookup searches for a name in the frame stack (innermost first).
func (ec *EvalContext) Lookup(name string) (Value, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if val, ok := ec.frames[i][name]; ok {
			return val, true
		}
	}
	return Value{}, false
}

// Eval evaluates an expression node. Returns a Value or an error.
// Increments ctx.steps on each eval call; errors when ctx.steps >= ctx.maxSteps.
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	// Step counter
	if err := ec.ctx.incrementStep(); err != nil {
		return Value{}, err
	}
	
	// Dispatch by node type (scaffolding; full implementation in later tasks)
	switch n := node.(type) {
	case *ast.LiteralInteger:
		return ec.evalLiteralInteger(n)
	case *ast.LiteralReal:
		return ec.evalLiteralReal(n)
	case *ast.LiteralBool:
		return ec.evalLiteralBool(n)
	default:
		return Value{}, fmt.Errorf("unsupported node type: %T", node)
	}
}

// Eval is the top-level entry point for evaluating an expression in an empty environment.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
	ec := NewEvalContext(ctx)
	return ec.Eval(node)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/core/runtime/`

Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/core/runtime/eval.go
git commit -m "feat(runtime): Tier 3 EvalContext + Eval scaffolding"
```

---

### Task 8: Tier 3 — Literal + Operator Evaluation (eval.go dispatch)

**Files:**
- Modify: `internal/core/runtime/eval.go`
- Create: `internal/core/runtime/eval_test.go`

- [ ] **Step 1: Write failing test for literals (eval_test.go)**

```go
package runtime

import (
	"testing"
)

func TestEval_Literals(t *testing.T) {
	tests := []struct {
		src      string
		expected Value
	}{
		{"42", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 42}}},
		{"3.14", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3.14}}},
		{"true", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}},
		{`"hello"`, Value{Kind: ValString, Str: "hello"}},
		{"null", Value{Kind: ValNull}},
	}
	
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, root := parseAndBuildModel(t, "calc test { return "+tt.src+"; }")
			ctx := NewContext(model, 1000)
			
			// Extract expression from calc body
			calcSym := resolveSymbol(t, root, "test")
			calcDecl := calcSym.Decl.(*ast.Usage)
			expr := calcDecl.Members[0] // simplified: assume single expression
			
			result, err := ctx.Eval(expr)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			
			if result.Kind != tt.expected.Kind {
				t.Errorf("expected Kind %v, got %v", tt.expected.Kind, result.Kind)
			}
		})
	}
}

func TestEval_Arithmetic(t *testing.T) {
	src := `calc test { return 1 + 2; }`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	calcSym := resolveSymbol(t, root, "test")
	calcDecl := calcSym.Decl.(*ast.Usage)
	expr := calcDecl.Members[0]
	
	result, err := ctx.Eval(expr)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	
	if result.Kind != ValConst || result.Const.Int != 3 {
		t.Errorf("expected 3, got %v", result)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core/runtime/... -run TestEval_Literals`

Expected: FAIL (literal eval not implemented)

- [ ] **Step 3: Implement literal evaluators (eval.go)**

```go
func (ec *EvalContext) evalLiteralInteger(n *ast.LiteralInteger) (Value, error) {
	val, _ := strconv.ParseInt(n.Value, 10, 64)
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
}

func (ec *EvalContext) evalLiteralReal(n *ast.LiteralReal) (Value, error) {
	val, _ := strconv.ParseFloat(n.Value, 64)
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: val}}, nil
}

func (ec *EvalContext) evalLiteralBool(n *ast.LiteralBool) (Value, error) {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: n.Value}}, nil
}

func (ec *EvalContext) evalLiteralString(n *ast.LiteralString) (Value, error) {
	// Strip quotes
	str := n.Value
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	return Value{Kind: ValString, Str: str}, nil
}

func (ec *EvalContext) evalNull(n *ast.NullExpr) (Value, error) {
	return Value{Kind: ValNull}, nil
}
```

- [ ] **Step 4: Add cases to Eval dispatch (eval.go)**

```go
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	if err := ec.ctx.incrementStep(); err != nil {
		return Value{}, err
	}
	
	switch n := node.(type) {
	case *ast.LiteralInteger:
		return ec.evalLiteralInteger(n)
	case *ast.LiteralReal:
		return ec.evalLiteralReal(n)
	case *ast.LiteralBool:
		return ec.evalLiteralBool(n)
	case *ast.LiteralString:
		return ec.evalLiteralString(n)
	case *ast.NullExpr:
		return ec.evalNull(n)
	case *ast.OperatorExpr:
		return ec.evalOperator(n)
	default:
		return Value{}, fmt.Errorf("unsupported node type: %T", node)
	}
}
```

- [ ] **Step 5: Implement operator evaluator (eval.go)**

```go
func (ec *EvalContext) evalOperator(n *ast.OperatorExpr) (Value, error) {
	// Try constant folding first
	if semVal, ok := ec.ctx.model.Eval(n); ok {
		return Value{Kind: ValConst, Const: semVal}, nil
	}
	
	// Otherwise, recursively eval operands
	switch n.Operator {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv:
		return ec.evalArithmetic(n)
	case ast.OpEq, ast.OpNe:
		return ec.evalEquality(n)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return ec.evalComparison(n)
	case ast.OpAnd, ast.OpOr:
		return ec.evalLogical(n)
	case ast.OpNeg:
		return ec.evalNeg(n)
	case ast.OpNot:
		return ec.evalNot(n)
	default:
		return Value{}, fmt.Errorf("unsupported operator: %v", n.Operator)
	}
}

func (ec *EvalContext) evalArithmetic(n *ast.OperatorExpr) (Value, error) {
	left, err := ec.Eval(n.Left)
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Right)
	if err != nil {
		return Value{}, err
	}
	
	// Simplified: assume both are ValConst int/real
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, ErrTypeMismatch
	}
	
	// Integer arithmetic
	if left.Const.Kind == semantics.ValInt && right.Const.Kind == semantics.ValInt {
		var result int64
		switch n.Operator {
		case ast.OpAdd:
			result = left.Const.Int + right.Const.Int
		case ast.OpSub:
			result = left.Const.Int - right.Const.Int
		case ast.OpMul:
			result = left.Const.Int * right.Const.Int
		case ast.OpDiv:
			if right.Const.Int == 0 {
				return Value{}, fmt.Errorf("division by zero")
			}
			result = left.Const.Int / right.Const.Int
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: result}}, nil
	}
	
	// Real arithmetic (coerce int to real if needed)
	leftReal := toReal(left.Const)
	rightReal := toReal(right.Const)
	var result float64
	switch n.Operator {
	case ast.OpAdd:
		result = leftReal + rightReal
	case ast.OpSub:
		result = leftReal - rightReal
	case ast.OpMul:
		result = leftReal * rightReal
	case ast.OpDiv:
		result = leftReal / rightReal
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: result}}, nil
}

func toReal(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// Stub evalEquality, evalComparison, evalLogical, evalNeg, evalNot (full impl in integration)
func (ec *EvalContext) evalEquality(n *ast.OperatorExpr) (Value, error) {
	// Simplified placeholder
	return Value{}, fmt.Errorf("equality not yet implemented")
}

func (ec *EvalContext) evalComparison(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("comparison not yet implemented")
}

func (ec *EvalContext) evalLogical(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("logical not yet implemented")
}

func (ec *EvalContext) evalNeg(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("negation not yet implemented")
}

func (ec *EvalContext) evalNot(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("not not yet implemented")
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test -v ./internal/core/runtime/... -run TestEval_Literals`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/core/runtime/eval.go internal/core/runtime/eval_test.go
git commit -m "feat(runtime): Tier 3 literal + arithmetic evaluation"
```

---

### Task 9: Tier 3 — Feature Access (FeatureReference + FeatureChainExpr)

**Files:** Modify `eval.go`, add tests to `eval_test.go`

Plan complete and saved to `docs/superpowers/plans/2026-07-30-sysml-v2-evaluation-runtime.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**

### Task 9: Tier 3 — Feature Access (FeatureReference + FeatureChainExpr)

**Files:** Modify `eval.go`, add tests to `eval_test.go`

- [ ] **Step 1: Write test for FeatureReference frame lookup**

Add to `eval_test.go`:
```go
func TestEval_FeatureReference(t *testing.T) {
	model := &semantics.Model{}
	ctx := NewContext(model, 1000)
	ec := NewEvalContext(ctx)
	
	// Push frame with binding
	ec.Push(map[string]Value{
		"x": Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 42}},
	})
	
	// Create FeatureReference node for "x"
	ref := &ast.FeatureReference{Name: &ast.QualifiedName{Names: []string{"x"}}}
	
	result, err := ec.Eval(ref)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	
	if result.Const.Int != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}
```

- [ ] **Step 2: Run test → verify FAIL**

Run: `go test -v ./internal/core/runtime/... -run TestEval_FeatureReference`

- [ ] **Step 3: Implement FeatureReference eval**

Add to `eval.go` Eval dispatch:
```go
case *ast.FeatureReference:
	return ec.evalFeatureReference(n)
```

Add method:
```go
func (ec *EvalContext) evalFeatureReference(n *ast.FeatureReference) (Value, error) {
	name := n.Name.String()
	
	// Check frame stack
	if val, ok := ec.Lookup(name); ok {
		return val, nil
	}
	
	return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
}
```

- [ ] **Step 4: Run test → verify PASS**

- [ ] **Step 5: Write test for FeatureChainExpr**

Add to `eval_test.go`:
```go
func TestEval_FeatureChainExpr(t *testing.T) {
	src := `
		part def B { attribute val: Integer = 10; }
		part def A { part b: B; }
	`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	aSym := resolveSymbol(t, root, "A")
	instA, _ := ctx.Instantiate(aSym)
	
	// Create chain expr: instA.b.val
	ec := NewEvalContext(ctx)
	ec.Push(map[string]Value{
		"self": Value{Kind: ValInstance, Instance: instA.ID},
	})
	
	// Build FeatureChainExpr AST node (or use parser)
	// For now, test via Integration tests (Task 15)
}
```

- [ ] **Step 6: Implement FeatureChainExpr eval**

Add to `eval.go`:
```go
case *ast.FeatureChainExpr:
	return ec.evalFeatureChainExpr(n)
```

Add method:
```go
func (ec *EvalContext) evalFeatureChainExpr(n *ast.FeatureChainExpr) (Value, error) {
	// Eval operand (must be ValInstance)
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	if operand.Kind != ValInstance {
		return Value{}, fmt.Errorf("%w: feature chain operand must be instance", ErrTypeMismatch)
	}
	
	// Get instance
	inst, ok := ec.ctx.getInstance(operand.Instance)
	if !ok {
		return Value{}, fmt.Errorf("instance %d not found", operand.Instance)
	}
	
	// Get slot (triggers lazy materialization)
	memberName := n.Member.String()
	slot, err := inst.GetSlot(ec.ctx, memberName)
	if err != nil {
		return Value{}, err
	}
	
	// Return slot value
	if slot.Feature.Multiplicity.Upper.Value <= 1 {
		return slot.Value, nil
	}
	return slot.Values, nil
}
```

- [ ] **Step 7: Run tests → verify PASS**

- [ ] **Step 8: Commit**

```bash
git add internal/core/runtime/eval.go internal/core/runtime/eval_test.go
git commit -m "feat(runtime): Tier 3 feature access (FeatureReference + FeatureChainExpr)"
```

---

### Task 10: Tier 3 — Sequence + Collection Expressions

**Files:** Modify `eval.go`, add tests to `eval_test.go`

- [ ] **Step 1: Write test for SequenceExpr**

```go
func TestEval_SequenceExpr(t *testing.T) {
	src := `calc test { return (1, 2, 3); }`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	calcSym := resolveSymbol(t, root, "test")
	calcDecl := calcSym.Decl.(*ast.Usage)
	expr := calcDecl.Members[0].(*ast.SequenceExpr)
	
	result, err := ctx.Eval(expr)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	
	if result.Kind != ValSequence || result.Sequence.Size() != 3 {
		t.Errorf("expected Sequence size 3, got %v", result)
	}
}
```

- [ ] **Step 2: Run test → FAIL**

- [ ] **Step 3: Implement SequenceExpr eval**

```go
case *ast.SequenceExpr:
	return ec.evalSequenceExpr(n)
```

```go
func (ec *EvalContext) evalSequenceExpr(n *ast.SequenceExpr) (Value, error) {
	seq := NewSequence()
	for _, elem := range n.Elements {
		val, err := ec.Eval(elem)
		if err != nil {
			return Value{}, err
		}
		seq.Append(val)
	}
	return Value{Kind: ValSequence, Sequence: seq}, nil
}
```

- [ ] **Step 4: Run test → PASS**

- [ ] **Step 5: Write test for CollectExpr**

```go
func TestEval_CollectExpr(t *testing.T) {
	// Build Sequence{1, 2, 3}, eval collect {in x; x * 2}
	// Expected: Sequence{2, 4, 6}
	// (Defer to integration tests for full syntax)
}
```

- [ ] **Step 6: Implement CollectExpr + SelectExpr stubs**

```go
case *ast.CollectExpr:
	return ec.evalCollectExpr(n)
case *ast.SelectExpr:
	return ec.evalSelectExpr(n)
```

```go
func (ec *EvalContext) evalCollectExpr(n *ast.CollectExpr) (Value, error) {
	// Eval operand → must be collection
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	var elements []Value
	switch operand.Kind {
	case ValSequence:
		elements = operand.Sequence.Elements()
	case ValSet:
		elements = operand.Set.Elements()
	default:
		return Value{}, fmt.Errorf("%w: collect operand must be collection", ErrTypeMismatch)
	}
	
	// Iterate, eval body for each element
	result := NewSequence()
	for _, elem := range elements {
		ec.Push(map[string]Value{"it": elem})
		val, err := ec.Eval(n.Body)
		ec.Pop()
		if err != nil {
			return Value{}, err
		}
		result.Append(val)
	}
	
	return Value{Kind: ValSequence, Sequence: result}, nil
}

func (ec *EvalContext) evalSelectExpr(n *ast.SelectExpr) (Value, error) {
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	var elements []Value
	switch operand.Kind {
	case ValSequence:
		elements = operand.Sequence.Elements()
	case ValSet:
		elements = operand.Set.Elements()
	default:
		return Value{}, ErrTypeMismatch
	}
	
	result := NewSequence()
	for _, elem := range elements {
		ec.Push(map[string]Value{"it": elem})
		predVal, err := ec.Eval(n.Predicate)
		ec.Pop()
		if err != nil {
			return Value{}, err
		}
		
		if predVal.Kind == ValConst && predVal.Const.Kind == semantics.ValBool && predVal.Const.Bool {
			result.Append(elem)
		}
	}
	
	return Value{Kind: ValSequence, Sequence: result}, nil
}
```

- [ ] **Step 7: Run tests → PASS**

- [ ] **Step 8: Commit**

```bash
git add internal/core/runtime/eval.go internal/core/runtime/eval_test.go
git commit -m "feat(runtime): Tier 3 SequenceExpr + CollectExpr/SelectExpr"
```

---

### Task 11: KerML Builtins — Registry + SequenceFunctions

**Files:** Create `builtins.go`, `builtins_test.go`

- [ ] **Step 1: Write test for SequenceFunctions::size**

```go
package runtime

import "testing"

func TestBuiltin_SequenceSize(t *testing.T) {
	seq := NewSequence()
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}})
	
	args := []Value{{Kind: ValSequence, Sequence: seq}}
	
	fn := builtins["SequenceFunctions::size"]
	result, err := fn(nil, args)
	if err != nil {
		t.Fatalf("size failed: %v", err)
	}
	
	if result.Const.Int != 2 {
		t.Errorf("expected size 2, got %d", result.Const.Int)
	}
}
```

- [ ] **Step 2: Run test → FAIL**

- [ ] **Step 3: Implement builtin registry (builtins.go)**

```go
package runtime

import (
	"errors"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

var builtins = map[string]func(*EvalContext, []Value) (Value, error){
	"SequenceFunctions::size":     builtinSequenceSize,
	"SequenceFunctions::isEmpty":  builtinSequenceIsEmpty,
	"SequenceFunctions::includes": builtinSequenceIncludes,
	"CollectionFunctions::size":   builtinCollectionSize,
	"CollectionFunctions::isEmpty": builtinCollectionIsEmpty,
}

func builtinSequenceSize(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, errors.New("SequenceFunctions::size: expected 1 argument")
	}
	
	col := args[0]
	var sz int64
	switch col.Kind {
	case ValSequence:
		sz = int64(col.Sequence.Size())
	case ValSet:
		sz = int64(col.Set.Size())
	default:
		return Value{}, errors.New("SequenceFunctions::size: expected collection")
	}
	
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: sz}}, nil
}

func builtinSequenceIsEmpty(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, errors.New("SequenceFunctions::isEmpty: expected 1 argument")
	}
	
	sizeVal, err := builtinSequenceSize(ec, args)
	if err != nil {
		return Value{}, err
	}
	
	isEmpty := sizeVal.Const.Int == 0
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: isEmpty}}, nil
}

func builtinSequenceIncludes(ec *EvalContext, args []Value) (Value, error) {
	// Stub: check if seq1 includes seq2 (all elements of seq2 in seq1)
	return Value{}, errors.New("SequenceFunctions::includes: not yet implemented")
}

func builtinCollectionSize(ec *EvalContext, args []Value) (Value, error) {
	return builtinSequenceSize(ec, args)
}

func builtinCollectionIsEmpty(ec *EvalContext, args []Value) (Value, error) {
	return builtinSequenceIsEmpty(ec, args)
}
```

- [ ] **Step 4: Run tests → PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/core/runtime/builtins.go internal/core/runtime/builtins_test.go
git commit -m "feat(runtime): KerML builtins registry + SequenceFunctions"
```

---

### Task 12: KerML Builtins — ControlFunctions (select, collect)

**Files:** Modify `builtins.go`, add tests to `builtins_test.go`

- [ ] **Step 1: Write test for ControlFunctions::select**

```go
func TestBuiltin_ControlSelect(t *testing.T) {
	seq := NewSequence()
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}})
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}})
	
	// Predicate: x > 1
	// (For unit test, pass lambda AST node or stub)
	// Defer full test to integration
}
```

- [ ] **Step 2: Implement ControlFunctions::select/collect**

```go
var builtins = map[string]func(*EvalContext, []Value) (Value, error){
	// ... existing
	"ControlFunctions::select":  builtinControlSelect,
	"ControlFunctions::collect": builtinControlCollect,
}

func builtinControlSelect(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, errors.New("ControlFunctions::select: expected 2 arguments")
	}
	
	col := args[0]
	pred := args[1] // lambda body (AST node wrapped in Value? TBD)
	
	// Implementation mirrors evalSelectExpr
	// (Full impl in integration; stub for now)
	return Value{}, errors.New("ControlFunctions::select: not yet implemented")
}

func builtinControlCollect(ec *EvalContext, args []Value) (Value, error) {
	return Value{}, errors.New("ControlFunctions::collect: not yet implemented")
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/core/runtime/builtins.go internal/core/runtime/builtins_test.go
git commit -m "feat(runtime): KerML ControlFunctions stubs (select/collect)"
```

---

### Task 13: Tier 3 — Invocation (calc + builtin dispatch)

**Files:** Modify `eval.go`, add tests to `eval_test.go`

- [ ] **Step 1: Write test for calc invocation**

```go
func TestEval_CalcInvocation(t *testing.T) {
	src := `
		calc add { in x: Integer; in y: Integer; return x + y; }
	`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 1000)
	
	// Build InvocationExpr for add(3, 5)
	// (Defer to integration for full syntax parse)
}
```

- [ ] **Step 2: Implement InvocationExpr eval**

```go
case *ast.InvocationExpr:
	return ec.evalInvocation(n)
```

```go
func (ec *EvalContext) evalInvocation(n *ast.InvocationExpr) (Value, error) {
	// Resolve target
	targetSym, ok := ec.ctx.model.Resolver().ResolveQualified(scope, n.Type)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, n.Type)
	}
	
	// Get qualified name
	qualName := symbolQualifiedName(targetSym)
	
	// Eval args
	args := make([]Value, len(n.Args))
	for i, arg := range n.Args {
		val, err := ec.Eval(arg)
		if err != nil {
			return Value{}, err
		}
		args[i] = val
	}
	
	// Check builtin registry
	if fn, ok := builtins[qualName]; ok {
		return fn(ec, args)
	}
	
	// User-defined calc: push frame, eval body
	return ec.evalCalc(targetSym, args)
}

func (ec *EvalContext) evalCalc(calcSym *symbols.Symbol, args []Value) (Value, error) {
	// Extract calc body
	calcDecl, ok := calcSym.Decl.(*ast.Usage)
	if !ok {
		return Value{}, errors.New("calc target is not a usage")
	}
	
	// Extract params (scan Members for param usages)
	// Simplified: assume positional binding
	bindings := make(map[string]Value)
	// TODO: extract param names from calcDecl.Members
	// For now, stub with arg0, arg1, ...
	for i, arg := range args {
		bindings[fmt.Sprintf("arg%d", i)] = arg
	}
	
	// Push frame
	ec.Push(bindings)
	defer ec.Pop()
	
	// Eval body (extract result expression via extractCalcResult helper)
	resultExpr := extractCalcResult(calcDecl.Members)
	return ec.Eval(resultExpr)
}

func extractCalcResult(members []ast.Node) ast.Node {
	// Hybrid: look for 'result' usage, else last expression
	for _, m := range members {
		if usage, ok := m.(*ast.Usage); ok && usage.Name == "result" {
			return usage.Value
		}
	}
	
	// Last expression
	if len(members) > 0 {
		return members[len(members)-1]
	}
	
	return nil
}

func symbolQualifiedName(sym *symbols.Symbol) string {
	// Build qualified name by walking owner chain
	parts := []string{sym.Name}
	for owner := sym.OwnerScope; owner != nil && owner.Node() != nil; owner = owner.Parent() {
		if ownerSym := owner.Node().(*symbols.Symbol); ownerSym != nil {
			parts = append([]string{ownerSym.Name}, parts...)
		}
	}
	return strings.Join(parts, "::")
}
```

- [ ] **Step 3: Run tests → PASS**

- [ ] **Step 4: Commit**

```bash
git add internal/core/runtime/eval.go internal/core/runtime/eval_test.go
git commit -m "feat(runtime): Tier 3 InvocationExpr (calc + builtin dispatch)"
```

---

### Task 14: Step Counter + Runaway Guard

**Files:** Modify `eval.go`, add tests to `eval_test.go`

- [ ] **Step 1: Write test for step limit**

```go
func TestEval_StepLimit(t *testing.T) {
	src := `
		calc infinite { return infinite(); }
	`
	model, root := parseAndBuildModel(t, src)
	ctx := NewContext(model, 10) // low limit
	
	infiniteSym := resolveSymbol(t, root, "infinite")
	_, err := ctx.Eval(infiniteSym.Decl)
	
	if err == nil || !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got %v", err)
	}
}
```

- [ ] **Step 2: Run test → verify step counter triggers**

Run: `go test -v ./internal/core/runtime/... -run TestEval_StepLimit`

Expected: PASS (step counter already implemented in Context)

- [ ] **Step 3: Commit**

```bash
git add internal/core/runtime/eval_test.go
git commit -m "test(runtime): verify step counter runaway guard"
```

---

### Task 15: Integration Tests

**Files:** Create `runtime_integration_test.go`, testdata fixtures

- [ ] **Step 1: Create testdata directory**

```bash
mkdir -p internal/core/runtime/testdata
```

- [ ] **Step 2: Write fixture: simple_calc.sysml**

```sysml
package test {
	calc add {
		in x: Integer;
		in y: Integer;
		return x + y;
	}
	
	part def Result {
		attribute sum: Integer = add(3, 5);
	}
}
```

- [ ] **Step 3: Write integration test**

```go
package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIntegration_SimpleCalc(t *testing.T) {
	src, _ := os.ReadFile(filepath.Join("testdata", "simple_calc.sysml"))
	
	model, root := parseAndBuildModel(t, string(src))
	ctx := NewContext(model, 100000)
	
	// Instantiate Result
	resultSym := resolveSymbol(t, root, "Result")
	inst, err := ctx.Instantiate(resultSym)
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	
	// Check sum slot value
	sumSlot, _ := inst.GetSlot(ctx, "sum")
	if sumSlot.Value.Const.Int != 8 {
		t.Errorf("expected sum=8, got %d", sumSlot.Value.Const.Int)
	}
}
```

- [ ] **Step 4: Run integration tests**

Run: `go test -v ./internal/core/runtime/... -run TestIntegration`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/core/runtime/runtime_integration_test.go internal/core/runtime/testdata/
git commit -m "test(runtime): end-to-end integration tests"
```

---

### Task 16: Package Documentation

**Files:** Modify `doc.go`

- [ ] **Step 1: Expand package doc with usage examples**

```go
// Package runtime provides the SysML v2 execution runtime: expression
// evaluation, instance materialization, and KerML operator library.
//
// # Usage Example
//
//	// Parse and validate model
//	model := semantics.NewModel(resolver)
//	
//	// Create runtime context
//	ctx := runtime.NewContext(model, 100000)
//	
//	// Instantiate a part
//	partSym := resolveSymbol(root, "MyCar")
//	inst, err := ctx.Instantiate(partSym)
//	if err != nil {
//		log.Fatal(err)
//	}
//	
//	// Evaluate an expression
//	exprNode := parseExpression("1 + 2")
//	result, err := ctx.Eval(exprNode)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(result.Const.Int) // 3
//
// # Architecture
//
// The runtime is organized in three tiers:
//
//   - Tier 1: Feature flattening (effective-feature lists per type)
//   - Tier 2: Instance model (lazy slot materialization, multiplicity-driven collections)
//   - Tier 3: Expression evaluator (literals, operators, feature access, calc invocation, KerML builtins)
//
// # Integration
//
//   - Consumes semantics.Model (inherits features, multiplicity, constant folding)
//   - Gates on pass-validated models (LevelConstraint success)
//   - One Context per workspace session (LSP/REPL lifetime)
//
// Behavioral simulation (actions, state machines) is out of scope (future Tiers 4–5).
package runtime
```

- [ ] **Step 2: Run gofmt**

Run: `gofmt -w internal/core/runtime/doc.go`

- [ ] **Step 3: Verify documentation**

Run: `go doc github.com/Open-MBEE/Systemica/internal/core/runtime`

Expected: Package doc renders correctly

- [ ] **Step 4: Commit**

```bash
git add internal/core/runtime/doc.go
git commit -m "docs(runtime): expand package documentation with examples"
```

---


## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-30-sysml-v2-evaluation-runtime.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
