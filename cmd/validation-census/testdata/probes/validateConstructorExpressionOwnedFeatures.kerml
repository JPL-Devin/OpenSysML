// Census: validateConstructorExpressionOwnedFeatures
// Expect: error: unresolved member: y
package P { class T; class C { feature x : T; } feature t : T; feature c : C = new C(y = t); }
