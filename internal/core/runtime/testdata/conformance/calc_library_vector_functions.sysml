// A calc body computing with VectorFunctions: a vector is the sequence of its
// elements, so the library's Cartesian operations compute over a sequence.
package test {
	private import ScalarValues::*;

	calc def scaledSum {
		in x : Real;
		return : Real[2] = VectorFunctions::cartesianVectorScalarMult(
			VectorFunctions::'cartesian+'((1.0, 2.0), (3.0, 4.0)), x);
	}
}
