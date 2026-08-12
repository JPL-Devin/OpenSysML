constraint def BacktrackedConditions {
	in attribute x : Real;
	in attribute constraint : Real;

	// A condition may start with the word `constraint` used as a feature name.
	// The nested-constraint attempt consumes it, finds no body, and must rewind
	// so the condition is read whole.
	assert constraint > 0;
	assert constraint == x;
	assert constraint.x > 0;

	// The nested form with a name and a body is still read as a nested constraint.
	assert constraint positive {
		x > 0
	}

	// Anonymous nested constraint: the attempt rewinds over the missing name only.
	assert constraint {
		x < 10
	}

	// Semicolons are optional between conditions, so a rewound `constraint`
	// followed by another condition reads as two members rather than one.
	assert constraint x > 0;
}
