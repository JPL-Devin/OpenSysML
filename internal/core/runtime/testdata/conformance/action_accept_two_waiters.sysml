package test {
	private import ScalarValues::*;

	// Two accepts park at once, in parallel branches, and each resumes on the
	// message of its own type: the String sent first does not resume the
	// Integer accept, and neither branch consumes the other's message.
	action communicator {
		attribute number : Integer = 0;
		attribute message : String = "none";

		first start;

		fork split;

		action listenNumber accept n : Integer;

		action listenText accept t : String;

		action gate;

		action sender {
			send "go" to listenText;
			send 5 to listenNumber;
		}

		join sync;

		action recorder {
			assign number := n;
			assign message := t;
		}

		done end;

		then start split;
		then split listenNumber;
		then split listenText;
		then split gate;
		then listenNumber sync;
		then listenText sync;
		then gate sender;
		then sender sync;
		then sync recorder;
		then recorder end;
	}
}
