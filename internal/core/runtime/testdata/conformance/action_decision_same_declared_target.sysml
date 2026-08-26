package test {
	private import ScalarValues::*;

	action guarded {
		attribute c : Boolean = true;
		attribute result : Integer = 0;

		first start;
		action shared {
			assign result := 42;
		}
		decide check;

		then start check;
		then check shared;
		then shared done;
		if c then shared;
		else shared;
	}
}
