// Census: validateTransitionFeatureMembershipGuardExpression
// Expect: error: transition guard must be Boolean
package P { state def S { state a; state b; transition first a if "yes" then b; } }
