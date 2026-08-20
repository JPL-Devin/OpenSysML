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
