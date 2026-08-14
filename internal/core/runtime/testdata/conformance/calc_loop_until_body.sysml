package test {
	// `loop … until` tests its condition after each iteration, so the body runs
	// at least once.
	calc halve {
		in n: Integer;
		attribute i : Integer = n;
		loop {
			i = i / 2;
		} until i <= 1;
		return : Integer = i;
	}
}
