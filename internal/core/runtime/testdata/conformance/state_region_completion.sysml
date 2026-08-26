package RegionCompletion {
    state def Machine {
        region left {
            entry; then lstart;
            state lstart;
            final ldone;
            transition first lstart when First then ldone;
        }
        region right {
            entry; then rstart;
            state rstart;
            final rdone;
            transition first rstart when Second then rdone;
        }
    }
}
