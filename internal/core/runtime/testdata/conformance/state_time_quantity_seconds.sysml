// A time trigger carrying a unit schedules the delay the unit measures, so
// `accept after 5 [s]` waits as the unitless `accept after 5` does.
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		initial start;
		state waiting {
			accept after 5 [s] then done;
		}
		state done {
			entry { log = log * 10 + 1; }
		}

		succession first start then waiting;
	}
}
