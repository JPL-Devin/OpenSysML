package test {
	private import ScalarValues::*;

	// Two tokens out of a fork perform `leg` at once. A binding between two of its nested
	// pins (`leg.inner.v = leg.rest.n`) and one to its own pin (`leg.inner.w = leg.x`) hold
	// within each performance: each rest takes its own inner's value, never the other's,
	// so the sums are 10 * 1 + 20 * 2, not 20 * 1 + 10 * 2.
	action pipeline {
		out attribute collected : Integer = 0;

		bind leg.inner.w = leg.x;
		bind leg.inner.v = leg.rest.n;

		fork split;
		action left { out v : Integer; assign v := 1; }
		action right { out v : Integer; assign v := 2; }
		action leg {
			in x : Integer;
			action inner { in w : Integer; out v : Integer; assign v := w * 10; }
			action rest { in n : Integer; assign collected := collected + n * x; }
			first inner;
			then rest;
		}
		join sync;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then leg;
		succession first right then leg;
		succession first leg then sync;
		succession first sync then done;

		flow left.v to leg.x;
		flow right.v to leg.x;
	}
}
