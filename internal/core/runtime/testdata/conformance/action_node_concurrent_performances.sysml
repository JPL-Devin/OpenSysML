package test {
	private import ScalarValues::*;

	// Two tokens out of a fork perform the same flow-owning node at once. Each
	// performance holds the payload delivered for it, works in its own frame, and
	// sends its own output on: the flows carry 1 and 2 through `stage` as 10 and
	// 20, never one performance's value twice.
	action pipeline {
		out attribute scaled : Integer = 0;
		out attribute collected : Integer = 0;

		fork split;
		action left { out v : Integer; assign v := 1; }
		action right { out v : Integer; assign v := 2; }
		action stage {
			in x : Integer;
			out y : Integer;
			action scale { assign y := x * 10; }
			action tally { assign scaled := scaled + y; }
			first scale;
			then tally;
		}
		action collect { in z : Integer; assign collected := collected + z; }
		join sync;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then stage;
		succession first right then stage;
		succession first stage then collect;
		succession first collect then sync;
		succession first sync then done;

		flow left.v to stage.x;
		flow right.v to stage.x;
		flow stage.y to collect.z;
	}
}
