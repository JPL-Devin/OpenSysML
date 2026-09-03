// Exercised from outside the package: resolving a chain or an inherited member
// needs the semantic model, which imports resolve.
package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// referencesModel writes the declaration forms whose references an editor has to
// find, from imports and aliases through chains, ends, transitions and states.
const referencesModel = `package Lib {
	attribute def Torque;
	part def Wheel;
	part def Engine {
		attribute rate : Torque;
		action generateTorque;
	}
	item def Fuel;
	port def FuelPort {
		out item fuel : Fuel;
	}
}
package App {
	private import Lib::*;
	alias Motor for Lib::Engine;
	dependency App::Car to Lib::Wheel;
	comment about App::Car /* the car */

	part def Car {
		part engine : Engine;
		part wheel : Wheel[4];
		attribute peak : Torque = engine.rate;
		port intake : FuelPort;
		perform engine.generateTorque;
	}
	part def Racecar :> Car {
		attribute :>> peak = 1;
		part :>> engine : Engine;
	}
	connection def Feed {
		end part driver : Car;
		end part driven : Car;
	}
	part car : Car;
	connection feed : Feed connect car.engine to car.wheel;
	flow of Fuel from car.intake.fuel to car.intake.fuel;
	action def Drive {
		in attribute speed : Torque;
		first start;
		then action step {
			assign speed := speed + 1;
		}
		if speed > 1 { action fast; }
		while speed > 1 { action loop; }
	}
	state def Trip {
		entry action begin;
		state running;
		state stopped;
		transition running_to_stopped first running then stopped;
	}
	constraint def Positive {
		in attribute limit : Torque;
		limit > 0
	}
	requirement def Fast {
		subject vehicle : Car;
		assume constraint { vehicle.peak > 0 }
		require Positive;
	}
}`

// A form the collector misses is a name the editor can neither navigate nor
// rename, so every collected reference must resolve on its own to what it named.
func TestEveryCollectedReferenceResolvesOnItsOwn(t *testing.T) {
	walk, root, rootScope := resolvedDoc(t, referencesModel)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}

	refs := resolve.References(root, rootScope)
	if len(refs) == 0 {
		t.Fatal("References found no name in a document full of them")
	}
	query, _, _ := resolvedDoc(t, referencesModel) // a fresh resolver, as the editor's is
	for _, ref := range refs {
		sym, ok := query.ResolveReference(ref)
		if !ok {
			t.Errorf("%s at offset %d does not resolve on its own, though the document walk resolved it",
				nameText(ref.QN), ref.QN.Span().Offset)
			continue
		}
		if walked, ok := walk.PartSymbol(ref.QN, len(ref.QN.Parts)-1); ok && walked.Name != sym.Name {
			t.Errorf("%s resolves to %q on its own but to %q in the document walk",
				nameText(ref.QN), sym.Name, walked.Name)
		}
	}
}

// The kind of an occurrence decides where it resolves — a redefinition in the
// generals, a chain member in the preceding segment — so it has to be tagged.
func TestCollectedReferencesCarryTheirResolutionKind(t *testing.T) {
	_, root, rootScope := resolvedDoc(t, referencesModel)
	byName := map[string][]resolve.Reference{}
	for _, ref := range resolve.References(root, rootScope) {
		byName[nameText(ref.QN)] = append(byName[nameText(ref.QN)], ref)
	}

	// `attribute :>> peak` redefines an inherited feature; `vehicle.peak` names
	// a member of the subject's type.
	redefines, chained := 0, 0
	for _, ref := range byName["peak"] {
		if ref.Redefines {
			redefines++
		}
		if ref.Chain != nil {
			chained++
		}
	}
	if redefines != 1 || chained != 1 {
		t.Errorf("`peak` yields %d redefinitions and %d chain members, want 1 and 1", redefines, chained)
	}
	if refs := byName["engine"]; len(refs) != 4 {
		// Not the declaration `part engine : Engine`: the redefinition in Racecar,
		// `engine.rate`, `perform engine.…` and `connect car.engine`.
		t.Errorf("`engine` appears %d times, want 4: %v", len(refs), kindsOf(refs))
	}
	// A chain's member segment carries the chain so it is looked up in the
	// operand rather than in the enclosing scope.
	for _, name := range []string{"rate", "generateTorque"} {
		refs := byName[name]
		if len(refs) != 1 || refs[0].Chain == nil {
			t.Errorf("`%s` = %v, want one reference tagged as a chain member", name, kindsOf(refs))
		}
	}
	// `perform engine.generateTorque` declares no name, so the performing usage is
	// the referrer: its borrowed binding must not shadow what the chain names.
	for _, ref := range byName["generateTorque"] {
		if ref.Referrer == nil {
			t.Error("`perform engine.generateTorque` records no referrer, so the target could resolve to the borrowed name")
		}
	}
}

