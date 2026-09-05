// Census: validateForkNodeIncomingSuccessions
// Expect: error: a fork node may have at most one
package CN01ForkTwoIncoming {
    action def A {
        action a;
        action b;
        fork f;
        action c;
        action d;
        first a then f;
        first b then f;
        first f then c;
        first f then d;
    }
}
