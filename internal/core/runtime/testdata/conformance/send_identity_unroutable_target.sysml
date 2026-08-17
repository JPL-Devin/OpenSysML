// A target whose path reaches no port of any object the sender can address is
// reported where it was written, rather than delivered to whatever else carries
// the last segment's name.
package SendIdentityUnroutable {
	import ScalarValues::*;

	item def Ping;

	part def Node {
		attribute count : Integer = 0;
	}

	part alpha : Node;

	action ship {
		first start;
		action sender { send Ping() to alpha.count; }
		done end;
		then start sender;
		then sender end;
	}
}
