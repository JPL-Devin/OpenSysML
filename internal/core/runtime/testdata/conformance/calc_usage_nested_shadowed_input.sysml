package test {
	import ScalarValues::*;

	calc def Split {
		in n : Integer;
		out low = n - 1;
		out high = n + 1;
	}

	// The nested usage's input has the name of the parameter it binds from, which
	// shadows it the way a nested binding does elsewhere: `in n = n` reads the
	// enclosing `n` rather than depending on itself.
	calc def Span {
		in n : Integer;
		calc s : Split {
			in n = n;
		}
		out width = s.high - s.low;
		out top = s.high;
	}

	calc spanOfFive : Span {
		in n = 5;
	}

	// The same nested usage read from another invocation, whose inputs differ, so
	// it runs again rather than answering with the first invocation's values.
	calc spanOfTen : Span {
		in n = 10;
	}

	attribute tenTop = spanOfTen.top;
	attribute tenWidth = spanOfTen.width;
}
