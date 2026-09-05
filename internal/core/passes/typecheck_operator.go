package passes

// Codes and messages of the pilot's three operator-expression rules
// (validateOperatorExpression{BracketOperator,Quantity,CastConformance}).
const (
	codeBracketOperator = "bracket-operator"
	codeQuantityUnit    = "quantity-unit"
	codeCastConformance = "cast-conformance"

	msgBracketOperator = "`x[i]` is not an index in KerML: `[` invokes BaseFunctions::'[', which the kernel library leaves abstract; index a sequence with `x#(i)`"
	msgQuantityUnit    = "the unit of a quantity must be a measurement reference, found %s: write a unit such as `[m]` or name a feature typed by MeasurementUnit or another measurement reference"
	msgCastConformance = "cast argument is typed by %s, unrelated to the target %s: neither type specializes the other, so the cast selects no value"
)
