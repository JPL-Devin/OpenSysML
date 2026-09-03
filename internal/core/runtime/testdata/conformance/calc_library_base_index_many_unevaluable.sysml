// BaseFunctions::'#' with several indexes addresses an Array, a value kind the
// runtime does not have, so only the single-index form is evaluable.
package test {
	private import ScalarValues::*;

	calc def cell {
		return : Integer = BaseFunctions::'#'((1, 2, 3, 4), (2, 2));
	}
}
