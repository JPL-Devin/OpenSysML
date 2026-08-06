package CallTriggerGuardTest {
    // A guard reading an argument the call trigger bound: the invocation matches
    // the trigger but the guard rejects it, so the machine stays in idle.
    state Controller {
        initial start;
        state idle;
        state cruising;

        start then idle;

        transition idle to cruising accept setSpeed(value) if value > 0;
    }
}
