// An autonomous machine that leaves `outer` from `second` and comes back
// through its shallow history: the recorded trace shows the history re-entering
// `second` rather than the `first` its default transition names.
package Test {
    state HistoryRevisit {
        attribute departures : Integer = 0;

        initial start;
        state outer {
            state first;
            state second;
            history resume;
        }
        state away;
        final done;

        transition start to first;
        transition first to second;
        transition second to away if departures == 0 do {
            departures = departures + 1;
        };
        transition second to done if departures > 0;
        transition away to resume;
        transition resume to first;
    }
}
