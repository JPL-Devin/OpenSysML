// A machine whose only outgoing transition is a change trigger: the condition is
// made true by the state's own do behavior, and the transition out of `armed`
// waits on one that never holds.
package Debug {
	state Watch {
		attribute ready = false;
		attribute blocked = false;

		initial init;
		state waiting {
			do {
				ready = true;
			}
		}
		state armed;
		final done;

		succession first init then waiting;
		transition waiting to armed accept when ready;
		transition armed to done accept when blocked;
	}
}
