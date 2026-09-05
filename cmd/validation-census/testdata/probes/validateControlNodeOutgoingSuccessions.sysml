// Census: validateControlNodeOutgoingSuccessions
// Expect: error: successions out of a join node must have source multiplicity [1]
package CN07JoinOutgoingSourceMultiplicity {
    action def A {
        action a;
        action b;
        join j;
        action c;
        first a then j;
        first b then j;
        succession first [0..1] j then c;
    }
}
