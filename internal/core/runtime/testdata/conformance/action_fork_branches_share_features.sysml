package test {
	// A fork duplicates control, not values: both branches are steps of the one
	// performance of `par`, so both assignments are visible when it completes.
	action par {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		first start;
		fork split;
		action left { assign x := 1; }
		action right { assign y := 2; }
		join sync;
		done end;

		then start split;
		then split left;
		then split right;
		then left sync;
		then right sync;
		then sync end;
	}
}
