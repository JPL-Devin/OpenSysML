package test {
	private import ScalarValues::*;

	action communicate {
		attribute atA : Integer = 0;
		attribute atB : Integer = 0;
		port outPort;
		port inA;
		port inB;
		connect (outPort, inA, inB);

		first start;
		action sender {
			send 100 via outPort;
		}
		action receiverA accept msgA : Integer via inA;
		action recordA {
			assign atA := msgA;
		}
		action receiverB accept msgB : Integer via inB;
		action recordB {
			assign atB := msgB;
		}
		done end;
		then start sender;
		then sender receiverA;
		then receiverA recordA;
		then recordA receiverB;
		then receiverB recordB;
		then recordB end;
	}
}
