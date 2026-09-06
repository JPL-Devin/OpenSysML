// What the runtime cannot compute about a measurement reference fails by name.
// A conversion between dimensions is incommensurable; a reference is neither a
// number nor an addend; a declaration's own members (unitConversion,
// quantityDimension) are not what the value holds; a vector quantity whose axes
// differ has no single reference; the coordinate frames the library declares
// have no runtime value, so `transform` is not evaluable; and `outer` declares
// a VectorQuantityValue result no outer product is.
package test {
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import QuantityCalculations::*;

	part def Block {
		attribute side : LengthValue = 3 [m];
		attribute inSeconds = ConvertQuantity(side, s);
		attribute scaled = m * 3;
		attribute summed = m + m;
		attribute conversion = m.unitConversion;
		attribute dimension = m.quantityDimension;
		attribute notAUnit = ConvertQuantity(side, 3);
		attribute mixed = VectorCalculations::'['((1.0, 2.0), (m, s));
		attribute turned = VectorCalculations::transform(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((0.0, 1.0)) [m]);
		attribute spread = VectorCalculations::outer((1.0, 2.0), (3.0, 4.0));
	}
}
