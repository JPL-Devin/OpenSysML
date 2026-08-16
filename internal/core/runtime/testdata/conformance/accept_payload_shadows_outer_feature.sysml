package test {
	import ScalarValues::*;

	attribute msg : Integer = 1;

	// The payload is the nearer declaration, so it shadows the package attribute
	// of the same name (KerML 8.2.3.5.3).
	action communicator {
		attribute seen : Integer = 0;

		first start;

		action sender {
			send 9 to receiver;
		}

		action receiver accept msg : Integer;

		action processor {
			assign seen := msg;
		}

		done end;

		then start sender;
		then sender receiver;
		then receiver processor;
		then processor end;
	}
}
