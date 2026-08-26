package test {
	state Machine {
		entry; then init;
		state init;
		state Running;
		final done;

		succession first init then Running;
		succession first Running then done;
	}
}
