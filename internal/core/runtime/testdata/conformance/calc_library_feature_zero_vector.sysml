// A vector-valued library feature reads as a name too, and the vector it
// answers is the sequence of its elements.
package test {
	private import ScalarValues::*;

	calc def displacement {
		return : Real[3] = VectorFunctions::cartesian3DZeroVector;
	}
}
