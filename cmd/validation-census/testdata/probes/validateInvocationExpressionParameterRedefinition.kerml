// Census: validateInvocationExpressionParameterRedefinition
// Expect: error: has no parameter named
package P {
  function F { in x : ScalarValues::Integer; return : ScalarValues::Integer; x }
  feature r : ScalarValues::Integer = F(y = 1);
}
