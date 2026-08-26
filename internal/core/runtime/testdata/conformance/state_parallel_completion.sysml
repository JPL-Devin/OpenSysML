package ParallelCompletion {
    state def Machine parallel {
        state left {
            entry; then lstart;
            state lstart;
            final ldone;
            transition first lstart when First then ldone;
        }
        state right {
            entry; then rstart;
            state rstart;
            final rdone;
            transition first rstart when Second then rdone;
        }
    }
}
