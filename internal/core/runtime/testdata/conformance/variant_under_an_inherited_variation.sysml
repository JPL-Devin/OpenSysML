// A usage typed by a variation definition is a variation point without
// restating the modifier, so the variants declared under it are choices it can
// be bound to rather than features of the object.
package test {
    private import ScalarValues::Real;

    part def Engine { attribute power : Real = 100.0; }
    variation part def EngineChoice :> Engine;

    part def Car {
        part engine : EngineChoice {
            variant part electric : Engine { attribute :>> power = 150.0; }
            variant part petrol : Engine { attribute :>> power = 90.0; }
        }
        attribute curbMass : Real = 900.0 + engine.power;
    }

    part electricCar : Car {
        part :>> engine = engine::electric;

        assert constraint isElectric {
            engine == engine::electric
        }
    }
}
