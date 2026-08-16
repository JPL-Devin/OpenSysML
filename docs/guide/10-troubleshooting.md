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
