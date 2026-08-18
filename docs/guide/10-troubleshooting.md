# 10. Troubleshooting

Symptom first. Budgets and every environment variable are in
[reference/environment.md](../reference/environment.md).

**REPL doesn't show prompt:**
- Check terminal supports readline (most Unix shells do)
- History stored in `$XDG_STATE_HOME/sysml/history`, or `~/.sysml_history` when `XDG_STATE_HOME` is unset; an unwritable path leaves history in memory for the session

**Import errors after build:**
- Run `go mod tidy`
- Verify Go version: `go version` (need 1.25+)

**Execution stops with "limit exceeded" or "exceeded max":**
- The run spent one of its budgets; the message names the variable that raises it (see [reference/environment.md](../reference/environment.md))
- If the model does not terminate, the budget is reporting a real bug — raising it only delays the error

**`%check`/`%explain` report "no SMT solver found":**
- Solving is an experimental extension and no solver is bundled; install z3 (or cvc5) per platform — [1. Install: installing a solver](01-install.md#installing-a-solver-optional). `brew install Open-MBEE/tap/opensysml` brings z3 with it
- A solver installed outside `PATH` is named by `OPENSYSML_SMT`; a value naming no executable is reported rather than passed over
- Nothing else needs a solver: `%constraint`, `%requirement` and `%satisfy` evaluate without one

**`%check`/`%explain`/`%configure` report "does not support a feature this query needs":**
- The solver `OPENSYSML_SMT` names rejected a feature the query needs — the message names the feature, the solver and what was being asked of it; nothing is answered from a script the solver would not accept
- Which feature each solver was measured to support, and how to check one of your own, is [1. Install: solver compatibility](01-install.md#solver-compatibility--pointing-the-driver-at-another-solver); z3 supports the whole subset
- cvc5 supports every feature today's commands need; the one part of the subset it lacks is objective optimization (`(maximize …)`, a z3 extension), which no command emits yet

**`%check` answers `unknown`:**
- With `Reason: the solver ran out of time`, the query needed longer than `OPENSYSML_SMT_TIMEOUT` (default `10s`); `unknown` is a verdict, never reported as `sat` or `unsat`
- Otherwise the solver gave up on the arithmetic, and the reason it gives says so

**Syntax errors:**
- SysML v2 textual notation only (no graphical/XMI)
- Keywords are case-sensitive
- Multiplicity goes after relationships: `part x subsets y [0..1];`

---

## Getting Help

- **GitHub Issues:** Report bugs or request features
- **Discussions:** Ask questions about SysML v2 usage
- **Spec Reference:** [OMG SysML v2.1 Beta 1 Specification](https://www.omg.org/spec/SysML/2.0) (2026-05 release)

---

Back to the [guide index](README.md).
