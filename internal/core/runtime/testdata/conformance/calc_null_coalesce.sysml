package test {
	// `??` evaluates its right operand only when the left one is null.
	calc orDefault {
		in n: Integer;
		n ?? 0
	}
}
