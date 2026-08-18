// A redefining declaration whose body values features the value it inherits
// would supply governs over that value: the more specific declaration of the
// feature holds, while a body that only re-declares features still reads the
// inherited value.
package test {
    private import ScalarValues::Real;

    attribute def Cost { attribute v : Real = 1.0; attribute w : Real = 2.0; }

    part def Ring {
        attribute template : Cost { attribute :>> v = 9.0; attribute :>> w = 8.0; }
        attribute cost : Cost = template;
        attribute kept : Cost = template;
        attribute plain : Cost = template;
    }

    part r : Ring {
        attribute :>> cost { attribute :>> v = 11.0; }
        // A body that only re-declares a feature states no value, so the
        // inherited value still binds this one.
        attribute :>> kept { attribute :>> v; }

        assert constraint bodyGovernsInheritedValue { cost.v == 11.0 }
        assert constraint featureNotValuedTakesItsTypesDefault { cost.w == 2.0 }
        assert constraint reDeclarationKeepsInheritedValue { kept.v == 9.0 }
        assert constraint valueBindsWithoutABody { plain.v == 9.0 }
    }
}
