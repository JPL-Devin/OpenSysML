// Census: validateDecisionNodeOutgoingSuccessions
// Expect: error: successions out of a decision node must have target multiplicity [0..1]
package CN09DecideOutgoingTargetMultiplicity {
    action def A {
        action a;
        decide d;
        action b;
        action c;
        first a then d;
        succession first d then [1] b;
        succession first d then [0..1] c;
    }
}
