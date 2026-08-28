// A name the performed action does not declare still writes the feature of the
// object performing it.
package test {
	private import ScalarValues::*;

	action def Mark {
		action step {
			assign seen := seen + 1;
		}
		first step;
	}

	part def Host {
		attribute seen : Integer = 0;
		perform action marking : Mark;
	}
}
