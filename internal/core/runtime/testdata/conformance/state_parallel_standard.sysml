package ParallelStandard {
	state def Machine parallel {
		state left {
			entry; then lstart;
			state lstart;
			state ldone;
			lstart then ldone;
		}
		state right {
			entry; then rstart;
			state rstart;
			state rdone;
			rstart then rdone;
		}
	}
}
