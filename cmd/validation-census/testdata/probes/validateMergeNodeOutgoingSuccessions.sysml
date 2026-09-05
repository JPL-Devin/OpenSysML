// Census: validateMergeNodeOutgoingSuccessions
// Expect: error: a merge node may have at most one
package CN03MergeTwoOutgoing {
    action def A {
        action a;
        action b;
        merge m;
        action c;
        action d;
        first a then m;
        first b then m;
        first m then c;
        first m then d;
    }
}
