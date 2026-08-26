state def RegionChoice parallel {
	attribute mode : Integer = 2;

	state left {
		initial lstart;
		state lidle;
		state lfast;
		state lslow;

		succession first lstart then lidle;
		transition lidle to pick;
	}

	state right {
		initial rstart;
		state rwatch;

		succession first rstart then rwatch;
	}

	choice pick;

	transition pick to lfast if mode == 2;
	transition pick to lslow;
}
