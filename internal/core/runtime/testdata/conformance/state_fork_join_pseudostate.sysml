package ForkJoinTest {
    state ParallelWork {
        attribute leftDone : Integer = 0;
        attribute rightDone : Integer = 0;

        initial init;
        state ready;
        state working {
            region left {
                initial leftStart;
                state building { entry { assign leftDone := 1; } }
                then leftStart building;
            }
            region right {
                initial rightStart;
                state checking { entry { assign rightDone := 1; } }
                then rightStart checking;
            }
        }
        fork split;
        join sync;
        final done;

        init then ready;
        transition ready to split;
        transition split to building;
        transition split to checking;
        transition building to sync;
        transition checking to sync;
        transition sync to done;
    }
}
