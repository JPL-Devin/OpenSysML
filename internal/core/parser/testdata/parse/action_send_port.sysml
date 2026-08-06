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
		action notifier {
			send 100 to reader;
		}
		action reader accept msg : Integer via inPort;
		done end;
		then start sender;
		then sender notifier;
		then notifier reader;
		then reader end;
	}
}
