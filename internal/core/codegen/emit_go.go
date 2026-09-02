package codegen

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// goPrelude is the support library of a generated Go program. It mirrors
// cPrelude: the same checks, the same errors, the same output format.
const goPrelude = `package main

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
)

type sysmlError struct{ msg string }

func (e sysmlError) Error() string { return e.msg }

func sysmlFail(msg string) { panic(sysmlError{msg}) }

var sysmlDepth int

func sysmlEnter() {
	if sysmlDepth >= sysmlMaxCalcDepth {
		sysmlFail("calc recursion limit exceeded")
	}
	sysmlDepth++
}

func sysmlLeave() { sysmlDepth-- }

func sysmlAdd(a, b int64) int64 {
	r := a + b
	if !((b <= 0 || r > a) && (b >= 0 || r < a)) {
		sysmlFail("arithmetic overflow: + exceeds the Integer range")
	}
	return r
}

func sysmlSub(a, b int64) int64 {
	r := a - b
	if !((b >= 0 || r > a) && (b <= 0 || r < a)) {
		sysmlFail("arithmetic overflow: - exceeds the Integer range")
	}
	return r
}

func sysmlMulOK(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, false
	}
	r := a * b
	return r, r/b == a
}

func sysmlMul(a, b int64) int64 {
	r, ok := sysmlMulOK(a, b)
	if !ok {
		sysmlFail("arithmetic overflow: * exceeds the Integer range")
	}
	return r
}

func sysmlNeg(a int64) int64 {
	if a == math.MinInt64 {
		sysmlFail("arithmetic overflow: negation exceeds the Integer range")
	}
	return -a
}

func sysmlMod(a, b int64) int64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return a % b
}

func sysmlQuot(a, b int64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	q, _ := new(big.Rat).SetFrac64(a, b).Float64()
	return q
}

func sysmlFinite(r float64) float64 {
	if math.IsNaN(r) || math.IsInf(r, 0) {
		sysmlFail("arithmetic overflow: result is not a finite Real")
	}
	return r
}

func sysmlRDiv(a, b float64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return sysmlFinite(a / b)
}

func sysmlRMod(a, b float64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return sysmlFinite(math.Mod(a, b))
}

func sysmlIPow(a, n int64) int64 {
	res := int64(1)
	for n > 0 {
		var ok bool
		if n&1 == 1 {
			if res, ok = sysmlMulOK(res, a); !ok {
				sysmlFail("arithmetic overflow: ** exceeds the Integer range")
			}
		}
		if n >>= 1; n == 0 {
			break
		}
		if a, ok = sysmlMulOK(a, a); !ok {
			sysmlFail("arithmetic overflow: ** exceeds the Integer range")
		}
	}
	return res
}

func sysmlRPow(base, exp float64) float64 {
	switch {
	case base == 0 && exp < 0:
		sysmlFail("arithmetic domain: 0 ** negative exponent is undefined")
	case base < 0 && exp != math.Trunc(exp):
		sysmlFail("arithmetic domain: negative base with fractional exponent is not a Real")
	}
	return sysmlFinite(math.Pow(base, exp))
}

func sysmlParseInt(s, name string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not an Integer\n", name, s)
		os.Exit(2)
	}
	return v
}

func sysmlParseReal(s, name string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not a finite Real\n", name, s)
		os.Exit(2)
	}
	return v
}

func sysmlParseBool(s, name string) bool {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	fmt.Fprintf(os.Stderr, "argument %s: %s is not a Boolean\n", name, s)
	os.Exit(2)
	return false
}

func sysmlFormat(v any) string {
	if r, ok := v.(float64); ok {
		format := byte('f')
		if abs := math.Abs(r); r != 0 && (abs < 1e-4 || abs >= 1e21) {
			format = 'g'
		}
		text := strconv.FormatFloat(r, format, -1, 64)
		if !strings.ContainsAny(text, ".eEnN") {
			text += ".0"
		}
		return text
	}
	return fmt.Sprint(v)
}

// sysmlRun invokes fn, converting a failed check into an error.
func sysmlRun(fn func()) (err error) {
	sysmlDepth = 0
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(sysmlError); ok {
				err = errors.New(e.msg)
				return
			}
			panic(r)
		}
	}()
	fn()
	return nil
}
`

