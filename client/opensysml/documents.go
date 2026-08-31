package opensysml

import (
	"context"
	"strconv"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Cell is one typed document-query value: Element, String, Int, Real, Bool or
// Infinity. A type switch over them is exhaustive.
type Cell interface {
	isCell()
}

// Element is a model element a document query selected or was bound to, named
// by qualified name.
type Element struct {
	// ID is the element's qualified name, the identity a binding names it by.
	ID string
	// Type is its metamodel type name ("PartUsage", …); answered, never bound.
	Type string
}

// Infinity is an unbounded multiplicity. It is answered, never bound.
type Infinity struct{}

func (Element) isCell()  { /* marker: closed Cell set */ }
func (Infinity) isCell() { /* marker: closed Cell set */ }
func (String) isCell()   { /* marker: closed Cell set */ }
func (Int) isCell()      { /* marker: closed Cell set */ }
func (Real) isCell()     { /* marker: closed Cell set */ }
func (Bool) isCell()     { /* marker: closed Cell set */ }

// String is the element as a binding names it, its qualified name.
func (e Element) String() string { return e.ID }

// String reports an unbounded multiplicity as the notation writes it.
func (Infinity) String() string { return "*" }

// Binding binds one entry parameter of a document query. Several values bind a
// nonscalar parameter.
type Binding struct {
	Parameter string
	Values    []Cell
}

// Bind is a binding of one parameter to the values given.
func Bind(parameter string, values ...Cell) Binding {
	return Binding{Parameter: parameter, Values: values}
}

// Rows is a document query's answer: its projected columns, and one row per
// selected element, both in the order the engine reports.
type Rows struct {
	// Columns are the projected properties in projection order.
	Columns []string
	// Rows are the selected elements and their cells.
	Rows []Row
}

// Row is one selected element and its projected cells, one per column.
type Row struct {
	// Element is the element the row is about.
	Element Element
	// Cells holds each column's values, in column order.
	Cells [][]Cell
}

func (c *client) RunDocumentQuery(
	ctx context.Context,
	model *Model,
	queryID string,
	bindings ...Binding,
) (*Rows, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	req := &pb.RunDocumentQueryRequest{ModelHash: hash, QueryId: queryID}
	for _, binding := range bindings {
		bound := &pb.DocumentQueryBinding{Parameter: binding.Parameter}
		for _, value := range binding.Values {
			sent, err := cellToProto(value)
			if err != nil {
				return nil, err
			}
			bound.Values = append(bound.Values, sent)
		}
		req.Bindings = append(req.Bindings, bound)
	}
	resp, err := c.caller.runDocumentQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	rows := &Rows{Columns: make([]string, 0, len(resp.Columns)), Rows: make([]Row, 0, len(resp.Rows))}
	for _, column := range resp.Columns {
		rows.Columns = append(rows.Columns, column.Name)
	}
	for _, row := range resp.Rows {
		converted := Row{Cells: make([][]Cell, 0, len(row.Cells))}
		if element, ok := cellFromProto(row.Element).(Element); ok {
			converted.Element = element
		}
		for _, cell := range row.Cells {
			values := make([]Cell, 0, len(cell.Values))
			for _, value := range cell.Values {
				values = append(values, cellFromProto(value))
			}
			converted.Cells = append(converted.Cells, values)
		}
		rows.Rows = append(rows.Rows, converted)
	}
	return rows, nil
}

func (c *client) RenderDocument(ctx context.Context, model *Model, documentID string) (string, error) {
	hash, err := c.call(model)
	if err != nil {
		return "", err
	}
	resp, err := c.caller.renderDocument(ctx, &pb.RenderDocumentRequest{ModelHash: hash, DocumentId: documentID})
	if err != nil {
		return "", err
	}
	return resp.Markdown, nil
}

// cellToProto marshals a bound value. Infinity is refused here, as the service
// refuses it: queries answer it, nothing binds it.
func cellToProto(cell Cell) (*pb.DocumentValue, error) {
	switch value := cell.(type) {
	case nil:
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "a binding carries no value"}
	case Element:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_ElementId{ElementId: value.ID}}, nil
	case String:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: string(value)}}, nil
	case Int:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: int64(value)}}, nil
	case Real:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: float64(value)}}, nil
	case Bool:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_BoolValue{BoolValue: bool(value)}}, nil
	case Infinity:
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "infinity is answered by queries, not bound to them",
		}
	default:
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "unknown document value kind"}
	}
}

func cellFromProto(value *pb.DocumentValue) Cell {
	switch kind := value.GetKind().(type) {
	case *pb.DocumentValue_ElementId:
		return Element{ID: kind.ElementId, Type: value.GetElementType()}
	case *pb.DocumentValue_StringValue:
		return String(kind.StringValue)
	case *pb.DocumentValue_IntValue:
		return Int(kind.IntValue)
	case *pb.DocumentValue_RealValue:
		return Real(kind.RealValue)
	case *pb.DocumentValue_BoolValue:
		return Bool(kind.BoolValue)
	case *pb.DocumentValue_Infinity:
		return Infinity{}
	default:
		return nil
	}
}

// CellText renders one cell value as a report writes it, the way the CLI's
// document tables do.
func CellText(cell Cell) string {
	switch value := cell.(type) {
	case nil:
		return ""
	case Element:
		return value.ID
	case String:
		return string(value)
	case Int:
		return strconv.FormatInt(int64(value), 10)
	case Real:
		return strconv.FormatFloat(float64(value), 'g', -1, 64)
	case Bool:
		return strconv.FormatBool(bool(value))
	case Infinity:
		return "*"
	default:
		return ""
	}
}
