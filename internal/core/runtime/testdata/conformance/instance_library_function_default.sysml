// A feature default and a carried constraint that compute with library
// functions, evaluated when the part is instantiated.
package test {
	import ScalarValues::*;
	import RealFunctions::*;
	import TrigFunctions::*;

	part def Tank {
		attribute area = 16.0;
		attribute side = sqrt(area);
		attribute tilt = cos(0.0) * 2.0 ** 2;

		constraint sideFits {
			side <= tilt
		}

		constraint sideExceedsTilt {
			side > tilt
		}
	}
}
