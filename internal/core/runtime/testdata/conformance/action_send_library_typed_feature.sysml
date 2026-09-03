package test {
	private import ScalarValues::*;
	private import RealFunctions::sqrt;

	// `root` is a feature typed directly by a library function: a call of it applies
	// that function. The send delivers the computed Real — 4.0 — which is the only
	// message the accept takes; a send of a `root` message carrying 16.0 would deadlock.
	ref root : sqrt;

	action pipeline {
		attribute got : Real = 0.0;
		attribute direct : Real = 0.0;

		first start;

		action sender {
			assign direct := root(x = 16.0);
			send root(16.0) to reader;
		}

		action reader accept n : Real;

		action recorder {
			assign got := n;
		}

		done;

		succession first start then sender;
		succession first sender then reader;
		succession first reader then recorder;
		succession first recorder then done;
	}
}
