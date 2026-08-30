// Package queryexec executes immutable document-query plans.
package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ValueKind classifies one scalar query value.
type ValueKind string

const (
	ValueElement  ValueKind = "element"
	ValueString   ValueKind = "string"
	ValueInteger  ValueKind = "integer"
	ValueReal     ValueKind = "real"
	ValueBoolean  ValueKind = "boolean"
	ValueInfinity ValueKind = "infinity"
)

// Value is one immutable scalar carried through query execution.
type Value struct {
	kind    ValueKind
	element *symbols.Symbol
	text    string
	integer int64
	real    float64
	boolean bool
	origin  provenance.Origin
}

// ElementValue constructs an element value with declaration provenance.
func ElementValue(sym *symbols.Symbol) Value {
	return Value{kind: ValueElement, element: sym, origin: provenance.Symbol(sym)}
}

// StringValue constructs a string value.
func StringValue(value string) Value {
	return Value{kind: ValueString, text: value}
}

// IntegerValue constructs an integer value.
func IntegerValue(value int64) Value {
	return Value{kind: ValueInteger, integer: value}
}

// RealValue constructs a real value.
func RealValue(value float64) Value {
	return Value{kind: ValueReal, real: value}
}

// BooleanValue constructs a Boolean value.
func BooleanValue(value bool) Value {
	return Value{kind: ValueBoolean, boolean: value}
}

func valueAt(value Value, origin provenance.Origin) Value {
	value.origin = origin
	return value
}

// Kind returns the value's scalar kind.
func (v Value) Kind() ValueKind { return v.kind }

// Element returns the value's element and whether it is an element value.
func (v Value) Element() (*symbols.Symbol, bool) {
	return v.element, v.kind == ValueElement && v.element != nil
}

// String returns the value's string and whether it is a string value.
func (v Value) String() (string, bool) { return v.text, v.kind == ValueString }

// Integer returns the value's integer and whether it is an integer value.
func (v Value) Integer() (int64, bool) { return v.integer, v.kind == ValueInteger }

// Real returns the value's real and whether it is a real value.
func (v Value) Real() (float64, bool) { return v.real, v.kind == ValueReal }

// Boolean returns the value's Boolean and whether it is a Boolean value.
func (v Value) Boolean() (bool, bool) { return v.boolean, v.kind == ValueBoolean }

// Origin returns the source declaration behind the value.
func (v Value) Origin() provenance.Origin { return v.origin }

// Bindings supplies named values to an entry query.
type Bindings map[string][]Value

// Column describes one ordered projected property.
type Column struct {
	name   string
	origin provenance.Origin
}

// Name returns the projected property name.
func (c Column) Name() string { return c.name }

// Origin returns the query expression that projected the column.
func (c Column) Origin() provenance.Origin { return c.origin }

// Cell is one immutable projected value sequence.
type Cell struct {
	values []Value
	origin provenance.Origin
}

// Values returns an independent copy of the cell values.
func (c Cell) Values() []Value { return append([]Value(nil), c.values...) }

// Origin returns the selected model element behind the cell.
func (c Cell) Origin() provenance.Origin { return c.origin }

// Row retains the selected element and its ordered projected cells.
type Row struct {
	element Value
	cells   []Cell
}

// Element returns the selected element.
func (r Row) Element() Value { return r.element }

// Cells returns an independent copy of the row's projected cells.
func (r Row) Cells() []Cell {
	out := make([]Cell, len(r.cells))
	for i, cell := range r.cells {
		out[i] = Cell{values: cell.Values(), origin: cell.origin}
	}
	return out
}

// Origin returns the selected element's declaration provenance.
func (r Row) Origin() provenance.Origin { return r.element.origin }

// RowSet is an immutable ordered query result.
type RowSet struct {
	columns []Column
	rows    []Row
	origin  provenance.Origin
}

// Columns returns an independent copy of the projected columns.
func (r *RowSet) Columns() []Column {
	if r == nil {
		return nil
	}
	return append([]Column(nil), r.columns...)
}

// Rows returns an independent copy of the result rows.
func (r *RowSet) Rows() []Row {
	if r == nil {
		return nil
	}
	out := make([]Row, len(r.rows))
	for i, row := range r.rows {
		out[i] = Row{element: row.element, cells: row.Cells()}
	}
	return out
}

// Origin returns the entry query declaration.
func (r *RowSet) Origin() provenance.Origin {
	if r == nil {
		return provenance.Origin{}
	}
	return r.origin
}
