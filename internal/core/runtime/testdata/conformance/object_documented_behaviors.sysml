// A note beside a behaviour a type binds describes the binding, not the
// behaviour: the object still runs the machine and the action the binding names.
package test {
	private import ScalarValues::*;

	state def Counting {
		entry; then idle;

		state idle {
			entry action bump {
				assign count := count + 1;
			}
		}
	}

	action def Report {
		first start;

		action log {
			assign reported := true;
		}

		succession first start then log;
	}

	part def Counter {
		attribute count : Integer = 0;
		attribute reported : Boolean = false;

		exhibit state counting : Counting {
			doc /* how the counter counts */
		}

		perform action report : Report {
			doc /* what the counter reports */
		}
	}
}
