package test {
	private import ScalarValues::*;

	// A pre-condition loop accumulates until its condition fails, so the loop
	// runs a number of times the model does not spell out.
	action counter {
		attribute total : Integer = 0;
		first start;

		action accumulate {
			while total < 5 {
				assign total := total + 1;
			}
		}

		done end;

		then start accumulate;
		then accumulate end;
	}
}
