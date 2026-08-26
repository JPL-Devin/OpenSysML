package Test {
    // An anonymous do body is a single action the machine runs to completion in
    // one round, so the two regions cannot interleave inside it: this gives
    // 123456, where state_concurrent_do's one-action-per-statement `do { … }`
    // form gives 124356 for the same digits.
    state def AtomicDo parallel {
        attribute seq : Integer = 0;

        state left {
            initial lstart;
            state lwork {
                do action {
                    assign seq := seq * 10 + 1;
                    assign seq := seq * 10 + 2;
                    assign seq := seq * 10 + 3;
                }
            }
            state ldone;

            succession first lstart then lwork;
            succession first lwork then ldone;
        }

        state right {
            initial rstart;
            state rwork {
                do action {
                    assign seq := seq * 10 + 4;
                    assign seq := seq * 10 + 5;
                    assign seq := seq * 10 + 6;
                }
            }
            state rdone;

            succession first rstart then rwork;
            succession first rwork then rdone;
        }
    }
}
