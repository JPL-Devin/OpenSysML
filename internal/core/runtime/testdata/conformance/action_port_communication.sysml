package test {
	import ScalarValues::*;
	
	port outPort;
	
	action communicate {
		attribute result : Integer = 0;
		first start;
		
		// Send via port
		action sender {
			send 100 via outPort;
		}
		
		// Accept message
		action receiver accept msg : Integer;
		
		// Process
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
