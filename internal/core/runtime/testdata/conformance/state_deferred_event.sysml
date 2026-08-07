// A state defers an event it cannot handle: the Ping arriving while `busy` is
// active is retained rather than dropped, and is delivered once the machine has
// left `busy` for a state that handles it.
package Test {
    state DeferredEvent {
        initial start;
        state busy {
            defer Ping;
        }
        state ready;
        final done;

        start then busy;

        transition busy to ready when Go;
        transition ready to done when Ping;
    }
}
