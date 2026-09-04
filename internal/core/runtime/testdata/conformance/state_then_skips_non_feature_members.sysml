package test {
    // A positional `then` between states sequences from the nearest preceding
    // state, passing over the documentation and nested definition written
    // between: the machine visits idle, then running, then done.
    state def Machine {
        attribute log : Integer = 0;

        entry; then idle;
        state idle {
            entry action { assign log := log * 10 + 1; }
        }
        doc /* idle precedes running */
        then state running {
            entry action { assign log := log * 10 + 2; }
        }
        action def Inner;
        comment about idle /* the first state */
        then state done {
            entry action { assign log := log * 10 + 3; }
        }
    }
}
