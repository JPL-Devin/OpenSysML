// A small model that exercises the highlighting: definitions, usages,
// expressions, state behavior and library references.
package VehicleDemo {
    private import ScalarValues::*;

    /* Wheels carry a pressure that the constraint below bounds. */
    attribute def Pressure :> Real;

    part def Wheel {
        attribute pressure : Pressure = 32.0;
        attribute worn : Boolean = false;
    }

    part def Engine {
        attribute power : Real;
        action start;
    }

    part def Vehicle {
        part engine : Engine;
        part wheels : Wheel[4];

        attribute mass : Real = 1200.0;

        constraint healthy {
            wheels->forAll(w => w.pressure > 28.0 and not w.worn)
        }

        state machine {
            entry; then parked;

            state parked;
            state moving;

            transition parked_to_moving
                first parked
                accept when engine.power > 0.0
                then moving;
        }
    }

    part myCar : Vehicle {
        part redefines engine {
            attribute redefines power = 110.0;
        }
    }

    part 'spare wheel' : Wheel;
}
