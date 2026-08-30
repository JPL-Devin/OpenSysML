# 4. The REPL

Running `sysml` without a model starts an interactive prompt. Each entry is a declaration, an
expression or a meta-command beginning with `%`. Declarations accumulate into a session model,
which is the model every command reports on.

```
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> private import ScalarValues::*;
✓ import ScalarValues::*
sysml> package Demo { part def Wheel { attribute diameter : Real = 16.0; } }
✓ package Demo
sysml> %eval Demo::Wheel::diameter
✓ Demo::Wheel::diameter
  = 16.0
```

The import makes `Real` resolvable, as described in [chapter 2](02-first-model.md). A session
that uses a library type without importing it is rejected with `unresolved reference: Real`.

## What a submission does

A declaration is parsed and validated on submission, and the result is reported immediately as
either `✓` with the declared name or a set of diagnostics. An unterminated declaration continues
on the next line (`  ...>`) until its braces close.

Two properties of the session model are important to understand before beginning a long session:

- **A submission adds to the namespace already present in the session.** Re-entering
  `package Demo { … }` with an additional member merges into the existing declaration —
  `note: added to the existing package Demo (its other members are kept)` — rather than
  replacing its body. Re-entering a member does replace that member, and the replacement is
  reported. An empty body (`package Demo { }`) empties the namespace.
- **A submission invalidates only what it changed.** Objects created with `%instantiate` are
  retained while the declarations they were built from remain untouched, and an `%action` or
  `%state` debugging session over an unaffected declaration continues to run. A retained object
  keeps its identity but not its execution state: the behaviors its type exhibits or performs
  restart from their initial states in the rebuilt analysis. Everything dropped or restarted is
  reported on a `note:` line describing what occurred.

`%list` shows the session's declarations, `%clear` resets the session, and `%save` writes it out
([chapter 7](07-saving-and-rdf.md)). Because `%clear` discards every declaration, nothing it held
remains available: it reports what was discarded, and the next command that would have used it
explains why it is no longer present.

```
sysml> %clear
session cleared
note: 1 instance was dropped because the session was reset; re-run %instantiate
sysml> %instances
(no instances created; 1 instance was dropped when the session was reset — re-run %instantiate)
```

## Seeing the model

`%print` writes the session back to the prompt as notation, either the whole model or, when a
name is given, a single element and its body:

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

`%print` uses the same writer that `%save` uses for a `.sysml` file, so it preserves comments and
the original text, re-indented as a save would indent it, and its output can be typed or loaded
back to rebuild the same model. Names are written as in every other command
(`%print 'My Pkg'::Car`). Printing only reads the session: it creates no objects, and an
`%action` or `%state` session continues across it. An empty session, a name that nothing
declares, and a name whose element the session holds no source for are each reported on a single
line. RDF output is available through `%save` with a `.ttl` path
([chapter 7](07-saving-and-rdf.md)); `%print` emits notation only.

## A name that needs quotes

A name that the notation requires to be quoted — one containing a space or punctuation, or a
keyword used as a name — is supplied to a command exactly as it is written in a model, including
the quotes. A quoted segment may appear anywhere in the qualified name:

```
sysml> package 'My Pkg' { part def Car { attribute m = 5.0; } }
sysml> %instantiate 'My Pkg'::Car
sysml> %features 'My Pkg'::Car
sysml> %eval 'My Pkg'::Car::m
```

Names that do not require quoting are written unquoted, as before. Commands report names back
with the same quoting, so a name returned by `%search` can be entered into the next command
exactly as it appears.

## Loading files

```
sysml> %load examples/rdf-interop-demo.sysml
sysml> %load examples/*.sysml
sysml> %load examples/
```

`%load` accepts files, globs and directories, and submits their contents as though they had been
typed, with one difference: a loaded namespace retains the identity of its file, so re-entering
it at the prompt *replaces* it (`note: replaced package …`) rather than adding to it. To change a
loaded file, edit it and load it again. Tab completion completes paths after `%load` and `%save`,
and completes meta-commands and symbol names elsewhere.

A file the parser cannot read is reported in the same way as a typed declaration, with the same
diagnostic pointing at the corresponding line of the file. None of its contents enter the
session, so the next submission is parsed against the model as it stood before the load. In the
non-interactive path, the diagnostics of a load are errors, so a script that loads a malformed
file fails rather than continuing against an empty session.

