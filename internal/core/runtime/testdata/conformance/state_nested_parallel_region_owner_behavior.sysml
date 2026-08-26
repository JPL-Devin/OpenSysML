package NestedParallelRegionOwner {
    private import ScalarValues::*;

    state def Machine {
        attribute worked : Integer = 0;

        initial start;
        state start;
        state outer parallel {
            state a {
                defer Ping;
                do action work {
                    assign worked := worked + 1;
                }
                entry; then a1;
                state a1;
            }
            state b {
                entry; then b1;
                state b1;
            }
        }
        state done;
        final finished;

        succession first start then outer;
        transition outer to done when Leave;
        transition done to finished when Ping;
    }
}
