// Census: validateMergeNodeIncomingSuccessions
// Expect: error: successions into a merge node must have source multiplicity [0..1]
package CN08MergeIncomingSourceMultiplicity {
    action def A {
        action a;
        action b;
        merge m;
        action c;
        succession first [1] a then m;
        first b then m;
        first m then c;
    }
}
