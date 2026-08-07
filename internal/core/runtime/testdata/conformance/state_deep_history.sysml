// A deep history re-enters the innermost substate that was active when its
// composite state was last exited, not just the outermost one a shallow history
// would restore.
package Test {
    state DeepHistory {
        initial start;
        state outer {
            state middle {
                state inner;
                state innerTwo;
            }
            deep history resume;
        }
        state away;
        final done;

        start then inner;

        transition inner to innerTwo when Next;
        transition innerTwo to away when Leave;
        transition away to resume when Back;
        transition resume to inner;
        transition innerTwo to done when Finish;
    }
}
