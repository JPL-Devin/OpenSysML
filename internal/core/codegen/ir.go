// Package codegen compiles the scalar subset of SysML v2 calcs to native code;
// see docs/project/native-compilation.md for the subset and its semantics.
package codegen

import "github.com/Open-MBEE/OpenSysML/internal/core/ast"

// Type is a scalar type of the compiled subset.
type Type int

const (
	TypeInvalid Type = iota
	TypeInt          // Integer and its subtypes, int64 with reported overflow
	TypeReal         // Real and Rational, IEEE 754 binary64
	TypeBool
)

func (t Type) String() string {
	switch t {
	case TypeInt:
		return "Integer"
	case TypeReal:
		return "Real"
	case TypeBool:
		return "Boolean"
	}
	return "invalid"
}

// Program is a set of compiled functions with one entry point.
type Program struct {
	Funcs []*Func
	Entry *Func
}

// Func is one compiled calculation.
type Func struct {
	Name   string // qualified SysML name
	Ident  string // identifier valid in every target language
	Params []Param
	Result Type
	Body   []Stmt
}

// Param is one input parameter.
type Param struct {
	Name string
	Type Type
}

// Expr is a typed expression.
type Expr interface {
	Type() Type
}

// IntLit, RealLit and BoolLit are literals.
type IntLit struct{ Value int64 }
type RealLit struct{ Value float64 }
type BoolLit struct{ Value bool }

// Var reads a parameter or a body-local variable.
type Var struct {
	Name string
	T    Type
}

// Binary applies an arithmetic, comparison or logical operator. Operands are
// already coerced to a common type; T is the result type.
type Binary struct {
	Op   ast.OperatorKind
	L, R Expr
	T    Type
}

// Unary applies `-`, `+` or `not`.
type Unary struct {
	Op ast.OperatorKind
	X  Expr
	T  Type
}

// Cond is `if c ? a else b`, both branches of type T.
type Cond struct {
	C, Then, Else Expr
	T             Type
}

// Call invokes another compiled function, arguments coerced to parameter types.
type Call struct {
	Fn   *Func
	Args []Expr
}

// ToReal widens an Integer to a Real.
type ToReal struct{ X Expr }

func (IntLit) Type() Type   { return TypeInt }
func (RealLit) Type() Type  { return TypeReal }
func (BoolLit) Type() Type  { return TypeBool }
func (v Var) Type() Type    { return v.T }
func (b Binary) Type() Type { return b.T }
func (u Unary) Type() Type  { return u.T }
func (c Cond) Type() Type   { return c.T }
func (c Call) Type() Type   { return c.Fn.Result }
func (ToReal) Type() Type   { return TypeReal }

// Stmt is a statement of a function body.
type Stmt interface{ stmt() }

// Declare introduces a body-local variable; Init is nil for a bare declaration.
type Declare struct {
	Name string
	T    Type
	Init Expr
}

// Assign writes a body-local variable or parameter.
type Assign struct {
	Name  string
	Value Expr
}

// If runs Then or Else on a Boolean condition.
type If struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
}

// While runs Body while Cond holds; Until, if set, is tested after each pass
// and stops the loop when it holds.
type While struct {
	Cond  Expr
	Until Expr
	Body  []Stmt
}

// Return answers the function's result.
type Return struct{ Value Expr }

func (Declare) stmt() {}
func (Assign) stmt()  {}
func (If) stmt()      {}
func (While) stmt()   {}
func (Return) stmt()  {}
