// A state whose do behavior has actions left with nothing queued, so %advance
// must report them as do work rather than as events processed.
package Debug {
	state Work {
		attribute count = 0;

		initial init;
		state busy {
			do {
				count = count + 1;
				count = count + 1;
				count = count + 1;
			}
		}
		final done;

		init then busy;
		busy then done;
	}
}
