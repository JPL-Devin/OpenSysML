package ActionExecutorDemo {
    import ScalarValues::*;

    // A calculation an action can bind an output from, with no control flow.
    action def SimpleAction {
        in x : Real;
        out result : Real;

        bind result = x * 2.0;
    }

    // Sequential: start -> compute -> end.
    action sequential {
        attribute result : Integer = 0;

        first start;
        action compute { assign result := 42 * 2; }
        done finish;

        then start compute;
        then compute finish;
    }

    // Fork and join. A fork duplicates control, not values: both branches are
    // steps of the one performance, so both assignments are visible at the join.
    action forkJoin {
        attribute task1 : Integer = 0;
        attribute task2 : Integer = 0;
        attribute task3 : Integer = 0;

        first start;
        fork split;
        action left { assign task1 := 10; }
        action middle { assign task2 := 20; }
        action right { assign task3 := 30; }
        join sync;
        done finish;

        then start split;
        then split left;
        then split middle;
        then split right;
        then left sync;
        then middle sync;
        then right sync;
        then sync finish;
    }

    // Decision with an else branch: the first guard that holds is taken.
    action conditional {
        attribute x : Integer = 15;
        attribute taken : Integer = 0;

        first start;
        action pathA { assign taken := 1; }
        action pathB { assign taken := 2; }
        done finish;

        then start check;
        then pathA finish;
        then pathB finish;

        decide check;
        if x > 10 then pathA;
        else pathB;
    }
}
