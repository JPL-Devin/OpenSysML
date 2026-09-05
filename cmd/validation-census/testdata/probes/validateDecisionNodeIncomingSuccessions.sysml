// Census: validateDecisionNodeIncomingSuccessions
// Expect: error: a decision node may have at most one
package CN04DecideTwoIncoming {
    private import ScalarValues::Boolean;
    action def A {
        in attribute go : Boolean;
        action a;
        action b;
        decide d;
        action c;
        action e;
        first a then d;
        first b then d;
        first d if go then c;
        first d if not go then e;
    }
}
