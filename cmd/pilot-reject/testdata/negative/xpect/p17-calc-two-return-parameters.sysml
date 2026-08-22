// Invalid: Only one return parameter is allowed (SysML 7.15 validateReturnParameterMembership; pilot CalculationUsage_Invalid1.sysml.xt).
package P17CalcTwoReturnParameters {
    attribute def T;
    calc def C {
        return r1 : T;
        return r2 : T;
    }
}
