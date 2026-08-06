package CallTriggers {
    state Controller {
        initial start;
        state idle {
            accept brake() then stopping;
        }
        state stopping;
        state cruising;

        start then idle;

        transition idle to cruising accept setSpeed(value) if value > 0;
    }
}
