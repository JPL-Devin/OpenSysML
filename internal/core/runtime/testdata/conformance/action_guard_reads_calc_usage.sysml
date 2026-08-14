package test {
    private import ScalarValues::*;

    calc def Twice {
        in k : Real;
        out d = k * 2.0;
    }

    action run {
        attribute i : Integer = 0;
        attribute log : Real = 0.0;
        calc t : Twice { in k = i; }

        first start;
        action bump {
            assign i := i + 1;
            assign log := log + t.d;
        }
        decision check;
        done fin;

        then start bump;
        then bump check;
        then check bump if t.d < 6.0;
        then check fin if t.d >= 6.0;
    }
}
