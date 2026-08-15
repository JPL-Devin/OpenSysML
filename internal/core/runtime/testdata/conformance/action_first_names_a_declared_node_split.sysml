package test {
	// The same start written as its own member: `first s1;` marks s1 as the first
	// node, and the succession out of it is written separately.
	action seq {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		action s1 assign x := 7;
		action s2 assign y := 9;

		first s1;
		then s1 s2;
	}
}
