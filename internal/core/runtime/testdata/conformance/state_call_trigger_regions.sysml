package CallTriggerRegionTest {
    // One invocation is broadcast to every orthogonal region: the regions that
    // have a matching call trigger each fire one transition, run-to-completion.
    state def Dashboard {
        region display {
            initial start;
            state Dimmed;
            state Lit;

            then start Dimmed;
            transition Dimmed to Lit accept wake();
        }

        region audio {
            initial start;
            state Muted;
            state Chiming;

            then start Muted;
            transition Muted to Chiming accept wake();
            transition Chiming to Muted accept mute();
        }
    }
}
