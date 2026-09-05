// Census: validateControlNodeIncomingSuccessions
// Expect: error: successions into a fork node must have target multiplicity [1]
package CN06ForkIncomingTargetMultiplicity {
    action def A {
        action a;
        fork f;
        action b;
        action c;
        succession first a then [0..1] f;
        first f then b;
        first f then c;
    }
}
