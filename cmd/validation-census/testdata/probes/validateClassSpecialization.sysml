// Census: validateClassSpecialization
// Expect: error: item cannot specialize attributeDef (kind mismatch)
package P { attribute def A; item def I :> A; }
