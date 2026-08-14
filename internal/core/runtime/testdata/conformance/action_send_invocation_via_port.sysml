package test {
	import ScalarValues::*;

	item def Data {
		attribute value : Integer;
	}

	// `send Data(reading) via commPort` sends the signal the invocation names,
	// carrying its argument, and the accept on the far end of the port binds that
	// payload to its parameter.
	action telemetry {
		attribute reading : Integer = 7;
		attribute received : Integer = 0;
		port commPort;
		port groundPort;
		connect commPort to groundPort;

		first start;

		action transmit {
			send Data(reading) via commPort;
		}

		action receive accept msg : Data via groundPort;

		action record {
			assign received := msg;
		}

		done end;

		then start transmit;
		then transmit receive;
		then receive record;
		then record end;
	}
}
