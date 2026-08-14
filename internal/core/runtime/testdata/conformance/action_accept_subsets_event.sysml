package test {
	import ScalarValues::*;

	item def Ping;

	// An accept whose payload subsets a declared event waits for an occurrence of
	// that one event: `send shutDown()` names the event feature, and the accept
	// subsetting it consumes the message it makes.
	action controller {
		attribute stopped : Integer = 0;
		item shutDown : Ping;

		first start;

		action finish {
			send shutDown() to interrupt;
		}

		action interrupt accept :> shutDown;

		action record {
			assign stopped := 1;
		}

		done end;

		then start finish;
		then finish interrupt;
		then interrupt record;
		then record end;
	}
}
