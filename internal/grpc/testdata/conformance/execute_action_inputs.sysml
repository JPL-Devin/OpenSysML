package test {
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
}
