package RegionCompletion {
    state def Machine parallel {
        state left {
            initial lstart;
            state lstart;
            final ldone;
            transition lstart to ldone when First;
        }
        state right {
            initial rstart;
            state rstart;
            final rdone;
            transition rstart to rdone when Second;
        }
    }
}
