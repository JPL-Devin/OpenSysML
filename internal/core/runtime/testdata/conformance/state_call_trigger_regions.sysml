package CallTriggerRegionTest {
    // One invocation is broadcast to every orthogonal region: the regions that
    // have a matching call trigger each fire one transition, run-to-completion.
    state def Dashboard parallel {
        state display {
            initial start;
            state Dimmed;
            state Lit;

            succession first start then Dimmed;
            transition Dimmed to Lit accept wake();
        }

        state audio {
            initial start;
            state Muted;
            state Chiming;

            succession first start then Muted;
            transition Muted to Chiming accept wake();
            transition Chiming to Muted accept mute();
        }
    }
}
