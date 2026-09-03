// A constraint body is a calculation body (SysML.xtext CalculationBody), so it
// carries action nodes; whether their owner may hold them is a semantic rule.
constraint def Bounded {
    fork f;
    join j;
    true
}
part def Tank {
    constraint c {
        merge m;
        decide d;
        true
    }
}
requirement def R {
    require constraint q { merge m; }
    assume constraint { fork f; }
}
part def Pump {
    assert constraint c { decide d; }
}
