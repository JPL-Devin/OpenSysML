// Census: validateJoinNodeOutgoingSuccessions
// Expect: error: a join node may have at most one
package CN02JoinTwoOutgoing {
    action def A {
        action a;
        action b;
        join j;
        action c;
        action d;
        first a then j;
        first b then j;
        first j then c;
        first j then d;
    }
}
