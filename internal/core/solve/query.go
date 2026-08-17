package solve

import (
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Role says why a query asserts a term.
type Role int

const (
	// RoleRequired is a condition the element requires to hold (`require`,
	// `assert`).
	RoleRequired Role = iota
	// RoleAssumed is a condition the element assumes (`assume`), trusted rather
	// than required.
	RoleAssumed
	// RoleDenied is the assertion a negated element makes: that its required
	// conditions do not all hold.
	RoleDenied
	// RoleDomain is a bound a declaration puts on a variable's values rather
	// than a condition the model wrote, such as a Natural being non-negative.
	RoleDomain
)

var roleNames = map[Role]string{
	RoleRequired: "required condition",
	RoleAssumed:  "assumed condition",
	RoleDenied:   "denied conditions",
	RoleDomain:   "declared domain",
}

// String names the role as an assertion's comment reads it.
func (r Role) String() string {
	if name, ok := roleNames[r]; ok {
		return name
	}
	return "condition"
}

// Var is one variable a query declares: the value a feature may take. It is
// unconstrained except by its sort and the domain assertions the query makes
// about it — values a model declares are not asserted here.
type Var struct {
	// Name is the variable's name, the qualified name of the feature it stands
	// for, with a feature chain's further steps appended with '.'.
	Name string

	// Sort is the sort of its values.
	Sort Sort

	// Symbol is the feature declaration it stands for.
	Symbol *symbols.Symbol

	// Dimension is the quantity dimension its magnitude is expressed in, over
	// base units, empty for a value that has none.
	Dimension string

	// File and Span are where the feature was declared.
	File string
	Span source.Span

	// Location renders File and Span as `file:line:col`.
	Location string
}

// Provenance records what an assertion came from, so a later step can map an
// answer about the script back to what the user wrote.
type Provenance struct {
	// Kind is the kind of element the assertion came from: "constraint",
	// "requirement", "satisfaction", or "declaration" for a domain assertion.
	Kind string

	// Element names that element as a verdict about it would.
	Element string

	// Condition is the condition as written, negation and grouping included.
	Condition string

	// Role says why the query asserts the term.
	Role Role

	// Declared is the element that declared the condition, which is the
	// supertype it was inherited from for an inherited one.
	Declared *symbols.Symbol

	// File and Span are where the condition was written.
	File string
	Span source.Span

	// Location renders File and Span as `file:line:col`.
	Location string
}

// Assertion is one term the query asserts, with where it came from.
type Assertion struct {
	// Term is the boolean term asserted.
	Term *Term

	// From records the condition the term encodes.
	From Provenance
}

// Query is a translated element: the variables its conditions read, the finite
// sorts they range over, and the terms it asserts. It is what a solver-facing
// step consumes, and what the SMT-LIB2 writer writes.
type Query struct {
	// Kind is the kind of element translated: "constraint", "requirement" or
	// "satisfaction".
	Kind string

	// Element names the translated element as a verdict about it would.
	Element string

	// Negated is set for an element asserting that its conditions do not all
	// hold (`assert not …`), which the query asserts as one denial.
	Negated bool

	// Sorts are the datatype sorts the query declares, ordered by name.
	Sorts []Sort

	// Vars are the variables the query declares, ordered by name.
	Vars []*Var

	// Assertions are the terms asserted, declared domains first and then the
	// conditions in the order the evaluator checks them.
	Assertions []Assertion

	// Nonlinear is set when a product or a quotient of two non-literal terms was
	// asserted, which decides the logic the script sets.
	Nonlinear bool
}

// Logic returns the SMT-LIB logic the sorts and operators used need: "ALL" once
// a datatype or string is involved, the narrowest arithmetic logic otherwise.
func (q *Query) Logic() string {
	usesInt, usesReal, usesString, usesDatatype := false, false, false, false
	note := func(s Sort) {
		switch s.Kind {
		case SortInt:
			usesInt = true
		case SortReal:
			usesReal = true
		case SortString:
			usesString = true
		case SortDatatype:
			usesDatatype = true
		}
	}
	for _, v := range q.Vars {
		note(v.Sort)
	}
	for _, a := range q.Assertions {
		a.Term.walk(func(t *Term) { note(t.Sort) })
	}
	switch {
	case usesDatatype || usesString:
		return "ALL"
	case usesInt && usesReal:
		if q.Nonlinear {
			return "QF_NIRA"
		}
		return "QF_LIRA"
	case usesReal:
		if q.Nonlinear {
			return "QF_NRA"
		}
		return "QF_LRA"
	case usesInt:
		if q.Nonlinear {
			return "QF_NIA"
		}
		return "QF_LIA"
	default:
		return "QF_UF"
	}
}

// sortVars orders variables by name, which is what makes a script deterministic.
func sortVars(vars []*Var) {
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
}

// sortSorts orders datatype sorts by name.
func sortSorts(sorts []Sort) {
	sort.Slice(sorts, func(i, j int) bool { return sorts[i].Name < sorts[j].Name })
}

// sortAssertions orders assertions by what they came from, which is what makes
// declared domains, collected as variables appear, deterministic.
func sortAssertions(asserts []Assertion) {
	sort.SliceStable(asserts, func(i, j int) bool {
		a, b := asserts[i].From, asserts[j].From
		if a.Element != b.Element {
			return a.Element < b.Element
		}
		return a.Condition < b.Condition
	})
}
