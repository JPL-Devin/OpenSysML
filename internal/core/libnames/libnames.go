// Package libnames is the one table of which library declarations an
// unqualified function name denotes when a model imports none of them.
//
// The OMG Kernel Function Library is in force whatever a model imports, so a
// bare call to a name it declares denotes those declarations; the static
// checker and the runtime both read this table, so what one accepts the other
// evaluates. The table lists only the declarations the runtime implements.
package libnames

import "sort"

// functions maps an unqualified name to the declarations a bare call may
// denote, most general first: the first entry is the one dispatch falls back
// to when the argument types single none of them out.
var functions = map[string][]string{
	// RealFunctions, RationalFunctions, IntegerFunctions, NaturalFunctions and
	// NumericalFunctions declare the arithmetic helpers over their own scalar.
	"sqrt":   {"RealFunctions::sqrt"},
	"floor":  {"RealFunctions::floor"},
	"round":  {"RealFunctions::round"},
	"abs":    {"NumericalFunctions::abs", "ComplexFunctions::abs", "RealFunctions::abs", "RationalFunctions::abs", "IntegerFunctions::abs"},
	"max":    {"NumericalFunctions::max", "RealFunctions::max", "RationalFunctions::max", "IntegerFunctions::max", "NaturalFunctions::max"},
	"min":    {"NumericalFunctions::min", "RealFunctions::min", "RationalFunctions::min", "IntegerFunctions::min", "NaturalFunctions::min"},
	"isZero": {"NumericalFunctions::isZero", "ComplexFunctions::isZero"},
	"isUnit": {"NumericalFunctions::isUnit", "ComplexFunctions::isUnit"},

	// TrigFunctions.
	"sin":    {"TrigFunctions::sin"},
	"cos":    {"TrigFunctions::cos"},
	"tan":    {"TrigFunctions::tan"},
	"cot":    {"TrigFunctions::cot"},
	"arcsin": {"TrigFunctions::arcsin"},
	"arccos": {"TrigFunctions::arccos"},
	"arctan": {"TrigFunctions::arctan"},
	"deg":    {"TrigFunctions::deg"},
	"rad":    {"TrigFunctions::rad"},

	// VectorFunctions and ComplexFunctions: the names only one of the two declares.
	"VectorOf":               {"VectorFunctions::VectorOf"},
	"CartesianVectorOf":      {"VectorFunctions::CartesianVectorOf"},
	"CartesianThreeVectorOf": {"VectorFunctions::CartesianThreeVectorOf"},
	"isZeroVector":           {"VectorFunctions::isZeroVector"},
	"isCartesianZeroVector":  {"VectorFunctions::isCartesianZeroVector"},
	"scalarVectorMult":       {"VectorFunctions::scalarVectorMult"},
	"vectorScalarMult":       {"VectorFunctions::vectorScalarMult"},
	"vectorScalarDiv":        {"VectorFunctions::vectorScalarDiv"},
	"inner":                  {"VectorFunctions::inner"},
	"norm":                   {"VectorFunctions::norm"},
	"angle":                  {"VectorFunctions::angle"},
	"rect":                   {"ComplexFunctions::rect"},
	"polar":                  {"ComplexFunctions::polar"},
	"re":                     {"ComplexFunctions::re"},
	"im":                     {"ComplexFunctions::im"},
	"arg":                    {"ComplexFunctions::arg"},

	// StringFunctions. ToString is also declared by the numeric and Boolean
	// libraries, whose overloads join the table as the runtime implements them.
	"Length":    {"StringFunctions::Length"},
	"Substring": {"StringFunctions::Substring"},
	"ToString":  {"StringFunctions::ToString"},

	// SequenceFunctions, CollectionFunctions and ControlFunctions: the
	// operations over sequences and bodies, also written `x->name()`.
	"size":         {"SequenceFunctions::size"},
	"isEmpty":      {"SequenceFunctions::isEmpty"},
	"notEmpty":     {"SequenceFunctions::notEmpty"},
	"includes":     {"SequenceFunctions::includes"},
	"includesOnly": {"SequenceFunctions::includesOnly"},
	"excludes":     {"SequenceFunctions::excludes"},
	"equals":       {"SequenceFunctions::equals"},
	"same":         {"SequenceFunctions::same"},
	"union":        {"SequenceFunctions::union"},
	"intersection": {"SequenceFunctions::intersection"},
	"including":    {"SequenceFunctions::including"},
	"includingAt":  {"SequenceFunctions::includingAt"},
	"excluding":    {"SequenceFunctions::excluding"},
	"subsequence":  {"SequenceFunctions::subsequence"},
	"excludingAt":  {"SequenceFunctions::excludingAt"},
	"head":         {"SequenceFunctions::head"},
	"tail":         {"SequenceFunctions::tail"},
	"last":         {"SequenceFunctions::last"},
	"contains":     {"CollectionFunctions::contains"},
	"containsAll":  {"CollectionFunctions::containsAll"},
	"select":       {"ControlFunctions::select"},
	"selectOne":    {"ControlFunctions::selectOne"},
	"reject":       {"ControlFunctions::reject"},
	"collect":      {"ControlFunctions::collect"},
	"forAll":       {"ControlFunctions::forAll"},
	"exists":       {"ControlFunctions::exists"},
	"allTrue":      {"ControlFunctions::allTrue"},
	"anyTrue":      {"ControlFunctions::anyTrue"},
	"reduce":       {"ControlFunctions::reduce"},
	"minimize":     {"ControlFunctions::minimize"},
	"maximize":     {"ControlFunctions::maximize"},
	"sum":          {"NumericalFunctions::sum", "ComplexFunctions::sum", "RealFunctions::sum", "RationalFunctions::sum", "IntegerFunctions::sum"},
	"product":      {"NumericalFunctions::product", "ComplexFunctions::product", "RealFunctions::product", "RationalFunctions::product", "IntegerFunctions::product"},
}

// extensions maps the unqualified name of an OpenSysML extension function to
// the package a model must import to call it by that name: no OMG library
// declares these, so nothing puts them in scope on its own.
var extensions = map[string]string{
	"exp":   "OpenSysMLMathFunctions",
	"ln":    "OpenSysMLMathFunctions",
	"log":   "OpenSysMLMathFunctions",
	"atan2": "OpenSysMLMathFunctions",
}

// Declarations returns the library declarations a bare call to name denotes,
// most general first, or nil when no OMG library the runtime implements
// declares it. The slice is the table's own and must not be modified.
func Declarations(name string) []string {
	return functions[name]
}

// Names returns every unqualified name the OMG libraries answer, sorted.
func Names() []string {
	out := make([]string, 0, len(functions))
	for name := range functions {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ExtensionPackage returns the package an unqualified call to an OpenSysML
// extension function needs imported, and whether name is one.
func ExtensionPackage(name string) (string, bool) {
	pkg, ok := extensions[name]
	return pkg, ok
}

// ExtensionNames returns every extension function name, sorted.
func ExtensionNames() []string {
	out := make([]string, 0, len(extensions))
	for name := range extensions {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
