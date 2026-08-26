package Test {
	private import ScalarValues::*;
	action addFive {
		attribute result : Integer = 0;
		first start;
		action inner {
			assign result := result + 5;
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
	action noStart {
		attribute result : Integer = 0;
	}
	state Machine {
		entry; then init;
		state init;
		state Running;

		succession first init then Running;
		succession first Running then done;
	}
}
