package ParallelTimed {
	state def Machine parallel {
		state left {
			entry; then waiting;
			state waiting {
				accept after 5 then ldone;
			}
			state ldone;
		}
		state right {
			entry; then active;
			state active;
		}
	}
}
