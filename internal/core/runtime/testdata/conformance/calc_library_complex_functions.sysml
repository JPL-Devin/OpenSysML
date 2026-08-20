// A calc body computing with ComplexFunctions: a Complex is the (re, im) pair
// rect constructs, and i squared is -1.
package test {
	private import ScalarValues::*;
	private import ComplexFunctions::*;

	calc def imaginaryUnitSquared {
		return : Real = re(ComplexFunctions::'*'(rect(0.0, 1.0), rect(0.0, 1.0)));
	}
}
