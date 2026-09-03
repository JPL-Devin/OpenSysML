package test {
	private import ScalarValues::*;

	// A binding at an undirected attribute of a node is an equality kept at both ends of
	// the node's performance: the node reads the enclosing feature's value when it begins,
	// and what it holds when the node ends is carried back where the node changed it — to
	// the enclosing feature, or on to the pin of a node downstream. A node that leaves the
	// attribute as it found it writes nothing back; one whose other end holds no value
	// yet gives that end its first value.
	action outer {
		attribute base : Integer = 1;
		attribute kept : Integer = 7;
		attribute fresh : Integer;
		out attribute seen : Integer;
		out attribute unchanged : Integer;
		out attribute given : Integer;
		out attribute downstream : Integer;

		bind acc.total = base;
		bind acc.k = kept;
		bind acc.total = next.n;
		bind giver.v = fresh;

		first start;
		then action acc {
			attribute total : Integer;
			attribute k : Integer;
			assign total := total + 41;
		}
		then action next { attribute n : Integer; assign downstream := n * 2; }
		then action giver { attribute v : Integer; assign v := 9; }
		then action fin {
			assign seen := base;
			assign unchanged := kept;
			assign given := fresh;
		}
		then done;
	}
}
