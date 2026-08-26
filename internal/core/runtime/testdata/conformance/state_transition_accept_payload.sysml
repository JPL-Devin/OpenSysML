package StateAcceptPayload {
    private import ScalarValues::*;

    item def Warning;

    state Monitor {
        attribute level : Integer = 0;

        initial start;
        state idle;
        state alerting;
        final done;

        succession first start then idle;

        // The accept names its payload, so the value the received occurrence
        // carries is readable by the transition's effect.
        transition first idle
            accept w : Warning
            do assign level := w
            then alerting;

        transition alerting to done;
    }
}
