state def RegionChoice {
	attribute mode : Integer = 2;

	region left {
		entry; then lstart;
		state lstart;
		state lidle;
		state lfast;
		state lslow;

		succession first lstart then lidle;
		transition first lidle then pick;
	}

	region right {
		entry; then rstart;
		state rstart;
		state rwatch;

		succession first rstart then rwatch;
	}

	choice pick;

	transition first pick if mode == 2 then lfast;
	transition first pick then lslow;
}
