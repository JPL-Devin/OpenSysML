// A library feature registered without a computable value reports itself as a
// diagnostic rather than answering nothing: cartesianZeroVector has no
// dimension to build a value of.
package test {
	private import ScalarValues::*;

	calc def anyDimensionZero {
		return : Real[0..*] = VectorFunctions::cartesianZeroVector;
	}
}
