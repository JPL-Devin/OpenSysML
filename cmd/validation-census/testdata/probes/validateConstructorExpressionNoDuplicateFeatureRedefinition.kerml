// Census: validateConstructorExpressionNoDuplicateFeatureRedefinition
// Expect: error: C binds feature "x" twice
package P {
  class T;
  class C { feature x : T; }
  feature t : T;
  feature c : C = new C(x = t, x = t);
}
