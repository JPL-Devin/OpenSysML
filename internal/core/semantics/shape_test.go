package semantics_test

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// shapeModel indexes src over the bundled libraries.
func shapeModel(t *testing.T, src string) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(src))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

func symbolNames(syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return out
}

// The constructible features are the shape features the type's own tier writes:
// own then inherited, a calc holding no position, the library's descriptors of
// the kind (`subitems`, `shape`) left out of a model type unless it restates one.
func TestConstructibleFeaturesAreTheModelsOwnShapeFeatures(t *testing.T) {
	m, idx := shapeModel(t, `package T {
		private import ScalarValues::*;
		item def Base { attribute a : Integer; }
		item def Frame :> Base {
			attribute x : Integer;
			calc doubled { x * 2 }
			attribute y : String;
			item :>> subitems : Frame[0..*];
		}
	}`)
	frame := dimensionSymbol(t, idx, "T::Frame")
	if got, want := symbolNames(m.ConstructibleFeatures(frame)), []string{"x", "y", "subitems", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ConstructibleFeatures(Frame) = %v, want %v", got, want)
	}
	rotation := dimensionSymbol(t, idx, "MeasurementReferences::Rotation")
	if got, want := symbolNames(m.ConstructibleFeatures(rotation)), []string{"axisDirection", "angle", "isIntrinsic"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ConstructibleFeatures(Rotation) = %v, want %v", got, want)
	}
	var shape []string
	for _, f := range m.ShapeFeatures(frame) {
		shape = append(shape, f.Name)
	}
	if got, want := shape[:4], []string{"x", "y", "subitems", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ShapeFeatures(Frame) starts %v, want %v", got, want)
	}
	if len(shape) == 4 {
		t.Errorf("ShapeFeatures(Frame) = %v, want the Item descriptors after the model's own", shape)
	}
}

// A feature redefined under another name, directly or through a chain, holds one
// position, that of its earliest declaration; every name of it binds that position.
func TestConstructibleFeaturesMergeRedefinitionsUnderOtherNames(t *testing.T) {
	m, idx := shapeModel(t, `package T {
		private import ScalarValues::*;
		item def Base { attribute a : Integer; attribute k : String; }
		item def Mid :> Base { attribute b redefines a; }
		item def Leaf :> Mid { attribute c redefines b; attribute z : Boolean; }
	}`)
	leaf := dimensionSymbol(t, idx, "T::Leaf")
	if got, want := symbolNames(m.ConstructibleFeatures(leaf)), []string{"c", "z", "k"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ConstructibleFeatures(Leaf) = %v, want %v", got, want)
	}
	c := m.ConstructibleFeatures(leaf)[0]
	for _, qn := range []string{"T::Leaf::c", "T::Mid::b", "T::Base::a"} {
		if got := m.ConstructibleFeatureFor(leaf, dimensionSymbol(t, idx, qn)); got != c {
			t.Errorf("ConstructibleFeatureFor(Leaf, %s) = %v, want c", qn, got)
		}
	}
	if got := m.ConstructibleFeatureFor(leaf, dimensionSymbol(t, idx, "T::Base::k")); got == nil || got.Name != "k" {
		t.Errorf("ConstructibleFeatureFor(Leaf, k) = %v, want k", got)
	}
	mid := dimensionSymbol(t, idx, "T::Mid")
	if got, want := symbolNames(m.ConstructibleFeatures(mid)), []string{"b", "k"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ConstructibleFeatures(Mid) = %v, want %v", got, want)
	}
}
