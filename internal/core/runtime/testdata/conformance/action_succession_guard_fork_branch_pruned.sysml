package test {
	// A guard on a branch leaving a fork prunes that branch: only the branches
	// whose guard holds get a token, and the rest of the fork runs as before.
	action launch {
		attribute armed : Integer = 0;
		attribute ignited : Integer = 0;
		attribute logged : Integer = 0;

		first start;
		fork split;
		action ignite assign ignited := 1;
		action log assign logged := 1;
		done end;

		then start split;
		then split ignite if armed > 0;
		then split log;
		then ignite end;
		then log end;
	}
}
