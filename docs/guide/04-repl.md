# 4. The REPL

`sysml` with no model starts a prompt. What you type is a declaration, an expression, or a
meta-command beginning with `%`; the declarations accumulate into a session model that every
command answers about.

```
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> package Demo { part def Wheel { attribute diameter : Real = 16.0; } }
✓ package Demo
sysml> %eval Demo::Wheel::diameter
  = 16.00
```

## What a submission does

A declaration is parsed and validated on submission, and the result is reported immediately —
`✓` with the declared name, or the diagnostics. An unterminated declaration continues on the
next line (`  ...>`) until its braces close.

Two consequences of the session model are worth knowing before a long session:

- **A submission replaces any earlier snippet declaring the same names.** Re-typing
  `package Demo { … }` to add one member replaces the whole package body rather than merging
  into it, so keep a growing model in a file and `%load` it.
- **A submission resets what was derived from the previous model** — the instances created with
  `%instantiate`, and an active `%action`/`%state` debugging session. `%step` after a
  declaration reports that there is no active session; start the debugger again.

`%list` shows the session's declarations, `%clear` resets it, and `%save` writes it out
([chapter 7](07-saving-and-rdf.md)).

## Loading files

```
sysml> %load examples/rdf-interop-demo.sysml
sysml> %load examples/*.sysml
sysml> %load examples/
```

`%load` takes files, globs and directories, and submits their contents as if typed. Tab
completes paths after `%load` and `%save`, and completes meta-commands and symbol names
everywhere else.

## Finding what a build offers

`%search` looks a substring up across the declared and library symbols with the kind of each,
and `%builtins` lists the library functions this build evaluates directly — which is how to tell
whether an expression will evaluate before writing a model around it.

```
sysml> %search Vehicle
sysml> %builtins
```

## Asking questions

| To ask | Use | Chapter |
|--------|-----|---------|
| what an expression is worth | `%eval` | [5](05-checking.md) |
| what an object's slots hold | `%instantiate`, `%slots`, `%instances` | [5](05-checking.md) |
| whether a check holds | `%constraint`, `%requirement`, `%satisfy`, `%calc` | [5](05-checking.md) |
| what a behavior does, step by step | `%action`, `%state`, `%step`, `%tokens`, `%advance` | [6](06-behavior.md) |
| where a run stopped and why | `%trace`, `%budget`, `%verbosity` | [10](10-troubleshooting.md) |

Every command, with its arguments, is [reference/repl-commands.md](../reference/repl-commands.md).

## Without the prompt

`-e` evaluates one expression against a model and exits, which is the same session machinery
without a terminal — see [chapter 3](03-command-line.md#running-from-a-script).

---

Next: [5. Expressions, calculations, constraints and requirements](05-checking.md).
