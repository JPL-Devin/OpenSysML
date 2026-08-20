// A library function this runtime has no representation for the values of
// reports itself by name: an aggregation over a collection of vectors.
package test {
	private import ScalarValues::*;
	private import VectorFunctions::*;

	calc def total {
		return : Real[3] = sum(((1.0, 2.0, 3.0), (4.0, 5.0, 6.0)));
	}
}
