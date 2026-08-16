// ComplexFunctions::i is the imaginary unit, read as an imported name rather
// than a qualified one: import makes a library feature readable unqualified.
package test {
	import ScalarValues::*;
	import ComplexFunctions::*;

	calc def imaginaryUnit {
		return : Complex = i;
	}
}
