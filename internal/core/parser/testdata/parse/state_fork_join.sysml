state ParallelSplit {
	initial start;
	state ready;
	state working {
		region left {
			initial leftStart;
			state building;
			then leftStart building;
		}
		region right {
			initial rightStart;
			state checking;
			then rightStart checking;
		}
	}
	fork split;
	join sync;
	final done;

	start then ready;
	transition ready to split;
	transition split to building;
	transition split to checking;
	transition building to sync;
	transition checking to sync;
	transition sync to done;
}
