package test {
	private import ScalarValues::*;

	// Every control node ends in ActionBody (SysML.xtext ForkNode, JoinNode), so it
	// may state steps of its own, run as control reaches the node.
	action gate {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		attribute z : Integer = 0;

		first start;
		fork split { assign x := 1; }
		action left { assign y := x + 1; }
		join sync { assign z := y + 1; }
		done end;

		then start split;
		then split left;
		then left sync;
		then sync end;
	}
}
