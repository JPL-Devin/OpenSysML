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

// cPrelude is the runtime every generated C program carries: checked int64,
// finite-only binary64, and the interpreter's once-rounded Integer quotient.
const cPrelude = `#include <errno.h>
#include <inttypes.h>
#include <math.h>
#include <setjmp.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>


typedef int64_t sysml_int;
typedef double sysml_real;
typedef bool sysml_bool;

static jmp_buf sysml_escape;
static const char *sysml_error_message;
static int sysml_depth;

static void sysml_fail(const char *msg) {
	sysml_error_message = msg;
	longjmp(sysml_escape, 1);
}

static inline void sysml_enter(void) {
	if (__builtin_expect(sysml_depth >= SYSML_MAX_CALC_DEPTH, 0))
		sysml_fail("calc recursion limit exceeded");
	sysml_depth++;
}

static inline void sysml_leave(void) { sysml_depth--; }

static inline sysml_int sysml_add(sysml_int a, sysml_int b) {
	sysml_int r;
	if (__builtin_expect(__builtin_add_overflow(a, b, &r), 0)) sysml_fail("arithmetic overflow: + exceeds the Integer range");
	return r;
}

static inline sysml_int sysml_sub(sysml_int a, sysml_int b) {
	sysml_int r;
	if (__builtin_expect(__builtin_sub_overflow(a, b, &r), 0)) sysml_fail("arithmetic overflow: - exceeds the Integer range");
	return r;
}

static inline sysml_int sysml_mul(sysml_int a, sysml_int b) {
	sysml_int r;
	if (__builtin_expect(__builtin_mul_overflow(a, b, &r), 0)) sysml_fail("arithmetic overflow: * exceeds the Integer range");
	return r;
}

static inline sysml_int sysml_neg(sysml_int a) {
	if (__builtin_expect(a == INT64_MIN, 0)) sysml_fail("arithmetic overflow: negation exceeds the Integer range");
	return -a;
}

static inline sysml_int sysml_mod(sysml_int a, sysml_int b) {
	if (__builtin_expect(b == 0, 0)) sysml_fail("division by zero");
	if (b == -1) return 0;
	return a % b;
}

/* The exact ratio a/b rounded once to binary64 (round to nearest, ties to even). */
static sysml_real sysml_quot(sysml_int a, sysml_int b) {
	if (__builtin_expect(b == 0, 0)) sysml_fail("division by zero");
	if (a > -(1LL << 53) && a < (1LL << 53) && b > -(1LL << 53) && b < (1LL << 53))
		return (sysml_real)a / (sysml_real)b;
	bool negative = (a < 0) != (b < 0);
	uint64_t ua = a < 0 ? -(uint64_t)a : (uint64_t)a;
	uint64_t ub = b < 0 ? -(uint64_t)b : (uint64_t)b;
	if (ua == 0) return 0.0;
	int la = 63 - __builtin_clzll(ua), lb = 63 - __builtin_clzll(ub);
	int s = 64 + lb - la;
	unsigned __int128 n = (unsigned __int128)ua << s;
	unsigned __int128 q = n / ub;
	bool sticky = (n % ub) != 0;
	int qbits = q >> 64 ? 128 - __builtin_clzll((uint64_t)(q >> 64)) : 64 - __builtin_clzll((uint64_t)q);
	int shift = qbits - 53;
	uint64_t mant = (uint64_t)(q >> shift);
	unsigned __int128 rem = q & (((unsigned __int128)1 << shift) - 1);
	unsigned __int128 half = (unsigned __int128)1 << (shift - 1);
	if (rem > half || (rem == half && (sticky || (mant & 1)))) {
		mant++;
		if (mant == (1ULL << 53)) { mant >>= 1; shift++; }
	}
	sysml_real r = ldexp((sysml_real)mant, shift - s);
	return negative ? -r : r;
}

static inline sysml_int sysml_nonnegative(sysml_int v, const char *type) {
	if (__builtin_expect(v < 0, 0)) {
		static char msg[128];
		snprintf(msg, sizeof msg, "type mismatch: cannot write %lld (an Integer) to a feature typed by %s", (long long)v, type);
		sysml_fail(msg);
	}
	return v;
}

static inline sysml_real sysml_finite(sysml_real r) {
	if (__builtin_expect(!isfinite(r), 0)) sysml_fail("arithmetic overflow: result is not a finite Real");
	return r;
}

static inline sysml_real sysml_rdiv(sysml_real a, sysml_real b) {
	if (__builtin_expect(b == 0, 0)) sysml_fail("division by zero");
	return sysml_finite(a / b);
}

static inline sysml_real sysml_rmod(sysml_real a, sysml_real b) {
	if (__builtin_expect(b == 0, 0)) sysml_fail("division by zero");
	return sysml_finite(fmod(a, b));
}

static sysml_int sysml_ipow(sysml_int a, sysml_int n) {
	sysml_int res = 1;
	while (n > 0) {
		if (n & 1) {
			if (__builtin_mul_overflow(res, a, &res)) sysml_fail("arithmetic overflow: ** exceeds the Integer range");
		}
		n >>= 1;
		if (n == 0) break;
		if (__builtin_mul_overflow(a, a, &a)) sysml_fail("arithmetic overflow: ** exceeds the Integer range");
	}
	return res;
}

static sysml_real sysml_rpow(sysml_real base, sysml_real exp) {
	if (base == 0 && exp < 0) sysml_fail("arithmetic domain: 0 ** negative exponent is undefined");
	if (base < 0 && exp != trunc(exp)) sysml_fail("arithmetic domain: negative base with fractional exponent is not a Real");
	return sysml_finite(pow(base, exp));
}

/* Writes r to buf as [-]d[.ddd]e[+-]xx with the fewest digits that read back as r,
 * trying the nearest p-digit decimal then its upper neighbour (wider above powers of two). */
static void sysml_shortest(sysml_real r, char *buf, size_t size) {
	for (int prec = 1; prec <= 17; prec++) {
		snprintf(buf, size, "%.*e", prec - 1, r);
		if (strtod(buf, NULL) == r) return;
		char up[40];
		snprintf(up, sizeof up, "%s", buf);
		char *first = up + (up[0] == '-');
		char *d = strchr(up, 'e') - 1;
		while (d >= first && (*d == '.' || *d == '9')) {
			if (*d == '9') *d = '0';
			d--;
		}
		if (d < first) continue; /* carried out: a shorter spelling already failed */
		(*d)++;
		if (strtod(up, NULL) == r) {
			snprintf(buf, size, "%s", up);
			return;
		}
	}
}

/* Prints r as the interpreter does: shortest round-trip digits, positional
 * for 1e-4 <= |r| < 1e21 and for 0, exponent form otherwise, always with a
 * fraction or exponent so a whole Real still reads as a Real. */
static void sysml_print_real(sysml_real r) {
	char buf[40];
	sysml_shortest(r, buf, sizeof buf);
	sysml_real a = fabs(r);
	if (r != 0 && (a < 1e-4 || a >= 1e21)) {
		fputs(buf, stdout);
		fputc('\n', stdout);
		return;
	}
	/* buf is [-]d[.ddd]e[+-]xx: reassemble the digits around the point. */
	char digits[24];
	int nd = 0, exp10 = 0;
	const char *p = buf;
	if (*p == '-') { fputc('-', stdout); p++; }
	for (; *p && *p != 'e'; p++) if (*p != '.') digits[nd++] = *p;
	exp10 = atoi(p + 1);
	int intdigits = exp10 + 1;
	if (intdigits <= 0) {
		fputs("0.", stdout);
		for (int i = intdigits; i < 0; i++) fputc('0', stdout);
		fwrite(digits, 1, nd, stdout);
	} else {
		for (int i = 0; i < intdigits; i++) fputc(i < nd ? digits[i] : '0', stdout);
		if (nd > intdigits) {
			fputc('.', stdout);
			fwrite(digits + intdigits, 1, nd - intdigits, stdout);
		} else {
			fputs(".0", stdout);
		}
	}
	fputc('\n', stdout);
}

static sysml_int sysml_parse_int(const char *s, const char *name) {
	char *end;
	errno = 0;
	long long v = strtoll(s, &end, 10);
	if (*s == 0 || *end != 0 || errno != 0) {
		fprintf(stderr, "argument %s: %s is not an Integer\n", name, s);
		exit(2);
	}
	return v;
}

static sysml_real sysml_parse_real(const char *s, const char *name) {
	char *end;
	double v = strtod(s, &end);
	if (*s == 0 || *end != 0 || !isfinite(v)) {
		fprintf(stderr, "argument %s: %s is not a finite Real\n", name, s);
		exit(2);
	}
	return v;
}

static sysml_bool sysml_parse_bool(const char *s, const char *name) {
	if (strcmp(s, "true") == 0) return true;
	if (strcmp(s, "false") == 0) return false;
	fprintf(stderr, "argument %s: %s is not a Boolean\n", name, s);
	exit(2);
}
`

