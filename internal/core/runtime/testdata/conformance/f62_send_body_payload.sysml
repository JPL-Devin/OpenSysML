package test {
	private import ScalarValues::*;

	// A send may state a body instead of an argument (SysML.xtext SendNode ends in
	// ActionBody): the payload its body binds is the message sent.
	action communicator {
		attribute number : Integer = 0;

		first start;

		action sender {
			send to counter { in :>> payload = 50; }
		}

		action counter accept n : Integer;

		action recorder {
			assign number := n;
		}

		done end;

		then start sender;
		then sender counter;
		then counter recorder;
		then recorder end;
	}
}
