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
                succession first leftStart then building;
            }
            region right {
                initial rightStart;
                state checking { entry { assign rightDone := 1; } }
                succession first rightStart then checking;
            }
        }
        fork split;
        join sync;
        final done;

        succession first init then ready;
        transition ready to split;
        transition split to building;
        transition split to checking;
        transition building to sync;
        transition checking to sync;
        transition sync to done;
    }
}