// EmitC writes program as a self-contained C translation unit, with a main that
// reads argv and prints the result when withMain is set (errors: status 1, stderr).
func EmitC(w io.Writer, p *Program, withMain bool) error {
	e := &cEmitter{w: w}
	e.raw(fmt.Sprintf("#define SYSML_MAX_CALC_DEPTH %d\n", runtime.DefaultMaxCalcDepth))
	e.raw(cPrelude)
	for _, fn := range p.Funcs {
		e.linef("static %s %s(%s);", cType(fn.Result), fn.Ident, cParams(fn))
	}
	e.raw("\n")
	for _, fn := range p.Funcs {
		e.function(fn)
	}
	e.entry(p.Entry, withMain)
	return e.err
}

type cEmitter struct {
	w      io.Writer
	err    error
	indent int
	result string
	// resultRange is checked on each return of the function being emitted.
	resultRange Range
	temps       int
}

// pure is an operand whose evaluation cannot fail, so its order is immaterial.
func pure(x Expr) bool {
	switch x := x.(type) {
	case IntLit, RealLit, BoolLit, Var:
		return true
	case ToReal:
		return pure(x.X)
	}
	return false
}

// sequenced evaluates operands left to right into temporaries, as the
// interpreter does, then applies body to them; C leaves argument order open.
func (e *cEmitter) sequenced(operands []Expr, body func(names []string) string) string {
	names := make([]string, len(operands))
	var pre strings.Builder
	for i, x := range operands {
		if pure(x) {
			names[i] = e.expr(x)
			continue
		}
		e.temps++
		names[i] = fmt.Sprintf("sysml_t%d", e.temps)
		fmt.Fprintf(&pre, "%s %s = %s; ", cType(x.Type()), names[i], e.expr(x))
	}
	if pre.Len() == 0 {
		return body(names)
	}
	return fmt.Sprintf("({ %s%s; })", pre.String(), body(names))
}

