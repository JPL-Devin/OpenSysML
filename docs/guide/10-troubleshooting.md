# 10. Troubleshooting

This chapter is organized by symptom. Budgets and the complete set of environment variables are
documented in [reference/environment.md](../reference/environment.md).

**The REPL does not display a prompt:**
- Confirm that the terminal supports readline (most Unix shells do)
- History is stored in `$XDG_STATE_HOME/sysml/history`, or in `~/.sysml_history` when `XDG_STATE_HOME` is unset; an unwritable path leaves history in memory for the session

**Import errors after building:**
- Run `go mod tidy`
- Verify the Go version with `go version` (1.25 or later is required)

**Execution stops with "limit exceeded" or "exceeded max":**
- The run exhausted one of its budgets; the message names the variable that raises it (see [reference/environment.md](../reference/environment.md))
- If the model does not terminate, the budget is reporting a genuine defect, and raising the budget only delays the error

**`%check`/`%explain` report "no SMT solver found":**
- Solving is an experimental extension and no solver is bundled; install z3 (or cvc5) for the platform in use — see [1. Install: installing a solver](01-install.md#installing-a-solver-optional). `brew install Open-MBEE/tap/opensysml` installs z3 as a dependency
- A solver installed outside `PATH` is named by `OPENSYSML_SMT`; a value that names no executable is reported rather than ignored
- No other command requires a solver: `%constraint`, `%requirement` and `%satisfy` evaluate without one

**`%check`/`%explain`/`%configure` report "does not support a feature this query needs":**
- The solver named by `OPENSYSML_SMT` rejected a feature the query requires; the message names the feature, the solver and the operation attempted. No result is reported from a script the solver would not accept
- The features each solver was measured to support, and the procedure for testing an additional solver, are documented in [1. Install: solver compatibility](01-install.md#solver-compatibility--pointing-the-driver-at-another-solver); z3 supports the entire subset
- cvc5 supports every feature the current commands require except objective optimization (`(maximize …)`, a z3 extension): `%optimize` refuses on cvc5, naming the missing extension, while every other command operates normally

**`%check` answers `unknown`:**
- With `Reason: the solver ran out of time`, the query required longer than `OPENSYSML_SMT_TIMEOUT` (default `10s`); `unknown` is a verdict in its own right and is never reported as `sat` or `unsat`
- Otherwise the solver was unable to decide the arithmetic, and the reason it reports states this

**Syntax errors:**
- Only SysML v2 textual notation is accepted; graphical notation and XMI are not
- Keywords are case-sensitive
- Multiplicity follows relationships: `part x subsets y [0..1];`

---

## Getting help

- **GitHub Issues:** report defects or request features
- **Discussions:** questions about SysML v2 usage
- **Specification reference:** [OMG SysML v2.1 Beta 1 Specification](https://www.omg.org/spec/SysML/2.0) (2026-07 release)

---

Back to the [guide index](README.md).
