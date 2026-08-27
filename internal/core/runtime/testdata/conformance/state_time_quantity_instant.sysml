// An instant carrying a unit names a time on the same clock a duration is
// measured on: `accept at 2 [min]` is due 120 seconds in.
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state waiting {
			accept at 2 [min] then done;
		}
		state done {
			entry { log = log * 10 + 1; }
		}

		succession first start then waiting;
	}
}
