package test {
	import ScalarValues::*;

	// A post-condition loop tests after it iterates, so the body runs once even
	// though the condition already holds when the loop is entered.
	action counter {
		attribute total : Integer = 10;
		first start;

		action accumulate {
			loop {
				assign total := total + 1;
			} until total > 0;
		}

		done end;

		then start accumulate;
		then accumulate end;
	}
}
