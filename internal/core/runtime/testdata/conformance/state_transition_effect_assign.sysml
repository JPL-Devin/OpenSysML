package test {
    // A transition's effect may be written as a bare statement (SysML 7.19.4
    // TransitionUsage effect behavior), terminated by the ';' that terminates the
    // transition: the compact `to` spelling takes exactly one ';', and `counter`
    // ends at 1 only if the effect ran.
    state TestTransitionEffectAssign {
        attribute counter : Integer = 0;

        initial init;
        state active;
        final done;

        succession first init then active;
        transition active to done do assign counter := counter + 1;
    }
}
