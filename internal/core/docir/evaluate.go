package docir

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Evaluate evaluates a compiled document plan into an immutable document,
// executing every referenced query through the query execution engine.
func Evaluate(
	plan *docplan.Plan,
	context queryexec.Context,
	options queryexec.Options,
) (*Document, error) {
	if plan == nil {
		return nil, &Error{Kind: ErrorInvalidPlan}
	}
	if context.Index == nil || context.Resolver == nil || context.Model == nil {
		return nil, &Error{Kind: ErrorInvalidContext, Document: plan.Name()}
	}
	e := &evaluator{document: plan.Name(), context: context, options: options}
	content, err := e.evaluateContent(plan.Content())
	if err != nil {
		return nil, err
	}
	return &Document{
		name:    plan.Name(),
		title:   plan.Title(),
		content: content,
		origin:  plan.Origin(),
	}, nil
}

type evaluator struct {
	document string
	context  queryexec.Context
	options  queryexec.Options
}

func (e *evaluator) evaluateContent(planned []docplan.Content) ([]Content, error) {
	content := make([]Content, 0, len(planned))
	for _, node := range planned {
		evaluated, err := e.evaluateNode(node)
		if err != nil {
			return nil, err
		}
		content = append(content, evaluated)
	}
	return content, nil
}

func (e *evaluator) evaluateNode(node docplan.Content) (Content, error) {
	switch node.Kind() {
	case docplan.ContentSection:
		children, err := e.evaluateContent(node.Children())
		if err != nil {
			return Content{}, err
		}
		return Content{
			kind:     ContentSection,
			name:     node.Name(),
			title:    node.Title(),
			children: children,
			origin:   node.Origin(),
		}, nil
	case docplan.ContentParagraph:
		return e.evaluateParagraph(node)
	case docplan.ContentTable:
		return e.evaluateTable(node)
	case docplan.ContentList:
		return e.evaluateList(node)
	default:
		return Content{}, &Error{
			Kind:     ErrorInvalidPlan,
			Document: e.document,
			Content:  node.Name(),
			Origin:   node.Origin(),
		}
	}
}

func (e *evaluator) evaluateParagraph(node docplan.Content) (Content, error) {
	content := Content{
		kind:   ContentParagraph,
		name:   node.Name(),
		origin: node.Origin(),
	}
	if node.Query() == nil {
		content.runs = []TextRun{{text: node.Text(), origin: node.Origin()}}
		return content, nil
	}
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	content.query = node.Query().Entry()
	content.queryOrigin = result.Origin()
	for _, row := range result.Rows() {
		content.runs = append(content.runs, e.rowRuns(row)...)
	}
	return content, nil
}

func (e *evaluator) evaluateTable(node docplan.Content) (Content, error) {
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	return Content{
		kind:        ContentTable,
		name:        node.Name(),
		caption:     node.Caption(),
		columns:     result.Columns(),
		rows:        result.Rows(),
		query:       node.Query().Entry(),
		queryOrigin: result.Origin(),
		origin:      node.Origin(),
	}, nil
}

func (e *evaluator) evaluateList(node docplan.Content) (Content, error) {
	result, err := e.executeQuery(node)
	if err != nil {
		return Content{}, err
	}
	rows := result.Rows()
	items := make([]ListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ListItem{runs: e.rowRuns(row), origin: row.Origin()})
	}
	return Content{
		kind:        ContentList,
		name:        node.Name(),
		style:       ListStyle(node.Style()),
		items:       items,
		query:       node.Query().Entry(),
		queryOrigin: result.Origin(),
		origin:      node.Origin(),
	}, nil
}

// executeQuery runs the planned query of a query-backed node.
func (e *evaluator) executeQuery(node docplan.Content) (*queryexec.RowSet, error) {
	reference := node.Query()
	bindings := make(queryexec.Bindings, len(reference.Bindings()))
	for _, binding := range reference.Bindings() {
		values := binding.Values()
		bound := make([]queryexec.Value, 0, len(values))
		for _, value := range values {
			bound = append(bound, executionValue(value))
		}
		bindings[binding.Parameter()] = bound
	}
	result, err := queryexec.Execute(reference.Program(), e.context, bindings, e.options)
	if err != nil {
		return nil, &Error{
			Kind:     ErrorQueryExecution,
			Document: e.document,
			Content:  node.Name(),
			Query:    reference.Entry(),
			Origin:   reference.Origin(),
			Err:      err,
		}
	}
	return result, nil
}

// executionValue converts one planned binding value into an execution value.
func executionValue(value docplan.BindingValue) queryexec.Value {
	if element, ok := value.Element(); ok {
		return queryexec.ElementValue(element)
	}
	if text, ok := value.String(); ok {
		return queryexec.StringValue(text)
	}
	if integer, ok := value.Integer(); ok {
		return queryexec.IntegerValue(integer)
	}
	if real, ok := value.Real(); ok {
		return queryexec.RealValue(real)
	}
	if boolean, ok := value.Boolean(); ok {
		return queryexec.BooleanValue(boolean)
	}
	return queryexec.Value{}
}

// rowRuns renders one query row as text runs: one run per projected cell
// value, or the element's name when the row has no projected cells.
func (e *evaluator) rowRuns(row queryexec.Row) []TextRun {
	cells := row.Cells()
	if len(cells) == 0 {
		return []TextRun{{text: e.valueText(row.Element()), origin: row.Origin()}}
	}
	var runs []TextRun
	for _, cell := range cells {
		for _, value := range cell.Values() {
			runs = append(runs, TextRun{text: e.valueText(value), origin: value.Origin()})
		}
	}
	return runs
}

// valueText renders one typed query value as deterministic plain text.
func (e *evaluator) valueText(value queryexec.Value) string {
	if element, ok := value.Element(); ok {
		if name := e.context.Model.EffectiveNameOf(element); name != "" {
			return name
		}
		return symbols.FQNOf(element)
	}
	if text, ok := value.String(); ok {
		return text
	}
	if integer, ok := value.Integer(); ok {
		return strconv.FormatInt(integer, 10)
	}
	if real, ok := value.Real(); ok {
		return strconv.FormatFloat(real, 'g', -1, 64)
	}
	if boolean, ok := value.Boolean(); ok {
		return strconv.FormatBool(boolean)
	}
	if value.Kind() == queryexec.ValueInfinity {
		return "*"
	}
	return ""
}