// narrowed checks v against the range of the feature it is written to.
func cNarrowed(v string, r Range) string {
	if r == RangeAny {
		return v
	}
	return fmt.Sprintf("sysml_nonnegative(%s, \"%s\")", v, r)
}

func (e *cEmitter) raw(s string) {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

func (e *cEmitter) linef(format string, args ...any) {
	e.raw(strings.Repeat("\t", e.indent) + fmt.Sprintf(format, args...) + "\n")
}

func cType(t Type) string {
	switch t {
	case TypeInt:
		return "sysml_int"
	case TypeReal:
		return "sysml_real"
	case TypeBool:
		return "sysml_bool"
	}
	return "void"
}

func cParams(fn *Func) string {
	if len(fn.Params) == 0 {
		return "void"
	}
	parts := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parts[i] = cType(p.Type) + " " + cLocal(p.Name)
	}
	return strings.Join(parts, ", ")
}

// cLocal namespaces a SysML local so it can never collide with a C keyword or
// a prelude symbol.
func cLocal(name string) string {
	var b strings.Builder
	b.WriteString("v_")
	encodeName(&b, name)
	return b.String()
}

func (e *cEmitter) function(fn *Func) {
	e.linef("static %s %s(%s) {", cType(fn.Result), fn.Ident, cParams(fn))
	e.indent++
	e.linef("sysml_enter();")
	for _, p := range fn.Params {
		if p.Range != RangeAny {
			e.linef("%s = %s;", cLocal(p.Name), cNarrowed(cLocal(p.Name), p.Range))
		}
	}
	e.result = cType(fn.Result)
	e.resultRange = fn.ResultRange
	e.block(fn.Body)
	e.linef("sysml_fail(\"calc %s completed without returning a value\");", fn.Name)
	e.indent--
	e.linef("}")
	e.raw("\n")
}

func (e *cEmitter) block(stmts []Stmt) {
	for _, s := range stmts {
		e.stmt(s)
	}
}

