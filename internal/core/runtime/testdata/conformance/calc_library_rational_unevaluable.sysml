// A Rational is a float64 in this runtime, so its exact numerator and
// denominator are not evaluable, and the function says so by name.
package test {
	private import ScalarValues::*;

	calc def third {
		return : Rational = RationalFunctions::rat(1, 3);
	}
}
