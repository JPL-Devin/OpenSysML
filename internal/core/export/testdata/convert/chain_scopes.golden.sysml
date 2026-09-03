package Gen {
    part def G1 {
        attribute x;
        attribute y;
    }
    part def G2 {
        attribute x;
    }
}
package Chains {
    part def Wheel :> Gen::G1, Gen::G2;
    part def Car {
        part w : Wheel;
        attribute a = w.Gen::G1::x;
        attribute b = w.Gen::G2::x;
        attribute c = w.y;
    }
}
