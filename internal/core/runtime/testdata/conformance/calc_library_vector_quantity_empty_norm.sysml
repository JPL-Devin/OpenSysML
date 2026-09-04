// CartesianVectorOf takes Real[*], so a vector quantity may have no axes: its
// norm is the plain 0.0, as there is no axis whose unit could carry it.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import SI::*;

	calc def emptyNorm {
		return : Real = norm(CartesianVectorOf(()) [m]);
	}
}
