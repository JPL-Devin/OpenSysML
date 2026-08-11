package UnbracedActionBodyBoundary {
	// An unbraced action body is one statement plus its `then`-chained
	// continuations; the following `done`/`then` members belong to `tally`,
	// not to `bump`.
	action tally {
		attribute total = 0.0;
		first start;
		action bump assign total := total + 1.62;
		done end;
		then start bump;
		then bump end;
	}

	// Same boundary when the succession edges directly follow the unbraced body:
	// `then start bump;` names members of `edgesFirst`, so it is not a continuation.
	action edgesFirst {
		attribute total = 0.0;
		first start;
		action bump assign total := total + 1.62;
		then start bump;
		then bump end;
		done end;
	}

	// `then` chained continuations stay inside the unbraced body.
	action chained {
		attribute x = 0;
		attribute y = 0;
		action steps assign x := 1; then assign y := 2;
	}
}