func (e *cEmitter) stmt(s Stmt) {
	switch s := s.(type) {
	case Declare:
		e.linef("%s %s = %s;", cType(s.T), cLocal(s.Name), e.expr(s.Init))
	case Assign:
		e.linef("%s = %s;", cLocal(s.Name), cNarrowed(e.expr(s.Value), s.Range))
	case If:
		e.linef("if (%s) {", e.expr(s.Cond))
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
		e.linef("while (%s) {", e.expr(s.Cond))
		e.indent++
		e.block(s.Body)
		if s.Until != nil {
			e.linef("if (%s) break;", e.expr(s.Until))
		}
		e.indent--
		e.linef("}")
	case Return:
		e.linef("{ %s sysml_r = %s; sysml_leave(); return sysml_r; }", e.result, cNarrowed(e.expr(s.Value), e.resultRange))
	default:
		e.err = fmt.Errorf("codegen: C emitter has no case for %T", s)
	}
}

func cZero(t Type) string {
	if t == TypeBool {
		return "false"
	}
	return "0"
}

func (e *cEmitter) expr(x Expr) string {
	switch x := x.(type) {
	case IntLit:
		if x.Value == math.MinInt64 {
			return "INT64_MIN"
		}
		return fmt.Sprintf("INT64_C(%d)", x.Value)
	case RealLit:
		return cReal(x.Value)
	case BoolLit:
		if x.Value {
			return "true"
		}
		return "false"
	case Var:
		return cLocal(x.Name)
	case ToReal:
		return "(sysml_real)" + e.expr(x.X)
	case Unary:
		return e.unary(x)
	case Binary:
		return e.binary(x)
	case Cond:
		return fmt.Sprintf("(%s ? %s : %s)", e.expr(x.C), e.expr(x.Then), e.expr(x.Else))
	case Call:
		values := make([]Expr, len(x.Args))
		for i, a := range x.Args {
			values[i] = a.Value
		}
		return e.sequenced(values, func(names []string) string {
			return fmt.Sprintf("%s(%s)", x.Fn.Ident, strings.Join(callOperands(x, names), ", "))
		})
	}
	e.err = fmt.Errorf("codegen: C emitter has no case for %T", x)
	return "0"
}

// callOperands picks, per parameter, the name of the last argument bound to it.
func callOperands(x Call, names []string) []string {
	operands := make([]string, len(x.Fn.Params))
	for i, a := range x.Args {
		operands[a.Param] = names[i]
	}
	return operands
}

// inParamOrder reports whether a call's arguments are its parameters once each,
// in order, so evaluating the call's operands in place is source order.
func inParamOrder(x Call) bool {
	if len(x.Args) != len(x.Fn.Params) {
		return false
	}
	for i, a := range x.Args {
		if a.Param != i {
			return false
		}
	}
	return true
}

// cReal renders a float64 so the C compiler reads back the identical value.
func cReal(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

func (e *cEmitter) unary(x Unary) string {
	operand := e.expr(x.X)
	switch x.Op {
	case ast.OpNot:
		return "(!" + operand + ")"
	case ast.OpPos:
		return operand
	case ast.OpNeg:
		if x.T == TypeInt {
			return "sysml_neg(" + operand + ")"
		}
		return "(-" + operand + ")"
	}
	e.err = fmt.Errorf("codegen: C emitter has no unary case for %s", x.Op)
	return "0"
}

func (e *cEmitter) binary(x Binary) string {
	switch x.Op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return fmt.Sprintf("(%s && %s)", e.expr(x.L), e.expr(x.R))
	case ast.OpOr, ast.OpConditionalOr:
		return fmt.Sprintf("(%s || %s)", e.expr(x.L), e.expr(x.R))
	case ast.OpImplies:
		return fmt.Sprintf("(!%s || %s)", e.expr(x.L), e.expr(x.R))
	}
	return e.sequenced([]Expr{x.L, x.R}, func(v []string) string { return e.strict(x, v[0], v[1]) })
}

