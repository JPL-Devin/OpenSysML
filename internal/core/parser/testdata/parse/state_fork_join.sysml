state ParallelSplit {
	initial start;
	state ready;
	state working {
		region left {
			initial leftStart;
			state building;
			succession first leftStart then building;
		}
		region right {
			initial rightStart;
			state checking;
			succession first rightStart then checking;
		}
	}
	fork split;
	join sync;
	final done;

	succession first start then ready;
	transition ready to split;
	transition split to building;
	transition split to checking;
	transition building to sync;
	transition checking to sync;
	transition sync to done;
}
