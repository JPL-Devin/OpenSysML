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
session cleared
note: 1 instance was dropped because the session was reset; re-run %instantiate
sysml> %instances
(no instances created; 1 instance was dropped when the session was reset — re-run %instantiate)
```

## Seeing the model

`%print` writes the session back as notation, at the prompt — the whole model, or one element and
its body when a name is given:

```
sysml> package Demo {
  ...>   // how heavy it is
  ...>   part def Vehicle {
  ...>     attribute mass = 1500.0;
  ...>   }
  ...> }
✓ package Demo
sysml> %print Demo::Vehicle
// how heavy it is
part def Vehicle {
    attribute mass = 1500.0;
}
```

It is the writer `%save` writes a `.sysml` file with, so a print keeps comments and the text as it
was written, re-indented the way a save would be, and what it prints can be typed or loaded back to
rebuild the same model.
Names are spelled as every other command spells them (`%print 'My Pkg'::Car`). Printing reads the
session and nothing more: no object is created, and an `%action`/`%state` session keeps running
across it. An empty session, a name nothing declares, and a name whose element this session holds
no source of each say so in one line. RDF is `%save`'s `.ttl` path ([chapter 7](07-saving-and-rdf.md));
a print is notation only.

## A name that needs quotes

A name the notation has to quote — one containing a space, a keyword used as a name, punctuation —
is written to a command exactly as it is written in a model, quotes included, and a quoted segment
can sit anywhere in the chain:

```
sysml> package 'My Pkg' { part def Car { attribute m = 5.0; } }
sysml> %instantiate 'My Pkg'::Car
sysml> %features 'My Pkg'::Car
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

A file whose text the parser cannot read is reported the way a typed declaration is — the same
diagnostic, pointing into the file at its own line — and nothing of it enters the session, so the
next submission is parsed against what was there before the load. In the non-interactive path the
diagnostics of a load are errors, so a script loading a broken file fails rather than continuing
against an empty session.

Two loaded files that both open `package P` declare two packages of that name, and the load says
so:

```
sysml> %load a.sysml b.sysml
note: P is opened by more than one loaded file; each opening stays a declaration of its own, so a
member of one is not visible unqualified in the other — qualify it (P::member)
```

Each file keeps its own identity — that is what makes re-loading one of them replace only its own
contribution — so the two openings cannot be one namespace without a file's edit silently deleting
the other file's members. Both openings' members are declared and reachable qualified
(`P::Wheel`, `P::Axle`); an unqualified reference across the two does not resolve. Re-typing a
package at the prompt is unaffected: it still folds into the package already in the session.

## Finding what a build offers

`%search` looks a substring up across the declared and library symbols with the kind of each,
and `%builtins` lists the library functions this build evaluates directly — which is how to tell
whether an expression will evaluate before writing a model around it.

```
sysml> %search Vehicle
sysml> %builtins
sysml> %view Demo::summary
```

`%view <name>` reports what a view exposes, the views nested in it, and whether it conforms to the
viewpoints it satisfies. Conformance is read-only and reported in declaration order: each `satisfy`
carries a verdict — `conforms`, `violated` or `unevaluable` — then each concern the viewpoint frames
carries its own, and a concern whose condition fails names the exposed element it failed for with
the reason. A `satisfy` inherited from a specialized view is marked `(from <view>)`, a concern the
viewpoint frames but the view does not is `violated`, and a concern stating no condition, or naming
one that does not resolve, is `unevaluable` with the reason rather than a pass.
The verdicts and the treatment of a nested view's framing are this tool's choice: SysML v2 leaves
verification verdict semantics non-normative.

```
sysml> %view Demo::report
view Demo::report
  exposes
    Demo::vehicle (partUsage)
  viewpoint conformance
    satisfy structure (from Demo::StructureView): violated
      concern budget: violated
        Demo::vehicle: satisfaction satisfy MassBudget by Demo::vehicle: require condition evaluated to false: s.mass < 1000.0
      concern modularity: conforms
```

A name the session cannot find is offered the qualified names it is known under, nearest scope
first — what the session itself declares before the library, and a package's member before a name
nested inside another element — and at most three of them.

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
was materialized under, whose feature values it reads — the same values an unpinned `%eval` reads after
`%instantiate`. The `:` separates the context from the expression; `::` inside the name is not a
separator.

## Asking questions

| To ask | Use | Chapter |
|--------|-----|---------|
| what an expression is worth | `%eval`, `%eval in … : …` | [5](05-checking.md) |
| what an object holds for each feature | `%instantiate`, `%features`, `%instances` | [5](05-checking.md) |
| whether a check holds | `%constraint`, `%requirement`, `%satisfy`, `%calc` | [5](05-checking.md) |
| whether a check *can* hold at all (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%check` | [reference](../reference/repl-commands.md) |
| which conditions conflict when it cannot (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%explain` | [reference](../reference/repl-commands.md) |
| what a behavior does, step by step | `%action`, `%state`, `%step`, `%tokens`, `%advance` | [6](06-behavior.md) |
| where a run stopped and why | `%trace`, `%budget`, `%verbosity` | [10](10-troubleshooting.md) |

Every command, with its arguments, is [reference/repl-commands.md](../reference/repl-commands.md).

## Without the prompt

`-e` evaluates one expression against a model and exits, which is the same session machinery
without a terminal — see [chapter 3](03-command-line.md#running-from-a-script).

---

Next: [5. Expressions, calculations, constraints and requirements](05-checking.md).
