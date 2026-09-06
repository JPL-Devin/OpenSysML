package M {
	private import ScalarValues::*;
	private import Quantities::*;
	private import MeasurementReferences::*;
	private import SI::*;
	private import QuantityCalculations::*;
	attribute q : ISQ::LengthValue = 3 [km];
	attribute u : MeasurementUnit = m;
	attribute speed = m / s;
	calc def ToUnit { in x : ScalarQuantityValue; in target : MeasurementUnit; return : ScalarQuantityValue = ConvertQuantity(x, target); }
	calc toUnit : ToUnit;
}
