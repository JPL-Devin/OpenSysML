package test {
	private import ScalarValues::*;
	
	action communicator {
		attribute receivedValue : Integer = 0;
		first start;
		
		// Send message
		action sender {
			send 42 to receiver;
		}
		
		// Accept message (binds to msg parameter)
		action receiver accept msg : Integer;
		
		// Process accepted message
		action processor {
			assign receivedValue := msg;
		}
		
		done end;
		
		then start sender;
		then sender receiver;
		then receiver processor;
		then processor end;
	}
}
