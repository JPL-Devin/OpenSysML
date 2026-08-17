package test {
	// A guard that does not evaluate to a Boolean is reported rather than treated
	// as either verdict: `bool guard[*]` admits no other value.
	action seq {
		attribute x : Integer = 1;
		attribute y : Integer = 0;

		action s1 assign x := 7;
		action s2 assign y := 9;

		first s1 if x + 1 then s2;
	}
}
