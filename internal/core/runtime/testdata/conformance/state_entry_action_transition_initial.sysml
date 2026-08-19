package test {
    // A transition out of the machine's named entry action names the state it
    // starts in (SysML 7.19.3), the same as the anonymous `entry; then off;`.
    state Modes {
        attribute seen : Integer = 0;

        entry action start { }
        transition start then off;

        state off {
            entry assign seen := seen + 1;
        }
        final done;

        off then done;
    }
}
