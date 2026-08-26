// A packaged state machine with timed transitions, so %advance has events
// scheduled at distinct times to drain.
package Debug {
	state Cycle {
		entry; then init;
		state init;
		state waiting {
			accept after 10 then working;
		}
		state working {
			accept after 5 then done;
		}
		final done;

		succession first init then waiting;
	}
}
