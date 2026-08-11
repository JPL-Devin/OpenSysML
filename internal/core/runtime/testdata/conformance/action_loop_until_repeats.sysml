package test {
	import ScalarValues::*;

	// The post-condition is re-tested after every iteration, so the loop keeps
	// going until the value it watches reaches the bound.
	action counter {
		attribute total : Integer = 0;
		first start;

		action accumulate {
			loop {
				assign total := total + 2;
			} until total >= 6;
		}

		done end;

		then start accumulate;
		then accumulate end;
	}
}
