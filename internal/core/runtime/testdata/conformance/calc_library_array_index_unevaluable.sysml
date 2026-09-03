// The runtime has no Array value kind, so indexing one is not evaluable.
package test {
	private import ScalarValues::*;

	calc def cell {
		return : Integer = CollectionFunctions::'array#'((1, 2, 3, 4), (2, 2));
	}
}
