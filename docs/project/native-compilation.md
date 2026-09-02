# Native compilation of calcs

`sysml -compile` translates a `calc def` (or a calc usage) into a standalone native executable,
ahead of time, through C or Go. The interpreter in `internal/core/runtime` stays the reference
semantics: a compiled program computes what `sysml -calc` computes, prints it the same way, and
fails on the same inputs — or the calc refuses to compile with a typed error saying which construct
is outside the subset. Nothing is compiled approximately.

This is a justification spike: it measures whether a native backend earns its place and, in
particular, whether the C toolchain dependency earns its place over pure Go. The numbers are in
[Measured results](#measured-results); the verdict is that it does, by a wide margin.

## Usage

```
sysml model.sysml -compile Pkg::Fib -o fib              # C, via cc -O3 -flto (default)
sysml model.sysml -compile Pkg::Fib -target go -o fib   # Go, via the Go toolchain
sysml model.sysml -compile Pkg::Fib -source -o fib.c    # write the generated source only

./fib 20             # 6765
./fib --repeat 100 20   # run 100 times, print once (for timing)
```

The executable takes the calc's parameters as command-line arguments, positionally, and prints
the result on one line in the interpreter's notation (`6765`, `1.75`, `2.0`, `1e21`, `true`). An
input the interpreter would reject — Integer overflow, division or modulo by zero, a non-finite
Real, recursion past the calc depth budget — exits with status 1 and the reason on stderr.

The generated source is always written beside the executable (`fib.c` / `fib.go`), so what was
compiled is inspectable. `OPENSYSML_CC` names the C compiler (default `cc`).

Programmatically: `Session.CompileCalc(name)` in `internal/repl` yields a `codegen.Program`, which
`codegen.Source` renders and `codegen.Build` compiles.

## The compiled subset

A calc compiles when everything it reaches is in this subset:

| Construct | Compiled as |
|---|---|
| `in` parameters typed `Integer`, `Natural`, `Positive`, `Real`/`Rational`, `Boolean`, with no multiplicity or `[1]` | `int64_t` / `double` / `bool` |
| Result: the body's trailing expression, or `return : T = <expr>;` | function result |
| Literals; parameter and body-local attribute references | as written |
| `+ - * / % **`, unary `-`, comparison, `== !=`, `and or xor not`, `implies`, `if c ? a else b` | checked native operations |
| `attribute x : T = e;`, `x = e;` / `assign x := e;` | locals and stores |
| `if` / `else`, `while … [until]`, `loop { … } until` | control flow |
| Invocation of another compilable calc, positional or named; direct and mutual recursion | native call |
| `calc c : D;`, `calc def E :> D;` adding no member of its own | compiles as `D` |

Everything else refuses: String, sequence or structured parameters/results, parameter defaults,
body-local attributes without a value (the interpreter holds null there, which no compiled type can),
multiplicity other than `[1]`, a calc that `:>`/`:>>`/`redefines` another *and* declares members
(redefining inherited parameters or body is not compiled), library-function invocations (`ScalarFunctions::sqrt` and the like), `for` and
collection operations, quantities and units, and `Integer ** <non-literal Integer>` (whether the
result is an Integer depends on the exponent's sign at run time, which a static type cannot
express; write the exponent as a literal or make the base Real). The refusal names the calc and
the construct (`codegen.UnsupportedError`, `errors.Is(err, codegen.ErrUnsupported)`).

## Semantics the generated code preserves

The runtime the generated program carries (`cPrelude` / `goPrelude`) reproduces the interpreter's
arithmetic rather than the host language's:

- **Integer** is `int64` with `+ - *`, negation and `**` checked for overflow (`__builtin_*_overflow`
  in C; widened checks in Go). `/` and `%` by zero are errors.
- **Integer `/`** is the exact rational quotient rounded once to binary64, as the interpreter's
  `IntQuotient` does — `7 / 2` is `3.5`, `1 / 3` is `0.3333333333333333`, and
  `9007199254740993 / 1` rounds the way the interpreter rounds. C does this with `__int128`
  remainder refinement; Go uses `math/big.Rat`.
- **Real** is binary64 and every result is checked finite; `1.0 / 0.0` and `1e308 * 10.0` are
  errors, not `inf`. `0.1 + 0.2` prints `0.30000000000000004`, exactly as the interpreter
  (see `exact-rational-evaluation.md`; no exact arithmetic is introduced here).
- **Mixed** Integer/Real operands widen the Integer, in comparisons too.
- **`and`/`or`/`implies`** short-circuit; the right operand's errors are not raised when the left
  decides. Every other operator, and every invocation, evaluates its operands left to right —
  named arguments in the order written, a parameter named twice taking the later value — so when
  two could fail the leftmost failure is the one reported. Generated C sequences operands through
  temporaries (GNU statement expressions) because C leaves argument order unspecified; Go's
  evaluation order already matches.
- **`Natural` and `Positive`** are checked exactly where the interpreter checks them: a negative
  Integer bound to such a parameter, assigned to such an attribute, or returned as such a result
  fails with the interpreter's `type mismatch` reason. Two interpreter behaviours are mirrored
  rather than corrected: `Positive` admits `0` (the interpreter's type lattice folds it into
  `Natural`), and an attribute's initializer is not checked, only later assignments.
- **Identifiers** of generated functions encode each name of the owner chain injectively
  (letters and digits verbatim, every other rune as `_hex_`, names joined by `_s_`), so `X::Y`,
  `X__Y`, the unrestricted name `'X::Y'` and a Unicode name never share a function.
- **Recursion** is bounded by the same depth as the interpreter's default
  (`runtime.DefaultMaxCalcDepth`), reported as the interpreter reports it.
- **Output** uses the interpreter's `FormatReal` convention: positional notation with a `.0` on
  whole values, exponent notation below `1e-4` and from `1e21`, `-0.0` preserved.

`internal/repl/compile_test.go:TestCompiledCalcsAgreeWithInterpreter` is the differential contract:
every calc in `testdata/compile_calcs.sysml` is compiled by both backends and run over a matrix of
values and failure inputs (overflow, zero divisors, non-finite Reals, deep recursion), and each
value or failure class must equal the interpreter's. `TestCompileRefusesWhatItCannotCompile` pins
the refusals.

### Known differences

- **No step budget.** The interpreter counts evaluation steps (`OPENSYSML_MAX_STEPS`, default
  10,000,000) and stops a runaway loop with `ErrStepLimitExceeded`. Compiled code counts nothing:
  a `while` that never terminates runs forever, and a long loop the interpreter would cut short
  runs to completion. The differential test lifts the interpreter's budget for this reason. This
  is the one bound the interpreter has that compiled code lacks; a compiled program is a program.
- **No evaluation trace.** There is nothing to `%trace`; the result is all the program produces.
- **GNU C.** The C backend uses `__int128`, `__builtin_*_overflow` and `setjmp`/`longjmp`, so it
  needs GCC or Clang, not an arbitrary ISO C compiler. Tested with GCC 11.4.

## Design

```
parser → resolve → semantics ─┐
                              ├→ codegen.Compiler ─→ codegen.Program (typed IR) ─→ EmitC / EmitGo ─→ cc / go build
lower.CalcBody (statements) ──┘
```

- `internal/core/codegen/ir.go` — the typed IR: `Func`, `Param`, expressions (`IntLit`, `Var`,
  `Binary`, `Call`, `ToReal`, …) and statements (`Declare`, `Assign`, `If`, `While`, `Return`).
  Every expression carries its scalar `Type`; the emitters never infer.
- `compile.go` — the front end. It walks the resolved symbol's members through
  `lower.CalcBody`, types every expression against the resolver and semantic model, inserts
  `ToReal` where the interpreter would widen, follows invocations into the callee's `calc def`
  and compiles that too, and refuses anything outside the subset with an `UnsupportedError`. The
  AST and semantic side tables are read, never mutated.
- `emit_c.go`, `emit_go.go` — one backend each. Both emit a self-contained program with the
  checked-arithmetic prelude, the calcs as functions, a `sysml_run` entry that turns an error into
  a status, and a `main`.
- `build.go` — `Source`, `Build`, `Targets`; C is compiled with
  `-O3 -flto -std=gnu11 -Wall -Wextra`, Go in a throwaway module.

## Benchmark methodology

`internal/repl/compile_bench_test.go:BenchmarkCompiledCalc` times the same invocation three ways
in one process: interpreted (`Session.RunCalc`), and as the C and Go executables run once with
`--repeat b.N`, so process start-up is amortized and each figure is per invocation.

```
go test ./internal/repl -run '^$' -bench BenchmarkCompiledCalc -benchtime 2s
```

The C loop is confirmed to do the work each iteration rather than being hoisted: `SumTo` scales
linearly (`1e6`: 0.39 ms, `1e7`: 3.8 ms, `1e8`: 38 ms per call).

## Measured results

Intel Xeon Platinum 8559C, 8 vCPUs, Go 1.25, GCC 11.4 `-O3 -flto`, 2026-09-02. Per invocation.

| Calc | Interpreted | Compiled Go | Compiled C | C vs interpreted | C vs Go |
|---|---:|---:|---:|---:|---:|
| `Fib(25)` — 242,785 recursive calls | 261 ms | 919 µs | 221 µs | 1180× | 4.2× |
| `SumTo(1000000)` — a `while` loop | 1216 ms | 764 µs | 379 µs | 3200× | 2.0× |
| `Collatz(27)` — 111 iterations of Real arithmetic | 206 µs | 5.2 µs | 0.98 µs | 210× | 5.3× |
| `Hypot(3.0, 4.0)` — one expression | 3.7 µs | 12 ns | 13 ns | 290× | 1.0× |

Reading the table:

- **Compilation is worth three orders of magnitude** on compute-bound calcs. That matches the
  interpreter census in `execution-performance-2026-09.md` (~1.1 µs per calc invocation, ~130 B
  allocated each): generated C spends about a nanosecond per `Fib` call.
- **C beats Go by 2–5× on every loop or recursion**, which is the justification asked for. The gap is
  most likely the checked arithmetic: GCC lowers `__builtin_add_overflow` to a flag test after the
  add, while Go's widened checks and function prologues stay in the hot path (inferred from the
  ratios, not from disassembly). The Go backend remains useful as
  a pure-Go fallback where no C compiler is installed, and as a second implementation the
  differential test checks the C against.
- **Trivial calcs are bound by the run harness**, not arithmetic: `Hypot` is 12 ns in either
  backend, most of it the `setjmp` (C) or `defer`/`recover` (Go) that turns an error into a
  status. A future C-ABI library entry point would drop that too.

## What this spike does not do

- Compile actions, state machines, constraints, requirements, parts, or instance graphs; the
  subset is scalar calcs. Extending the IR to collections and structured values is the next step
  and the one that would let a whole analysis case compile.
- Enforce the step budget (see [Known differences](#known-differences)).
- Link the compiled calc into the REPL or gRPC service; the output is a standalone executable.
- Offer a stable C ABI. The generated `sysml_run` signature is an implementation detail.
