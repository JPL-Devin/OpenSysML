// Census: validateSpecializationSpecificNotConjugated
// Expect: error: Conjugated type cannot be a specialized type
package P {
  class A; class B;
  class C ~ A specializes B;
}
