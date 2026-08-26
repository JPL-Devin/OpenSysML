package test {
	state Machine {
		initial init;
		state Running;
		final done;
		
		succession first init then Running;
		succession first Running then done;
	}
}
