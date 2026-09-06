package S {
	private import ScalarValues::*;
	private import Collections::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import SI::*;
	attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
	attribute v : CartesianVectorValue = VectorOf((3.0, 4.0));
	attribute d : VectorQuantityValue = VectorOf((3.0, 4.0)) [m];
	calc def Length { in x : CartesianVectorValue; return : Real = norm(x); }
	calc length : Length;
}
