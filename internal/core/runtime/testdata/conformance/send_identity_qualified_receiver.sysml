// A receiver addressed by a qualified name is the element that name resolves to,
// not a path through the sender's features: `::` separates namespaces where `.`
// chains features, so the machine reaches `heard` on the signal it sent itself.
package SendIdentityQualified {
	item def Ping;

	part def Node {
		state Machine {
			initial start;

			state waiting {
				entry send Ping to SendIdentityQualified::Node::Machine;
			}
			final heard;

			start then waiting;
			transition waiting to heard when Ping;
		}
	}

	part solo : Node;
}
