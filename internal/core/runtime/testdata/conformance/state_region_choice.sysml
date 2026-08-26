// A choice reached from inside an orthogonal region whose branch stays in that
// region moves only that region; its sibling keeps the state it was in.
package Test {
	state def RegionChoice {
		attribute mode : Integer = 2;

		region left {
			initial lstart;
			state lidle;
			state lfast;
			state lslow;

			succession first lstart then lidle;
			transition lidle to pick;
		}

		region right {
			initial rstart;
			state rwatch;

			succession first rstart then rwatch;
		}

		choice pick;

		transition pick to lfast if mode == 2;
		transition pick to lslow;
	}
}
