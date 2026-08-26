package CallTriggerTest {
    // A call trigger fires on an invocation of the operation it names; a trigger
    // declaring a parameter takes only a call carrying that argument, and binds
    // it so the guard can read it.
    state Controller {
        initial start;
        state idle;
        state braking;
        state cruising;
        final done;

        succession first start then idle;

        transition idle to cruising accept setSpeed(value) if value > 0;
        transition idle to braking accept brake();

        transition cruising to done;
        transition braking to done;
    }
}
