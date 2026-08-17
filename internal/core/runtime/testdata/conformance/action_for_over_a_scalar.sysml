// A `for` iterates a collection, so a scalar input is reported: reading it as a
// one-element collection would hide the modelling error behind one iteration.
package test {
	private import ScalarValues::*;

	action run {
		attribute single : Integer = 7;
		attribute visited : Integer = 0;

		first start;

		action iterate {
			for s in single {
				assign visited := visited + 1;
			}
		}

		done end;

		then start iterate;
		then iterate end;
	}
}
