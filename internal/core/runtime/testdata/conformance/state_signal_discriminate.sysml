package SignalTest {
    // P1 event-matching contract: a state with two signal-triggered transitions
    // must fire the transition whose trigger matches the injected signal.
    // Injecting sigB must reach pathB (finalState = "done"), NOT pathA.
    state Discriminator {
        initial start;
        state waiting;
        state pathA;
        state pathB;
        final done;

        start then waiting;

        transition waiting to pathA when sigA;
        transition waiting to pathB when sigB;

        transition pathA to done;
        transition pathB to done;
    }
}
