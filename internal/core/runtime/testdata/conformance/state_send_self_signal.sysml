package test {
	// A machine that drives itself: the entry action of `waiting` sends Ping,
	// and the only transition out of `waiting` accepts Ping. Nothing is injected
	// from outside, so `done` is reached only if the sent message actually
	// reaches the machine's own event queue.
	item def Ping;

	state Driver {
		initial start;

		state waiting {
			entry { send Ping to Driver; }
		}

		final done;

		start then waiting;

		transition waiting to done when Ping;
	}
}
