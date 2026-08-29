package opensysml

// Value is one evaluated SysML value. It is a sealed sum: the concrete types
// are Int, Real, Bool, String, InstanceID, Sequence, Null, Unset, Quantity and
// EnumLiteral, and a type switch over them is exhaustive.
type Value interface {
	isValue()
}

// Number is a Value that is a numeric magnitude: Int or Real. Integers and
// reals stay apart end to end, so an integer compares exactly.
type Number interface {
	Value
	isNumber()
}

// Int is an integer value.
type Int int64

// Real is a real value.
type Real float64

// Bool is a boolean value.
type Bool bool

// String is a string value.
type String string

// InstanceID refers to an Instance by the id an Instantiation assigned it.
// Ids are per call: they name instances within one answer, nothing beyond it.
type InstanceID int64

// Sequence is an ordered collection of values.
type Sequence []Value

// Null is the absence of a value. Its text is the service's reason when it has
// one ("unsupported: variant selection"), and empty for a plain null.
type Null string

// Unset is a valueless feature of a value type: materialized, holding no
// value. It is reported, never accepted.
type Unset struct{}

// Quantity is a magnitude and the measurement unit it is expressed in, exactly
// as written: 5.4 [km/h] arrives as 5.4 km/h, not converted to base units.
type Quantity struct {
	// Magnitude keeps Int and Real apart, as the rest of Value does.
	Magnitude Number
	// Unit as written ("km/h"), or as an operation composed it ("m/s"); empty
	// for a unit never written down, described by Term alone.
	Unit string
	// Term is what the unit reduces to, which decides commensurability.
	Term *UnitTerm
}

// UnitTerm is a unit's reduction to base units, with its scale kept as an
// unevaluated ratio so an exact conversion stays exact.
type UnitTerm struct {
	ScaleNum float64
	ScaleDen float64
	// Factors are the base units of the reduction, carrying no zero exponents.
	Factors []UnitFactor
}

// UnitFactor is one base unit raised to an exponent.
type UnitFactor struct {
	// UnitID is the FQN of the base unit ("SI::m").
	UnitID string
	// Exponent the base unit is raised to.
	Exponent float64
}

// EnumLiteral is one literal of an enumeration definition. A literal is its
// own identity: two values are the same literal exactly when LiteralID is.
type EnumLiteral struct {
	// LiteralID is the FQN of the literal's declaration ("D::Color::red").
	LiteralID string
	// EnumerationID is the FQN of the enumeration declaring it ("D::Color").
	EnumerationID string
	// Name is the literal as a reader writes it ("Color::red").
	Name string
}

func (Int) isValue()         { /* marker: closed Value set */ }
func (Real) isValue()        { /* marker: closed Value set */ }
func (Bool) isValue()        { /* marker: closed Value set */ }
func (String) isValue()      { /* marker: closed Value set */ }
func (InstanceID) isValue()  { /* marker: closed Value set */ }
func (Sequence) isValue()    { /* marker: closed Value set */ }
func (Null) isValue()        { /* marker: closed Value set */ }
func (Unset) isValue()       { /* marker: closed Value set */ }
func (Quantity) isValue()    { /* marker: closed Value set */ }
func (EnumLiteral) isValue() { /* marker: closed Value set */ }

func (Int) isNumber()  { /* marker: closed Number set */ }
func (Real) isNumber() { /* marker: closed Number set */ }
