package test {
	// A node written with an unbraced body must not swallow the enclosing
	// body's remaining members: `done end;` and the successions below belong to
	// `outer`, so `outer` still runs start -> bump -> end and `total` is 1.62.
	action outer {
		attribute total : Real = 0.0;

		first start;

		action bump assign total := total + 1.62;

		done end;

		then start bump;
		then bump end;
	}
}
