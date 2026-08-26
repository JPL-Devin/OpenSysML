// A state whose do behavior has actions left while the only queued event is far
// in the future, so a small %advance must still run the behavior.
package Debug {
	state Slow {
		attribute count = 0;

		initial init;
		state working {
			do {
				count = count + 1;
				count = count + 1;
			}
			accept after 100 then done;
		}
		final done;

		succession first init then working;
	}
}
