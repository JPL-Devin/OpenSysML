package C {
	private import ScalarValues::*;
	private import ComplexFunctions::*;
	part def Signal {
		attribute z : Complex = rect(1.5, -2.0);
		attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
	}
}
