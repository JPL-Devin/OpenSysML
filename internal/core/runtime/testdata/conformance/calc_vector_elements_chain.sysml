// A numerical vector is the sequence of its elements, so chaining into the
// `elements` feature of a VectorOf result reads that same sequence.
package test {
	private import ScalarValues::*;
	private import VectorFunctions::*;

	calc def probe {
		return : Real[*] = 'cartesian+'(
			CartesianThreeVectorOf((1.0, 2.0, 3.0)),
			CartesianThreeVectorOf((4.0, 5.0, 6.0))).elements;
	}
}
