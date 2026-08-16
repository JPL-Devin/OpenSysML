// A send through a port joined to nothing is reported where it was written,
// rather than dropping the message and leaving the accept to time out.
package UnroutableSend {
	action ship {
		port lonely;
		first start;
		action sender { send 42 via lonely; }
		done end;
		then start sender;
		then sender end;
	}
}
