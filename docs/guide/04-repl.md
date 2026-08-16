# 4. The REPL

`sysml` with no model starts a prompt. What you type is a declaration, an expression, or a
meta-command beginning with `%`; the declarations accumulate into a session model that every
command answers about.

```
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> import ScalarValues::*;
✓ import ScalarValues::*
sysml> package Demo { part def Wheel { attribute diameter : Real = 16.0; } }
✓ package Demo
sysml> %eval Demo::Wheel::diameter
✓ Demo::Wheel::diameter
  = 16.00
```

The import is what makes `Real` resolvable, as in [chapter 2](02-first-model.md); a session that
uses a library type without it is rejected with `unresolved reference: Real`.

## What a submission does

A declaration is parsed and validated on submission, and the result is reported immediately —
`✓` with the declared name, or the diagnostics. An unterminated declaration continues on the
next line (`  ...>`) until its braces close.

Two properties of the session model are worth knowing before a long session:

- **A submission adds to the namespace already in the session.** Re-typing `package Demo { … }`
  with one more member folds into the declaration already there — `note: added to the existing
  package Demo (its other members are kept)` — rather than replacing its body. Re-typing a
  member *does* replace that member, and says so; an empty body (`package Demo { }`) is how a
  namespace is emptied.
- **A submission only invalidates what it changed.** Objects created with `%instantiate` are
  carried over while the declarations they were built from are untouched, and an
  `%action`/`%state` debugging session over a declaration the submission left alone keeps
  running. What is dropped is reported as a `note:` line saying what to re-run.

`%list` shows the session's declarations, `%clear` resets it, and `%save` writes it out
([chapter 7](07-saving-and-rdf.md)). `%clear` replaces every declaration, so nothing it held can be
proved to still exist: it reports what it took, and the next command that would have used it says why
it is gone.

```
sysml> %clear
note: 1 object was dropped (the session was reset); %instantiate to create it again
✓ Session cleared
sysml> %instances
No instances (1 object was dropped when the session was reset)
```

## A name that needs quotes

A name the notation has to quote — one containing a space, a keyword used as a name, punctuation —
is written to a command exactly as it is written in a model, quotes included, and a quoted segment
can sit anywhere in the chain:

```
sysml> package 'My Pkg' { part def Car { attribute m = 5.0; } }
sysml> %instantiate 'My Pkg'::Car
sysml> %slots 'My Pkg'::Car
sysml> %eval 'My Pkg'::Car::m
```

The unquoted spelling of a name that does not need quoting is unchanged. Commands report a name back
quoted the same way, so a name from `%search` can be typed into the next command as it appears.

## Loading files

```
sysml> %load examples/rdf-interop-demo.sysml
sysml> %load examples/*.sysml
sysml> %load examples/
```

`%load` takes files, globs and directories, and submits their contents as if typed — with one
difference: a loaded namespace keeps the file's identity, so re-typing it at the prompt *replaces*
it (`note: replaced package …`) rather than adding to it. Edit the file and load it again. Tab
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

## Where an expression is evaluated

`%eval <expression>` evaluates in the namespace the session is working in — the last one it declared
— so a scratch package typed later moves it. Name a context to pin it instead:

```
sysml> %eval in Demo::Vehicle : mass * 2
✓ mass * 2 (in Demo::Vehicle)
  = 3000.00
sysml> %instantiate Demo::Vehicle
sysml> %eval in Demo::Vehicle : mass * 2
✓ mass * 2 (on Demo::Vehicle ID: 1)
  = 3000.00
```

The pinned name may be an element, whose namespace the expression then reads, or a name an object
was materialized under, whose slots it reads — the same values an unpinned `%eval` reads after
`%instantiate`. The `:` separates the context from the expression; `::` inside the name is not a
separator.

## Asking questions

| To ask | Use | Chapter |
|--------|-----|---------|
| what an expression is worth | `%eval`, `%eval in … : …` | [5](05-checking.md) |
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
