package NestedParallelBehavior {
    private import ScalarValues::*;

    state def Machine {
        attribute entered : Integer = 0;
        attribute exited : Integer = 0;

        initial start;
        state start;
        state outer parallel {
            entry action begin {
                assign entered := entered + 1;
            }
            exit action finish {
                assign exited := exited + 1;
            }
            state left {
                entry; then lidle;
                state lidle;
            }
            state right {
                entry; then ridle;
                state ridle;
            }
        }
        state done;

        start then outer;
        transition outer to done when Finish;
    }
}
