// Exercised from outside the package: resolving a chain or an inherited member
// needs the semantic model, which imports resolve.
package resolve_test

import (
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

// A named multiplicity's bounds and the multiplicity it subsets are names an
// editor navigates, so the collector has to report them.
func TestNamedMultiplicitiesReferencesAreCollected(t *testing.T) {
	const src = `package M {
	attribute def Count;
	attribute limit : Count;
	multiplicity exactlyOne [1];
	multiplicity upToLimit [0..limit];
	multiplicity fewer subsets upToLimit;
}`
	r, root, rootScope := resolvedDocNamed(t, "m.kerml", src)
	found := map[string]bool{}
	for _, ref := range resolve.References(root, rootScope) {
		found[nameText(ref.QN)] = true
		if _, ok := r.ResolveReference(ref); !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
		}
	}
	for _, want := range []string{"Count", "limit", "upToLimit"} {
		if !found[want] {
			t.Errorf("References does not report %s", want)
		}
	}
}

// A cast names its type and may bound it by a feature; both are references the
// document walk resolves and the collector reports.
func TestCastReferencesAreCollected(t *testing.T) {
	const src = `package C {
	part def Shape;
	attribute limit;
	part s : Shape;
	attribute one = (as Shape);
	attribute some = (as Shape[0..limit]);
}`
	walk, root, rootScope := resolvedDocNamed(t, "c.sysml", src)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}
	found := map[string]int{}
	for _, ref := range resolve.References(root, rootScope) {
		found[nameText(ref.QN)]++
		if _, ok := walk.ResolveReference(ref); !ok {
			t.Errorf("%s does not resolve on its own", nameText(ref.QN))
		}
	}
	if found["Shape"] != 3 || found["limit"] != 1 {
		t.Errorf("References reports Shape %d times and limit %d times, want 3 and 1", found["Shape"], found["limit"])
	}
}

// An import's target is collected as the import's reference, read with its rule:
// a spelling only the import itself surfaces reaches nothing, while an `import
// all` reaches a private member.
func TestImportTargetsResolveWithoutTheImport(t *testing.T) {
	const src = `package Lib {
	part def Cell;
	private part def Hidden;
}
package Consumer {
	package Lib {
		part def Cell;
	}
	private import Lib::Cell;
}
package Opener {
	private import all Lib::Hidden;
}`
	r, root, rootScope := resolvedDoc(t, src)
	var refs []resolve.Reference
	for _, ref := range resolve.References(root, rootScope) {
		if ref.Import != nil {
			refs = append(refs, ref)
		}
	}
	if len(refs) != 2 {
		t.Fatalf("References tags %d import targets, want 2", len(refs))
	}
	cell, ok := r.ProbeReference(refs[0])
	if !ok || nameText(refs[0].QN) != "Lib::Cell" {
		t.Fatalf("`import Lib::Cell` = %v, %v; want the nested Lib's Cell", cell, ok)
	}
	if sym, ok := r.ProbeReference(refs[0].Spelled(spelling(false, "Cell"))); ok {
		t.Errorf("`Cell` written as the import's own target reaches %s through the import itself", sym.Name)
	}
	if sym, ok := r.ProbeReference(refs[1]); !ok || sym.Name != "Hidden" {
		t.Errorf("`import all Lib::Hidden` = %v, %v; want the private member", sym, ok)
	}
	if sym, ok := r.ProbeReference(refs[1].Spelled(spelling(true, "Lib", "Hidden"))); !ok || sym.Name != "Hidden" {
		t.Errorf("`$::Lib::Hidden` read as an `import all` target = %v, %v; want the private member", sym, ok)
	}
}

// spelling is a qualified name on a fresh node, as a trial spelling is.
func spelling(global bool, parts ...string) *ast.QualifiedName {
	qn := &ast.QualifiedName{Global: global}
	for _, part := range parts {
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: part})
	}
	return qn
}

// `first x` may be written before `x` is declared in the same body, so the
// initial node names that later declaration rather than its own label.
func TestAnInitialReferenceReachesALaterDeclaration(t *testing.T) {
	const src = `package P {
	action def Drive {
		first go;
		then action stop;
		action go;
	}
	action def Idle {
		first start;
		then action wait;
	}
}`
	r, root, _ := resolvedDoc(t, src)
	initials := map[string]*ast.InitialNode{}
	for _, def := range declared(root.Members[0]).(*ast.Package).Members {
		for _, member := range declared(def).(*ast.Definition).Members {
			if n, ok := declared(member).(*ast.InitialNode); ok {
				initials[n.Name] = n
			}
		}
	}
	sym, ok := r.InitialSymbol(initials["go"])
	if !ok {
		t.Fatal("`first go` names the action declared after it, but InitialSymbol reports none")
	}
	if usage, isUsage := sym.Decl.(*ast.Usage); !isUsage || usage.Ident.Name != "go" {
		t.Errorf("`first go` names %T, want the action usage `go`", sym.Decl)
	}
	if sym, ok := r.InitialSymbol(initials["start"]); ok {
		t.Errorf("`first start` names %T, but no member is called start", sym.Decl)
	}
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

// declared is the member a membership wraps, or the node itself.
func declared(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
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
