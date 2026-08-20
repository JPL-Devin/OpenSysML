package test {
	private import ScalarValues::*;

	// An accept reached before its message suspends the action rather than
	// failing. The fork puts `reader` and the sending branch in parallel, and
	// `gate` delays the send by one step, so `reader` is parked at its accept
	// before `sender` posts anything: `received` is 7 only if the parked token
	// resumed when the message arrived.
	action communicator {
		attribute received : Integer = 0;

		first start;

		fork split;

		action reader accept n : Integer;

		action gate;

		action sender {
			send 7 to reader;
		}

		join sync;

		action recorder {
			assign received := n;
		}

		done end;

		then start split;
		then split reader;
		then split gate;
		then reader sync;
		then gate sender;
		then sender sync;
		then sync recorder;
		then recorder end;
	}
}
