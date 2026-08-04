package SignalTest {
    // P1 event-matching contract: an injected signal with no matching outgoing
    // transition must be dropped without firing anything.
    // Injecting sigC (no transition) must leave the machine in `waiting`.
    state Unmatched {
        initial start;
        state waiting;
        state pathA;

        start then waiting;

        transition waiting to pathA when sigA;
    }
}
