// A sequence literal of sequences flattens to one sequence of numbers, which is
// no collection of vectors: sum reports the element that is not a vector.
package test {
	private import ScalarValues::*;
	private import VectorFunctions::*;

	calc def total {
		return : Real[3] = sum(((1.0, 2.0, 3.0), (4.0, 5.0, 6.0)));
	}
}