// strict is a binary operator whose operands l and r are already evaluated.
func (e *cEmitter) strict(x Binary, l, r string) string {
	operands := x.L.Type()
	switch x.Op {
	case ast.OpAdd, ast.OpSub, ast.OpMul:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_%s(%s, %s)", map[ast.OperatorKind]string{ast.OpAdd: "add", ast.OpSub: "sub", ast.OpMul: "mul"}[x.Op], l, r)
		}
		return fmt.Sprintf("sysml_finite(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpDiv:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_quot(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysml_rdiv(%s, %s)", l, r)
	case ast.OpMod:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_mod(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysml_rmod(%s, %s)", l, r)
	case ast.OpPow:
		return e.pow(x, l, r)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe, ast.OpEq, ast.OpNeq:
		return fmt.Sprintf("(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpXor:
		return fmt.Sprintf("(%s != %s)", l, r)
	}
	e.err = fmt.Errorf("codegen: C emitter has no binary case for %s", x.Op)
	return "0"
}

// pow follows semantics.Pow: Integer ** Integer is an Integer, anything else a
// Real. The compiler has already widened the operands of a Real power.
func (e *cEmitter) pow(x Binary, l, r string) string {
	if x.T == TypeInt {
		return fmt.Sprintf("sysml_ipow(%s, %s)", l, r)
	}
	return fmt.Sprintf("sysml_rpow(%s, %s)", l, r)
}

func cOperator(op ast.OperatorKind) string {
	switch op {
	case ast.OpAdd:
		return "+"
	case ast.OpSub:
		return "-"
	case ast.OpMul:
		return "*"
	case ast.OpLt:
		return "<"
	case ast.OpLe:
		return "<="
	case ast.OpGt:
		return ">"
	case ast.OpGe:
		return ">="
	case ast.OpEq:
		return "=="
	case ast.OpNeq:
		return "!="
	}
	return "?"
}

// entry emits the public wrapper and, optionally, main.
func (e *cEmitter) entry(fn *Func, withMain bool) {
	e.linef("/* %s: returns 0 and stores the result, or 1 with sysml_error set. */", fn.Name)
	e.linef("const char *sysml_error(void) { return sysml_error_message; }")
	e.raw("\n")
	params := cParams(fn)
	if params == "void" {
		params = ""
	} else {
		params += ", "
	}
	e.linef("int sysml_run(%s%s *result) {", params, cType(fn.Result))
	e.indent++
	e.linef("sysml_depth = 0;")
	e.linef("if (setjmp(sysml_escape)) return 1;")
	args := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		args[i] = cLocal(p.Name)
	}
	e.linef("*result = %s(%s);", fn.Ident, strings.Join(args, ", "))
	e.linef("return 0;")
	e.indent--
	e.linef("}")
	if !withMain {
		return
	}
	e.raw("\n")
	e.linef("int main(int argc, char **argv) {")
	e.indent++
	e.linef("long long repeat = 1;")
	e.linef("bool bad_repeat = false;")
	e.linef("if (argc >= 3 && strcmp(argv[1], \"--repeat\") == 0) {")
	e.linef("\tchar *end; errno = 0; repeat = strtoll(argv[2], &end, 10);")
	e.linef("\tbad_repeat = *argv[2] == '\\0' || *end != '\\0' || errno != 0 || repeat < 1;")
	e.linef("\targv += 2; argc -= 2;")
	e.linef("}")
	e.linef("if (bad_repeat || argc != %d) {", len(fn.Params)+1)
	e.indent++
	e.linef("fprintf(stderr, \"usage: %%s [--repeat N]%s\\n\", argv[0]);", cUsage(fn))
	e.linef("return 2;")
	e.indent--
	e.linef("}")
	for i, p := range fn.Params {
		e.linef("%s %s = sysml_parse_%s(argv[%d], \"%s\");", cType(p.Type), cLocal(p.Name), cType(p.Type)[len("sysml_"):], i+1, p.Name)
	}
	e.linef("%s result = %s;", cType(fn.Result), cZero(fn.Result))
	e.linef("for (long long i = 0; i < repeat; i++) {")
	e.indent++
	e.linef("if (sysml_run(%s)) {", strings.Join(append(args, "&result"), ", "))
	e.indent++
	e.linef("fprintf(stderr, \"%s: %%s\\n\", sysml_error());", fn.Name)
	e.linef("return 1;")
	e.indent--
	e.linef("}")
	e.indent--
	e.linef("}")
	switch fn.Result {
	case TypeInt:
		e.linef("printf(\"%%\" PRId64 \"\\n\", result);")
	case TypeReal:
		e.linef("sysml_print_real(result);")
	case TypeBool:
		e.linef("puts(result ? \"true\" : \"false\");")
	}
	e.linef("return 0;")
	e.indent--
	e.linef("}")
}

func cUsage(fn *Func) string {
	var b strings.Builder
	for _, p := range fn.Params {
		fmt.Fprintf(&b, " <%s:%s>", p.Name, p.Type)
	}
	return b.String()
}