// EmitGo writes program as a self-contained Go main package whose command line
// and output match the C program's.
func EmitGo(w io.Writer, p *Program) error {
	e := &goEmitter{w: w}
	e.raw(goPrelude)
	e.raw(fmt.Sprintf("const sysmlMaxCalcDepth = %d\n\n", runtime.DefaultMaxCalcDepth))
	for _, fn := range p.Funcs {
		e.function(fn)
	}
	e.main(p.Entry)
	return e.err
}

type goEmitter struct {
	w      io.Writer
	err    error
	indent int
}

func (e *goEmitter) raw(s string) {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

func (e *goEmitter) linef(format string, args ...any) {
	e.raw(strings.Repeat("\t", e.indent) + fmt.Sprintf(format, args...) + "\n")
}

func goType(t Type) string {
	switch t {
	case TypeInt:
		return "int64"
	case TypeReal:
		return "float64"
	case TypeBool:
		return "bool"
	}
	return "struct{}"
}

func goLocal(name string) string { return cLocal(name) }

func goParams(fn *Func) string {
	parts := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parts[i] = goLocal(p.Name) + " " + goType(p.Type)
	}
	return strings.Join(parts, ", ")
}

func (e *goEmitter) function(fn *Func) {
	e.raw("\n")
	e.linef("func %s(%s) %s {", fn.Ident, goParams(fn), goType(fn.Result))
	e.indent++
	e.linef("sysmlEnter()")
	e.linef("defer sysmlLeave()")
	e.block(fn.Body)
	e.linef("sysmlFail(%s)", strconv.Quote("calc "+fn.Name+" completed without returning a value"))
	e.linef("panic(\"unreachable\")")
	e.indent--
	e.linef("}")
}

func (e *goEmitter) block(stmts []Stmt) {
	for _, s := range stmts {
		e.stmt(s)
	}
}

func (e *goEmitter) stmt(s Stmt) {
	switch s := s.(type) {
	case Declare:
		if s.Init == nil {
			e.linef("var %s %s", goLocal(s.Name), goType(s.T))
		} else {
			e.linef("var %s %s = %s", goLocal(s.Name), goType(s.T), e.expr(s.Init))
		}
		e.linef("_ = %s", goLocal(s.Name))
	case Assign:
		e.linef("%s = %s", goLocal(s.Name), e.expr(s.Value))
	case If:
		e.linef("if %s {", e.expr(s.Cond))
		e.indent++
		e.block(s.Then)
		e.indent--
		if len(s.Else) > 0 {
			e.linef("} else {")
			e.indent++
			e.block(s.Else)
			e.indent--
		}
		e.linef("}")
	case While:
		e.linef("for %s {", e.expr(s.Cond))
		e.indent++
		e.block(s.Body)
		if s.Until != nil {
			e.linef("if %s {", e.expr(s.Until))
			e.linef("\tbreak")
			e.linef("}")
		}
		e.indent--
		e.linef("}")
	case Return:
		e.linef("return %s", e.expr(s.Value))
	default:
		e.err = fmt.Errorf("codegen: Go emitter has no case for %T", s)
	}
}

