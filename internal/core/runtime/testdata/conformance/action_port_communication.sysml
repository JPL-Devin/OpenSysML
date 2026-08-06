package test {
	import ScalarValues::*;

	action communicate {
		attribute result : Integer = 0;
		port outPort;
		port inPort;
		connect outPort to inPort;

		first start;
		action sender {
			send 100 via outPort;
		}
		action receiver accept msg : Integer via inPort;
		action processor {
			assign result := msg;
		}
		done end;
		then start sender;
		then sender receiver;
		then receiver processor;
		then processor end;
	}
}
