package test {
	// Same as action_unbraced_body_successions, but the succession edges follow the
	// unbraced-body node directly: `then start bump;` names members of `outer`, so
	// it must not be parsed as a continuation of `bump`'s body.
	action outer {
		attribute total : Real = 0.0;

		first start;

		action bump assign total := total + 1.62;

		then start bump;
		then bump end;

		done end;
	}
}
