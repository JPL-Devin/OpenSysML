state def RegionChoice {
	attribute mode : Integer = 2;

	region left {
		initial lstart;
		state lidle;
		state lfast;
		state lslow;

		then lstart lidle;
		transition lidle to pick;
	}

	region right {
		initial rstart;
		state rwatch;

		then rstart rwatch;
	}

	choice pick;

	transition pick to lfast if mode == 2;
	transition pick to lslow;
}
