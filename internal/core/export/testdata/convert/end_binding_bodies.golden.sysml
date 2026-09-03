package R89 {
    enum def SeamCoupling {
        learnFromData;
        fixed;
    }
    port def P {
        out item x;
    }
    part def W {
        port outp : P;
    }
    part def Rd {
        port inp : ~P;
    }
    part def Ctx {
        part w : W;
        part r : Rd;
        interface seam connect w.outp to r.inp {
            attribute coupling : SeamCoupling = SeamCoupling::learnFromData;
        }
    }
}
package Bodies {
    attribute def Prio;
    item def Fuel;
    part def A {
        port p;
        attribute a;
        out item i : Fuel;
    }
    part def B {
        port q;
        attribute b;
        in item j : Fuel;
    }
    part def Ctx {
        part x : A;
        part y : B;
        connection c connect x.p to y.q {
            attribute w : Prio;
        }
        connect x.p to y.q {
            attribute w : Prio;
        }
        flow f from x.i to y.j {
            attribute w : Prio;
        }
        binding bd bind x.a = y.b {
            attribute w : Prio;
        }
        bind x.a = y.b {
            attribute w : Prio;
        }
        allocation al allocate x to y {
            attribute w : Prio;
        }
        succession s first x then y {
            attribute w : Prio;
        }
        action def Act {
            action a1;
            action a2;
            succession first a1 then a2 {
                attribute w : Prio;
            }
            first a1 then a2 {
                attribute w : Prio;
            }
        }
    }
}
