package Test {
	private import ScalarValues::*;
	action addFive {
		attribute result : Integer = 0;
		first start;
		action inner {
			assign result := result + 5;
		}
		done end;
		then start inner;
		then inner end;
	}
	action noStart {
		attribute result : Integer = 0;
	}
	state Machine {
		initial init;
		state Running;
		final done;

		init then Running;
		Running then done;
	}
}