func (e *goEmitter) expr(x Expr) string {
	switch x := x.(type) {
	case IntLit:
		if x.Value == math.MinInt64 {
			return "int64(math.MinInt64)"
		}
		return fmt.Sprintf("int64(%d)", x.Value)
	case RealLit:
		return "float64(" + cReal(x.Value) + ")"
	case BoolLit:
		return strconv.FormatBool(x.Value)
	case Var:
		return goLocal(x.Name)
	case ToReal:
		return "float64(" + e.expr(x.X) + ")"
	case Unary:
		operand := e.expr(x.X)
		switch x.Op {
		case ast.OpNot:
			return "(!" + operand + ")"
		case ast.OpPos:
			return operand
		case ast.OpNeg:
			if x.T == TypeInt {
				return "sysmlNeg(" + operand + ")"
			}
			return "(-" + operand + ")"
		}
	case Binary:
		return e.binary(x)
	case Cond:
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()", goType(x.T), e.expr(x.C), e.expr(x.Then), e.expr(x.Else))
	case Call:
		args := make([]string, len(x.Args))
		for i, a := range x.Args {
			args[i] = e.expr(a)
		}
		return fmt.Sprintf("%s(%s)", x.Fn.Ident, strings.Join(args, ", "))
	}
	e.err = fmt.Errorf("codegen: Go emitter has no case for %T", x)
	return "0"
}

func (e *goEmitter) binary(x Binary) string {
	l, r := e.expr(x.L), e.expr(x.R)
	ints := x.L.Type() == TypeInt
	switch x.Op {
	case ast.OpAdd, ast.OpSub, ast.OpMul:
		if ints {
			return fmt.Sprintf("sysml%s(%s, %s)", map[ast.OperatorKind]string{ast.OpAdd: "Add", ast.OpSub: "Sub", ast.OpMul: "Mul"}[x.Op], l, r)
		}
		return fmt.Sprintf("sysmlFinite(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpDiv:
		if ints {
			return fmt.Sprintf("sysmlQuot(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRDiv(%s, %s)", l, r)
	case ast.OpMod:
		if ints {
			return fmt.Sprintf("sysmlMod(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRMod(%s, %s)", l, r)
	case ast.OpPow:
		if x.T == TypeInt {
			return fmt.Sprintf("sysmlIPow(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRPow(%s, %s)", l, r)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe, ast.OpEq, ast.OpNeq:
		return fmt.Sprintf("(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpAnd, ast.OpConditionalAnd:
		return fmt.Sprintf("(%s && %s)", l, r)
	case ast.OpOr, ast.OpConditionalOr:
		return fmt.Sprintf("(%s || %s)", l, r)
	case ast.OpXor:
		return fmt.Sprintf("(%s != %s)", l, r)
	case ast.OpImplies:
		return fmt.Sprintf("(!%s || %s)", l, r)
	}
	e.err = fmt.Errorf("codegen: Go emitter has no binary case for %s", x.Op)
	return "0"
}

func (e *goEmitter) main(fn *Func) {
	e.raw("\n")
	e.linef("func main() {")
	e.indent++
	e.linef("args := os.Args[1:]")
	e.linef("repeat := 1")
	e.linef("if len(args) >= 2 && args[0] == \"--repeat\" {")
	e.linef("\trepeat, _ = strconv.Atoi(args[1])")
	e.linef("\targs = args[2:]")
	e.linef("}")
	e.linef("if len(args) != %d {", len(fn.Params))
	e.linef("\tfmt.Fprintf(os.Stderr, \"usage: %%s [--repeat N]%s\\n\", os.Args[0])", cUsage(fn))
	e.linef("\tos.Exit(2)")
	e.linef("}")
	args := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parser := map[Type]string{TypeInt: "sysmlParseInt", TypeReal: "sysmlParseReal", TypeBool: "sysmlParseBool"}[p.Type]
		e.linef("%s := %s(args[%d], %q)", goLocal(p.Name), parser, i, p.Name)
		args[i] = goLocal(p.Name)
	}
	e.linef("var result %s", goType(fn.Result))
	e.linef("for i := 0; i < repeat; i++ {")
	e.linef("\tif err := sysmlRun(func() { result = %s(%s) }); err != nil {", fn.Ident, strings.Join(args, ", "))
	e.linef("\t\tfmt.Fprintf(os.Stderr, \"%s: %%s\\n\", err)", fn.Name)
	e.linef("\t\tos.Exit(1)")
	e.linef("\t}")
	e.linef("}")
	e.linef("fmt.Println(sysmlFormat(result))")
	e.indent--
	e.linef("}")
}
