package test {
	private import ScalarValues::*;

	// A merge's body belongs to the traversal, not to every arrival: the token
	// whose succession a guard prunes leaves the merge open, so the merge fires
	// once and states its steps once.
	action orJoin {
		attribute ready : Boolean = false;
		attribute mergeRuns : Integer = 0;
		attribute passed : Integer = 0;

		first start;
		fork split;
		action slow;
		action slower { assign ready := true; }
		merge gate { assign mergeRuns := mergeRuns + 1; }
		action tail { assign passed := mergeRuns; }
		done finish;

		then start split;
		then split gate;
		then split slow;
		then slow slower;
		then slower gate;
		then gate tail if ready;
		then tail finish;
	}
}
