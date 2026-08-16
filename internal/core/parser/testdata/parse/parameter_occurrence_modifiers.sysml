// A parameter is a usage declaration prefixed by its direction, so a lone occurrence
// modifier declares the kind and the declaration may be omitted (SysML.xtext RefPrefix).
package ParameterModifiers {
    occurrence def Flight;
    part def Vehicle;
    action def Observe {
        in snapshot ;
        in event ;
        in ;
        in x;
        in individual v : Vehicle;
        in snapshot atStart : Flight;
        in timeslice item cargoInFlight;
        out mass : Real;
        inout ref counter : Integer;
    }
}
