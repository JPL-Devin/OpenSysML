// Calc parameters with defaults, a nested calc invocation in the result
// expression, and a bodyless typed calc usage that inherits both.
package library {
	calc def Scale {
		in x : Integer;
		in factor : Integer = 2;
		return x * factor;
	}

	calc combine {
		in a : Integer;
		in b : Integer;
		return double(a) + double(b);
	}

	calc double {
		in x : Integer;
		return x * 2;
	}

	calc scaled : Scale;
}
