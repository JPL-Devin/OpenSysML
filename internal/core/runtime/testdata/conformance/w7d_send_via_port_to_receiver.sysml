package W7D {
	action route {
		attribute got : Integer = 0;
		port senderPort;
		port receiverPort;
		connect senderPort to receiverPort;
		first start;
		action sender {
			send 42 via senderPort to receiver;
		}
		action receiver accept value : Integer via receiverPort {
			assign got := value;
		}
		action sibling accept value : Integer via receiverPort {
			assign got := -1;
		}
		done end;
		then start sender;
		then sender receiver;
		then receiver end;
	}
}
