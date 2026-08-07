package Deferral {
    state Controller {
        initial start;
        state busy {
            defer Ping, setSpeed(value);
        }
        state ready;
        final done;

        start then busy;

        transition busy to ready when Go;
        transition ready to done when Ping;
    }
}
