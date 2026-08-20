package test {
	private import ScalarValues::*;

	// The reader is declared before the accept node that binds the payload and
	// runs after it: a body's feature space is not ordered by declaration
	// position.
	action communicator {
		attribute seen : Integer = 0;

		first start;

		action processor {
			assign seen := msg + 1;
		}

		action receiver accept msg : Integer;

		action sender {
			send 4 to receiver;
		}

		done end;

		then start sender;
		then sender receiver;
		then receiver processor;
		then processor end;
	}
}
