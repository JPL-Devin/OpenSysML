package test {
	state Machine {
		initial init;
		state Running;
		final done;

		init then Running;
		Running then done;
	}
}
