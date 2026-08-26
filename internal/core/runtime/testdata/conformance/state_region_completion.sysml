package RegionCompletion {
    state def Machine {
        region left {
            initial lstart;
            state lstart;
            final ldone;
            transition lstart to ldone when First;
        }
        region right {
            initial rstart;
            state rstart;
            final rdone;
            transition rstart to rdone when Second;
        }
    }
}
