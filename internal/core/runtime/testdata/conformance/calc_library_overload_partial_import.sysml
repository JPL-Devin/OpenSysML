// Importing some of the function packages does not hide the rest of the Kernel
// Function Library: a Complex argument still selects ComplexFunctions::abs.
package test {
	private import ScalarValues::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import RationalFunctions::*;

	part def Probe {
		attribute absInteger = abs(-2);
		attribute absComplex = abs(rect(3.0, 4.0));
		attribute zeroComplex = isZero(rect(0.0, 0.0));
	}
}
