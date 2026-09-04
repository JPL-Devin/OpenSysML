// Census: validateDataTypeSpecialization
// Expect: error: attribute cannot specialize itemDef (kind mismatch)
package P { item def I; attribute def A :> I; }
