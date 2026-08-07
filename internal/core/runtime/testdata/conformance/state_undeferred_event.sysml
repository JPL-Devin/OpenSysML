// The same machine without the deferral: the Ping arriving while `busy` is
// active is dropped where no transition handles it, so the machine stops in
// `ready`. This is what makes state_deferred_event evidence of deferral rather
// than of queue ordering.
package Test {
    state UndeferredEvent {
        initial start;
        state busy;
        state ready;
        final done;

        start then busy;

        transition busy to ready when Go;
        transition ready to done when Ping;
    }
}
