package test {
	// Nested invocation: the outer calc's result invokes another calc twice, so
	// the trace fixes the order the nested invocations are evaluated in.
	calc combine {
		in a: Integer;
		in b: Integer;
		return double(a) + double(b);
	}

	calc double {
		in x: Integer;
		return x * 2;
	}
}