Two loaded files that both open `package P` declare two packages of that name, and the load
reports this:

```
sysml> %load a.sysml b.sysml
note: P is opened by more than one loaded file; each opening stays a declaration of its own, so a
member of one is not visible unqualified in the other — qualify it (P::member)
```

Each file retains its own identity, which is what allows re-loading one of them to replace only
its own contribution. Merging the two openings into a single namespace would therefore allow an
edit to one file to silently delete the other file's members. The members of both openings are
declared and reachable when qualified (`P::Wheel`, `P::Axle`), but an unqualified reference
across the two does not resolve. Re-entering a package at the prompt is unaffected: it still
merges into the package already present in the session.

## Finding what a build offers

`%search` looks up a substring across the declared and library symbols and reports the kind of
each. `%builtins` lists the library functions that the current build evaluates directly, which
determines whether an expression can be evaluated before a model is written around it.

```
sysml> %search Vehicle
sysml> %builtins
sysml> %view Demo::summary
```

`%view <name>` reports the elements a view exposes, the views nested within it, and whether it
conforms to the viewpoints it satisfies. Conformance checking is read-only and reported in
declaration order. Each `satisfy` carries a verdict of `conforms`, `violated` or `unevaluable`,
followed by a verdict for each concern the viewpoint frames; a concern whose condition fails
names the exposed element it failed for, together with the reason. A `satisfy` inherited from a
specialized view is marked `(from <view>)`. A concern that the viewpoint frames but the view does
not is reported as `violated`, and a concern that states no condition, or names one that does not
resolve, is reported as `unevaluable` with the reason rather than as a pass. These verdicts, and
the treatment of a nested view's framing, are decisions made by this implementation, because
SysML v2 leaves verification verdict semantics non-normative.

```
sysml> package Demo {
  ...>     private import ScalarValues::*;
  ...>     private import Views::*;
  ...>     part def Wheel {
  ...>         attribute diameter : Real = 16.0;
  ...>     }
  ...>     part def Vehicle {
  ...>         attribute mass : Real;
  ...>         part wheel : Wheel;
  ...>     }
  ...>     part vehicle : Vehicle {
  ...>         attribute :>> mass = 1200.0;
  ...>     }
  ...>     concern def MassBudget {
  ...>         subject s : Vehicle;
  ...>         attribute maxMass : Real = 1000.0;
  ...>         require constraint {
  ...>             s.mass < maxMass
  ...>         }
  ...>     }
  ...>     concern def Modularity {
  ...>         subject s : Vehicle;
  ...>         require constraint {
  ...>             1 < 2
  ...>         }
  ...>     }
  ...>     viewpoint def StructurePerspective {
  ...>         frame concern budget : MassBudget;
  ...>         frame concern modularity : Modularity;
  ...>     }
  ...>     viewpoint structure : StructurePerspective;
  ...>     view def StructureView {
  ...>         satisfy structure;
  ...>         frame concern budget : MassBudget;
  ...>         frame concern modularity : Modularity;
  ...>     }
  ...>     view report : StructureView {
  ...>         expose vehicle;
  ...>         view detail {
  ...>             expose Wheel;
  ...>         }
  ...>     }
  ...>     view summary : StructureView {
  ...>         expose Vehicle;
  ...>         view detail {
  ...>             expose Wheel;
  ...>         }
  ...>     }
  ...>     view parts {
  ...>         expose Vehicle;
  ...>         render asElementTable;
  ...>     }
  ...> }
✓ package Demo

sysml> %view Demo::report
view Demo::report
  exposes
    Demo::vehicle (partUsage)
  nested views
    Demo::report::detail (viewUsage)
  viewpoint conformance
    satisfy structure (from Demo::StructureView): violated
      concern budget: violated
        Demo::vehicle: satisfaction satisfy MassBudget by Demo::vehicle: require condition evaluated to false: s.mass < maxMass
      concern modularity: conforms
```

When a name cannot be found, the session suggests the qualified names under which it is known,
nearest scope first — declarations in the session before library symbols, and a package's member
before a name nested inside another element — up to a maximum of three suggestions.

## Rendering a view