// A filter condition's names are not restricted by the namespace's own filters,
// so the collector marks them and resolution keeps them unfiltered.
func TestAFilterConditionsOwnNamesAreMarked(t *testing.T) {
	const src = `package Meta {
	metadata def Safety;
}
package App {
	private import Meta::*;
	filter @Safety;
	part def Belt;
}`
	r, root, rootScope := resolvedDoc(t, src)
	var conditions, plain int
	for _, ref := range resolve.References(root, rootScope) {
		if ref.Condition {
			conditions++
			if _, ok := r.ResolveReference(ref); !ok {
				t.Errorf("the condition's own name %s must resolve unfiltered", nameText(ref.QN))
			}
			continue
		}
		plain++
	}
	if conditions != 1 {
		t.Errorf("%d references marked as a filter condition's own name, want 1 (@Safety)", conditions)
	}
	if plain == 0 {
		t.Error("the import's own name must be reported as an ordinary reference")
	}
}

// A send's receiver (`via p to ground`) is a name like its payload and port; a
// constructor's argument label (`new T(frames = …)`) names a feature of T — its
// own or inherited — so it is collected as one and resolves to T's feature, never
// to a same-named feature in the sender's scope.
func TestSendReceiverAndConstructorLabelsAreReferences(t *testing.T) {
	const src = `package App {
	item def Telemetry { attribute frames; }
	item def Burst :> Telemetry { attribute rate; }
	port def Radio;
	part def Station;
	action def Downlink {
		port antenna : Radio;
		part ground : Station;
		attribute frames;
		send new Telemetry(frames = 3) via antenna to ground;
		send new Burst(frames = 4, rate = 2) via antenna to missing;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 1 || !strings.Contains(walk.Diagnostics[0].Message, "missing") {
		t.Fatalf("the unresolved receiver `missing` must be the one diagnostic, got %v", walk.Diagnostics)
	}
	pkg := unwrapMember(root.Members[0])
	telemetry := rootScope.ChildFor(pkg).ChildFor(memberOf(pkg, 0))
	want, ok := telemetry.LookupLocal("frames")
	if !ok || want == nil {
		t.Fatal("Telemetry's `attribute frames` was not indexed")
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "frames" && name != "rate" {
			continue
		}
		if ref.Constructed == nil {
			t.Errorf("label `%s` at %v is not tagged as a constructor argument", name, ref.QN.Span())
		}
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Fatalf("label `%s` at %v did not resolve", name, ref.QN.Span())
		}
		if name == "frames" && sym != want {
			t.Errorf("label `frames` at %v resolved to %s's feature, want Telemetry's", ref.QN.Span(), sym.Name)
		}
	}
	if byName["ground"] != 1 || byName["missing"] != 1 {
		t.Errorf("receivers collected: ground=%d missing=%d, want 1 and 1", byName["ground"], byName["missing"])
	}
	if byName["frames"] != 2 || byName["rate"] != 1 {
		t.Errorf("labels collected: frames=%d rate=%d, want 2 and 1", byName["frames"], byName["rate"])
	}
}

// A label naming no feature of the constructed type is unresolved where it is
// written, though the sender's scope declares that name.
func TestConstructorLabelMustNameAFeatureOfTheConstructedType(t *testing.T) {
	const src = `package App {
	item def Telemetry { attribute frames; }
	part def Station;
	action def Downlink {
		part ground : Station;
		attribute count;
		send new Telemetry(count = 3) to ground;
	}
}`
	walk, _, _ := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 1 || !strings.Contains(walk.Diagnostics[0].Message, "count") {
		t.Fatalf("the label `count` must be the one unresolved name, got %v", walk.Diagnostics)
	}
	if got, want := walk.Diagnostics[0].Span.Offset, strings.LastIndex(src, "count = 3"); got != want {
		t.Errorf("reported at offset %d, want the label at %d", got, want)
	}
}

// A transition's guard and effect are collected in the scope holding the
// parameter its trigger declares, as the document walk resolves them, so a send
// whose receiver is that parameter reaches it rather than a same-named feature
// of the machine — also on a fresh resolver, as rename uses, with nothing memoized
// from a document walk.
func TestTransitionEffectReferencesResolveInTriggerScope(t *testing.T) {
	const src = `package App {
	item def Request;
	state def Server {
		part origin : Request;
		state idle;
		state busy;
		transition first idle accept origin : Request if origin != null do send new Request() to origin then busy;
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", walk.Diagnostics)
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	pkg := unwrapMember(root.Members[0])
	part, ok := rootScope.ChildFor(pkg).ChildFor(memberOf(pkg, 1)).LookupLocal("origin")
	if !ok || part == nil {
		t.Fatal("the machine's `part origin` was not indexed")
	}
	uses := 0
	for _, ref := range resolve.References(root, rootScope) {
		if nameText(ref.QN) != "origin" {
			continue
		}
		uses++
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Fatalf("`origin` at %v did not resolve", ref.QN.Span())
		}
		if sym == part {
			t.Errorf("`origin` at %v reached the machine's part, want the accept's parameter", ref.QN.Span())
		}
	}
	if uses != 2 {
		t.Errorf("collected %d `origin` reference(s) (guard and receiver), want 2", uses)
	}
}

// A transition's trailing body (`then busy { … }`) resolves and is collected like
// its effect: in the trigger's scope, so its sends reach the accept's parameter
// and an unresolved receiver or constructor label in it is reported.
func TestTransitionBodyResolvesInTriggerScope(t *testing.T) {
	const src = `package App {
	item def Request { attribute id; }
	state def Server {
		state idle;
		state busy;
		transition t first idle accept origin : Request then busy {
			action log;
			send new Request(id = 1) to origin;
			send new Request(count = 1) to missing;
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 2 {
		t.Fatalf("the label `count` and the receiver `missing` must be the two unresolved names, got %v", walk.Diagnostics)
	}
	for _, d := range walk.Diagnostics {
		if !strings.Contains(d.Message, "count") && !strings.Contains(d.Message, "missing") {
			t.Errorf("unexpected diagnostic %v", d)
		}
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "origin" && name != "id" {
			continue
		}
		if _, ok := cold.ResolveReference(ref); !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
	}
	if byName["origin"] != 1 || byName["id"] != 1 || byName["count"] != 1 || byName["missing"] != 1 {
		t.Errorf("body references collected: %v, want origin, id, count and missing once each", byName)
	}
	pkg := unwrapMember(root.Members[0])
	trans, ok := rootScope.ChildFor(pkg).ChildFor(memberOf(pkg, 1)).LookupLocal("t")
	if !ok || trans == nil || trans.Scope == nil {
		t.Fatal("the transition `t` was not indexed with a scope")
	}
	if _, ok := trans.Scope.LookupLocal("log"); !ok {
		t.Error("the body's `action log` is not a member of the transition")
	}
}

// An unnamed transition's trailing body declares into a scope of its own, nested
// under the state and holding the trigger's parameters below it: a feature it
// declares is seen by a later member, and its sends reach the accept's parameter.
func TestUnnamedTransitionBodyDeclaresItsOwnScope(t *testing.T) {
	const src = `package App {
	item def Request { attribute id; }
	state def Server {
		attribute retries;
		state idle;
		state busy;
		transition first idle accept origin : Request then busy {
			attribute retries;
			send new Request(id = retries) to origin;
			send new Request(count = 1) to missing;
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 2 {
		t.Fatalf("the label `count` and the receiver `missing` must be the two unresolved names, got %v", walk.Diagnostics)
	}
	pkg := unwrapMember(root.Members[0])
	server := memberOf(pkg, 1)
	serverScope := rootScope.ChildFor(pkg).ChildFor(server)
	trans, ok := memberOf(server, 3).(*ast.TransitionMember)
	if !ok {
		t.Fatalf("the transition is a %T", memberOf(server, 3))
	}
	body := serverScope.ChildFor(trans)
	if body == nil {
		t.Fatal("the unnamed transition owns no scope")
	}
	local, ok := body.LookupLocal("retries")
	if !ok || local.Decl == memberOf(server, 0) {
		t.Fatal("the body's `attribute retries` is not a member of the transition's scope")
	}
	if _, ok := serverScope.LookupLocal("retries"); !ok {
		t.Fatal("the state's own `attribute retries` was not indexed")
	}
	trigger := symbols.TriggerScope(serverScope, trans)
	if trigger == nil || trigger.Parent() != body {
		t.Fatalf("TriggerScope is %v, want the parameter scope nested in the transition's", trigger)
	}
	if _, ok := trigger.LookupLocal("origin"); !ok {
		t.Error("the accept's parameter is not declared in the trigger scope")
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "retries" && name != "origin" && name != "id" {
			continue
		}
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
		if name == "retries" && sym != local {
			t.Errorf("`retries` at %v reached %v, want the body's own declaration", ref.QN.Span(), sym)
		}
	}
	if byName["retries"] != 1 || byName["origin"] != 1 || byName["id"] != 1 || byName["count"] != 1 || byName["missing"] != 1 {
		t.Errorf("body references collected: %v, want retries, origin, id, count and missing once each", byName)
	}
}

// A control node's body — an initial node's, a named fork's, an unnamed decision's —
// declares into a scope of the node's own and is resolved and collected there, so
// a constructor's unknown type or label in it is reported like one anywhere else.
func TestControlNodeBodiesResolveInTheirOwnScope(t *testing.T) {
	const src = `package App {
	item def Request { attribute id; }
	action def Flow {
		attribute retries;
		action a;
		action b;
		first start then a {
			attribute retries;
			send new Request(id = retries) to a;
			send new Missing() to a;
		}
		fork f {
			attribute retries;
			send new Request(nope = retries) to b;
		}
		decide {
			attribute retries;
			send new Request(id = retries) to b;
			send new Request(id = 1) to gone;
		}
	}
}`
	walk, root, rootScope := resolvedDoc(t, src)
	if len(walk.Diagnostics) != 3 {
		t.Fatalf("the type `Missing`, the label `nope` and the receiver `gone` must be the three unresolved names, got %v", walk.Diagnostics)
	}
	pkg := unwrapMember(root.Members[0])
	flow := memberOf(pkg, 1)
	flowScope := rootScope.ChildFor(pkg).ChildFor(flow)
	outer, ok := flowScope.LookupLocal("retries")
	if !ok {
		t.Fatal("the action's own `attribute retries` was not indexed")
	}
	locals := map[*symbols.Symbol]bool{}
	for i := 3; i <= 5; i++ {
		node := memberOf(flow, i)
		body := flowScope.ChildFor(node)
		if body == nil {
			t.Fatalf("the %T owns no scope", node)
		}
		local, ok := body.LookupLocal("retries")
		if !ok || local == outer {
			t.Fatalf("the %T body's `attribute retries` is not a member of the node's scope", node)
		}
		locals[local] = true
	}
	cold := resolve.New(walk.Index())
	cold.SetModel(semantics.NewModel(cold))
	byName := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		name := nameText(ref.QN)
		byName[name]++
		if name != "retries" && name != "id" {
			continue
		}
		sym, ok := cold.ResolveReference(ref)
		if !ok {
			t.Errorf("`%s` at %v did not resolve", name, ref.QN.Span())
		}
		if name == "retries" && !locals[sym] {
			t.Errorf("`retries` at %v reached %v, want the body's own declaration", ref.QN.Span(), sym)
		}
	}
	if byName["retries"] != 3 || byName["id"] != 3 || byName["Missing"] != 1 || byName["nope"] != 1 || byName["gone"] != 1 {
		t.Errorf("body references collected: %v, want retries and id thrice, Missing, nope and gone once", byName)
	}
}

// memberOf returns the i-th member declaration of a package or definition node.
func memberOf(n ast.Node, i int) ast.Node {
	switch v := n.(type) {
	case *ast.Package:
		return unwrapMember(v.Members[i])
	case *ast.Definition:
		return unwrapMember(v.Members[i])
	}
	return nil
}

func unwrapMember(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

// resolvedDoc parses and resolves src the way the workspace does, with the model
// attached before the walk.
func resolvedDoc(t *testing.T, src string) (*resolve.Resolver, *ast.RootNamespace, *symbols.Scope) {
	return resolvedDocNamed(t, "app.sysml", src)
}

func resolvedDocNamed(t *testing.T, name, src string) (*resolve.Resolver, *ast.RootNamespace, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument(name, root)
	return r, root, idx.DocumentRoot(name)
}

// nameText renders a qualified name the way it was written.
func nameText(q *ast.QualifiedName) string {
	out := ""
	for i, p := range q.Parts {
		if i > 0 {
			out += "::"
		}
		out += p.Text
	}
	return out
}

// kindsOf describes references by the tags that decide where they resolve.
func kindsOf(refs []resolve.Reference) []string {
	var out []string
	for _, ref := range refs {
		kind := "plain"
		switch {
		case ref.Redefines:
			kind = "redefines"
		case ref.Chain != nil:
			kind = "chain"
		case ref.Referrer != nil:
			kind = "reference"
		}
		out = append(out, kind)
	}
	return out
}
