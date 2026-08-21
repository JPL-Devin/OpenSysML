package W7D {
	private import ScalarValues::*;
	action route {
		attribute receiverGot : Integer = 0;
		attribute siblingGot : Integer = 0;
		port senderPort;
		port receiverPort;
		connect senderPort to receiverPort;
		first start;
		action sender {
			send 42 via senderPort to receiver;
			send 7 via senderPort to sibling;
		}
		action receiver accept value : Integer via receiverPort {
			assign receiverGot := value;
		}
		action sibling accept value : Integer via receiverPort {
			assign siblingGot := value;
		}
		fork split;
		join sync;
		done end;
		then start sender;
		then sender split;
		then split receiver;
		then split sibling;
		then receiver sync;
		then sibling sync;
		then sync end;
	}
}
