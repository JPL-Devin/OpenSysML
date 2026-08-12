package MemberThenOrderTest {
	// A `then` attached to a state declares a completion transition, so the
	// machine follows it rather than declaration order: `fourth` is entered after
	// `second` only because of the keyword between them.
	state Sequenced {
		initial start;

		state second;
		then state fourth;

		state third;
		final done;

		start then second;
		fourth then third;
		third then done;
	}
}
