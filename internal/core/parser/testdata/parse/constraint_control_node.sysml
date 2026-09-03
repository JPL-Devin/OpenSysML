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
// The statements of a calculation body are read too, so a node nested in one
// of their bodies is reached rather than the statement failing to parse.
constraint def Guarded {
    in attribute y : ScalarValues::Integer;
    if y > 0 { fork f; }
    while y > 0 { join j; }
    x = 3;
    y > 0
}
part def Valve {
    assert constraint c { if x > 0 { decide d; } x > 0 }
    constraint c2 { if x > 0 ? true else false }
}
requirement def Bounds {
    require constraint q { if x > 0 { fork f; } x > 0 }
    assume constraint { while x > 0 { join j; } }
    assume constraint { if x > 0 ? true else false }
}
