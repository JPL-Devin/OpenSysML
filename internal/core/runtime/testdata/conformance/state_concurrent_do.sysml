package Test {
    // Two orthogonal regions whose do behaviors run while their states are
    // active. Each digit records one do action, so the resulting number shows
    // the interleaving: left enters one step before right, then the two share
    // the machine one action at a time (1,2,4,3,5,6). A do behavior run to its
    // end at entry would give 1,2,3,4,5,6 instead.
    state def Interleave {
        region left {
            initial lstart;
            state lwork {
                do {
                    seq = seq * 10 + 1;
                    seq = seq * 10 + 2;
                    seq = seq * 10 + 3;
                }
            }
            state ldone;

            then lstart lwork;
            then lwork ldone;
        }

        region right {
            initial rstart;
            state rwork {
                do {
                    seq = seq * 10 + 4;
                    seq = seq * 10 + 5;
                    seq = seq * 10 + 6;
                }
            }
            state rdone;

            then rstart rwork;
            then rwork rdone;
        }

        attribute seq : Integer = 0;
    }
}
