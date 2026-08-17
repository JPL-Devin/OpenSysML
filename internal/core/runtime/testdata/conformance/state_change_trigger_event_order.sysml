// The order the driver dispatches in: a change condition already true is taken
// before a queued signal and before a timer that is due later, since it is
// enabled at the current time. The signal is not lost — it is dispatched by the
// next step, in the state the change transition led to.
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;
		attribute ready : Boolean = true;

		initial start;
		state A {
			accept after 5 [s] then late;
		}
		state B;
		state late {
			entry { log = log * 10 + 8; }
		}
		state C {
			entry { log = log * 10 + 3; }
		}

		start then A;
		transition A to B accept when ready do assign log := log * 10 + 1;
		transition A to late when Go do assign log := log * 10 + 9;
		transition B to C when Go do assign log := log * 10 + 2;
	}
}
