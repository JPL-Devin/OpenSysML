package ParallelCompletion {
    state def Machine parallel {
        state left {
            entry; then lstart;
            state lstart;
            final ldone;
            transition lstart to ldone when First;
        }
        state right {
            entry; then rstart;
            state rstart;
            final rdone;
            transition rstart to rdone when Second;
        }
    }
}
