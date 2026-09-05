// A library function this runtime has no representation for the values of
// reports itself by name: a tensor quantity has no value kind.
package test {
	private import ScalarValues::*;
	private import TensorCalculations::*;

	calc def zeroTensor {
		return : Boolean = isZeroTensorQuantity(1.0);
	}
}
