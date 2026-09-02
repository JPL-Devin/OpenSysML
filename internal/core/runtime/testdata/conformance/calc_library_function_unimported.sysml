// A Kernel Function Library function called by its bare name without an import
// of its package is in force; an OpenSysML extension function is not.
package test {
	private import ScalarValues::*;

	part def Probe {
		attribute root = sqrt(16.0);
		attribute magnitude = abs(-2);
		attribute count = size(root);
		attribute growth = exp(1.0);
	}
}
