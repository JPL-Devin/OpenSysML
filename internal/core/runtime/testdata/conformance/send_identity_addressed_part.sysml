// A send addressing a part delivers to that object and no other, even where the
// sender accepts the same signal: alpha addresses worker and stays waiting with
// its own accept unfired, while worker takes what it addressed to itself.
package SendIdentityPart {
	item def Ping;

	part def Node {
		state Listen {
			initial start;

			state waiting {
				entry send Ping() to worker;
			}
			state heard;

			start then waiting;

			transition first waiting
				accept Ping
				then heard;
		}
	}

	part alpha : Node;
	part worker : Node;
}
