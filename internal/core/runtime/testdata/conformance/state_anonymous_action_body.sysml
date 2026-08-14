package Test {
    // Entry, do and exit behaviors written as anonymous inline action bodies.
    // Each digit records one statement, so the number shows the ordering: entry
    // loop (3, 2, 1), the do body (8), the nested entry (7) that the region's
    // initial transition reaches one step later, then the exit body (9).
    state def Bodies {
        attribute log : Integer = 0;

        initial start;

        state working {
            entry action {
                attribute k : Integer = 3;
                while k > 0 {
                    assign log := log * 10 + k;
                    assign k := k - 1;
                }
            }
            do action { assign log := log * 10 + 8; }
            exit action { assign log := log * 10 + 9; }

            region inside {
                initial nstart;
                state nested {
                    entry action { assign log := log * 10 + 7; }
                    do action { }
                    exit action { }
                }

                then nstart nested;
            }
        }

        state done;

        then start working;
        then working done;
    }
}
