package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// attributeModel exercises own, inherited, redefined and non-constant attributes.
const attributeModel = `
package Demo {
    part def Base {
        attribute mass : ISQ::MassValue = 1000.0 [SI::kg];
        attribute label : ScalarValues::String = "base";
        attribute inheritedOnly : ScalarValues::Real = 3.0;
    }
    part def Car :> Base {
        attribute :>> mass = 1600.0 [SI::kg];
        attribute wheels : ScalarValues::Integer = 4;
        attribute derived : ScalarValues::Real = wheels * 2;
        part engine;
    }
}
`

// byName indexes reported attributes, so a test names what it asserts.
func byName(attrs []*pb.AttributeInfo) map[string]*pb.AttributeInfo {
	out := make(map[string]*pb.AttributeInfo, len(attrs))
	for _, attr := range attrs {
		out[attr.Name] = attr
	}
	return out
}

// TestAttributesAreReportedOwnThenInherited verifies the attribute set is what
// the element has, own first, with a redefinition masking what it redefines and
// non-attribute members left out. What it inherits from the library is withheld,
// and counted rather than silently dropped.
func TestAttributesAreReportedOwnThenInherited(t *testing.T) {
	get := parseForFacts(t, attributeModel)
	car := get("Demo::Car")

	var names []string
	for _, attr := range car.Attributes {
		names = append(names, attr.Name)
	}
	want := []string{"mass", "wheels", "derived", "label", "inheritedOnly"}
	if len(names) != len(want) {
		t.Fatalf("attribute names = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("attribute names = %v, want %v", names, want)
		}
	}
	if car.WithheldLibraryAttributes == 0 {
		t.Error("the library attributes a part inherits are absent but not counted, which is silence")
	}
}

// A library element asked about directly reports what it declares: the policy
// withholds library-inherited attributes, not the library's own contents.
func TestOwnAttributesOfALibraryElementAreReported(t *testing.T) {
	get := parseForFacts(t, attributeModel)
	occurrence := get("Occurrences::Occurrence")
	if occurrence == nil {
		t.Fatal("the library element is not reported at all")
	}
	if len(occurrence.Attributes) == 0 {
		t.Error("Occurrences::Occurrence reports none of the attributes it declares")
	}
}

// TestAttributesCarryResolvedFacts verifies each attribute carries the type,
// unit and constant default the service resolves, including through a
// redefinition that restates neither.
func TestAttributesCarryResolvedFacts(t *testing.T) {
	get := parseForFacts(t, attributeModel)
	attrs := byName(get("Demo::Car").Attributes)

	mass := attrs["mass"]
	if mass.Type == "" {
		t.Error("redefining attribute reports no type; want the redefined one's")
	}
	if mass.Unit != "SI::kg" {
		t.Errorf("mass unit = %q, want SI::kg", mass.Unit)
	}
	if got := mass.Value.GetRealValue(); got != 1600.0 {
		t.Errorf("mass value = %v, want 1600", got)
	}

	if got := attrs["label"].Value.GetStringValue(); got != "base" {
		t.Errorf("inherited label = %q, want base (unquoted)", got)
	}
	if got := attrs["wheels"].Value.GetIntValue(); got != 4 {
		t.Errorf("wheels = %v, want 4", got)
	}
	if attrs["derived"].Value != nil {
		t.Errorf("derived reports a value %v; a non-constant default is absent, not guessed",
			attrs["derived"].Value)
	}
}

// TestARedefinitionWithANonConstantDefaultReportsNoValue verifies a
// redefinition that states a default the service cannot fold reports none,
// rather than the number it redefined and no longer states.
func TestARedefinitionWithANonConstantDefaultReportsNoValue(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Base {
        attribute mass : ScalarValues::Real = 1000.0;
    }
    part def Car :> Base {
        attribute wheels : ScalarValues::Integer = 4;
        attribute :>> mass = wheels * 2;
    }
}
`)
	attrs := byName(get("Demo::Car").Attributes)
	if value := attrs["mass"].Value; value != nil {
		t.Errorf("redefined mass reports %v; the model states a computed default, not %v",
			value, value)
	}
	if attrs["mass"].Type == "" {
		t.Error("redefined mass reports no type; the redefined one's still applies")
	}
}

// TestAnIntermediateNonConstantDefaultStopsTheChainWalk verifies an attribute
// writing no value takes the nearest stated default, not one an intermediate
// declaration already replaced with a computed default.
func TestAnIntermediateNonConstantDefaultStopsTheChainWalk(t *testing.T) {
	get := parseForFacts(t, `
package Demo {
    part def Base {
        attribute mass : ScalarValues::Real = 1000.0;
    }
    part def Mid :> Base {
        attribute wheels : ScalarValues::Integer = 4;
        attribute :>> mass = wheels * 2;
    }
    part def Leaf :> Mid {
        attribute :>> mass;
    }
}
`)
	if value := byName(get("Demo::Leaf").Attributes)["mass"].Value; value != nil {
		t.Errorf("Leaf mass reports %v; Mid replaced that default with a computed one", value)
	}
}

// TestAttributesOfAnElementWithNoneIsEmpty verifies an element declaring none
// reports none, and still states how many library-inherited ones it left out.
func TestAttributesOfAnElementWithNoneIsEmpty(t *testing.T) {
	get := parseForFacts(t, attributeModel)
	engine := get("Demo::Car::engine")
	if attrs := engine.Attributes; len(attrs) != 0 {
		t.Errorf("engine attributes = %v, want none", attrs)
	}
	if engine.WithheldLibraryAttributes == 0 {
		t.Error("engine inherits library attributes but reports none withheld")
	}
}