`%render <name>` renders the exposed set in the form the view's `render` member specifies: a
containment tree with nested views as subtrees, an interconnection diagram of the exposed parts
and the connections between them, a state machine's states and transitions, an action's nodes and
successions, or a table of the exposed elements. A view that specifies no rendering is rendered
as a tree:

```
sysml> %render Demo::summary
Demo::summary - tree rendering (the view states no rendering; a tree is the default)

part def Demo::Vehicle
  attribute mass (Real)
  part wheel (Wheel)
view Demo::summary::detail
  part def Demo::Wheel
    attribute diameter (Real)
```

A view that states `render asElementTable;` is rendered as aligned columns instead, listing the
exposed elements, what they declare, and the views nested within the rendered view.

`%render <name> mermaid` writes a graph-shaped rendering as a Mermaid diagram, and
`%render <name> markdown` writes a table as a Markdown table. Either form can be pasted directly
into a Markdown document or an editor, and requesting a form that the rendering kind does not
support reports the form it does support. State and action renderings read the lowered graphs
that the runtime executes, so the rendering reflects what actually runs. The rendering itself is
specific to this implementation, because SysML v2 §10.2 specifies the notation rather than how a
tool draws it.

Rendering only reads the model: it creates no objects, and a `%render` issued between two `%step`
commands leaves an action or state debugging session unchanged. A view that exposes nothing
renders as empty and reports this, a rendering kind the build does not produce is reported by
name together with the view rather than substituted, and an element the rendering cannot
represent is reported rather than omitted. The equivalent operation outside the prompt is
[`sysml -render`](../reference/cli.md#rendering-a-view).

## Where an expression is evaluated

`%eval <expression>` evaluates in the namespace the session is currently working in, which is the
most recently declared one, so declaring a scratch package later changes that context. To pin the
context, name it explicitly:

```
sysml> %eval in Demo::Vehicle : mass * 2
✓ mass * 2 (in Demo::Vehicle)
  = 3000.0
sysml> %instantiate Demo::Vehicle
✓ Created instance of Demo::Vehicle
  ID: 1
  Use %features Demo::Vehicle to inspect
sysml> %eval in Demo::Vehicle : mass * 2
✓ mass * 2 (on Demo::Vehicle ID: 1)
  = 3000.0
```

The pinned name may be an element, in which case the expression reads that element's namespace,
or a name under which an object was materialized, in which case the expression reads that
object's feature values — the same values an unpinned `%eval` reads after `%instantiate`. The `:`
character separates the context from the expression; the `::` within a qualified name is not a
separator.

## Command summary

| To determine | Use | Chapter |
|--------|-----|---------|
| what a view shows | `%view`, `%render` | this chapter |
| what an expression is worth | `%eval`, `%eval in … : …` | [5](05-checking.md) |
| what an object holds for each feature | `%instantiate`, `%features`, `%instances` | [5](05-checking.md) |
| whether a check holds | `%constraint`, `%requirement`, `%satisfy`, `%calc` | [5](05-checking.md) |
| whether a check *can* hold at all (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%check` | [reference](../reference/repl-commands.md) |
| which conditions conflict when it cannot (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%explain` | [reference](../reference/repl-commands.md) |
| what values satisfy it, keeping what is already fixed (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%solve` | [reference](../reference/repl-commands.md) |
| which variants its conditions permit (experimental, needs [z3 or cvc5](01-install.md#installing-a-solver-optional)) | `%configure` | [reference](../reference/repl-commands.md) |
| which values are best for an analysis case's objectives (experimental, needs [z3](01-install.md#installing-a-solver-optional)) | `%optimize` | [reference](../reference/repl-commands.md) |
| what a behavior does, step by step | `%action`, `%state`, `%step`, `%tokens`, `%advance` | [6](06-behavior.md) |
| where a run stopped and why | `%trace`, `%budget`, `%verbosity` | [10](10-troubleshooting.md) |
| whether what is typed is conforming SysML v2 | `%strict` | [3](03-command-line.md#strict-conformance) |

Every command, with its arguments, is [reference/repl-commands.md](../reference/repl-commands.md).

## Non-interactive use

`-e` evaluates a single expression against a model and exits, using the same session machinery
without a terminal. See [chapter 3](03-command-line.md#running-from-a-script).

---

Next: [5. Expressions, calculations, constraints and requirements](05-checking.md).
