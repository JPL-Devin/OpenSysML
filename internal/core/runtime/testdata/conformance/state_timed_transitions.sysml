package test {
    // `accept at t` names an instant; `accept after d` an offset from entry.
    // Both timed transitions leave `working`, which is entered at time 10: the
    // `at 5` instant is already past and so fires immediately, while `after 3`
    // would fire at 13. Were `at` read as an offset it would fire at 15 and the
    // machine would sit in `later` instead of reaching `done`.
    state TimedTransitions {
        entry; then init;
        state init;
        state waiting {
            accept after 10 then working;
        }
        state working {
            accept at 5 then instant;
            accept after 3 then later;
        }
        state instant {
            accept after 1 then done;
        }
        state later;
        final done;

        succession first init then waiting;
    }
}
