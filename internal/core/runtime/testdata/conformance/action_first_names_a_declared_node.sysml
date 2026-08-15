package test {
	// `first s1 then s2;` names the node the flow starts at: s1 runs, then s2. The
	// `first` end is not an initial node of its own, so s1 is not skipped.
	action seq {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		action s1 assign x := 7;
		action s2 assign y := 9;

		first s1 then s2;
	}
}
