// Census: validateAssociationBinarySpecialization
// Expect: error: redefines no end of
package P {
  class A; class B; class C;
  assoc Bin specializes Links::BinaryLink { end a : A; end b : B; end c : C; }
}
