# REPL meta-commands

Every command `sysml` accepts at the prompt. The session model behind them (what a submission
replaces, what it drops) is explained in [guide chapter 4](../guide/04-repl.md).

Every command that takes a `<name>` accepts the quoted spelling the notation uses, including a
quoted segment containing a space and a quoted segment in the middle of a chain:
`%instantiate 'My Pkg'::Car`, `%features Top::'My Pkg'::Car`.

| Command | Description |
|---------|-------------|
| `%help` | Show help message |
| `%list` | List all declarations in current session |
| `%clear` | Clear session (reset all declarations) |
| `%load <path>...` | Submit the contents of files, directories or globs |
| `%print [name]` | Print the session model as SysML notation at the prompt, or only the named element and its body (`%print 'My Pkg'::Car`). Comments are kept, since the same writer `%save` writes notation with is used, and what is printed can be typed back in. Notation only: nothing about RDF is reported. Reading the model materializes nothing and leaves a debugging session running |
| `%save <file>` | Write the session model to a file: `.sysml` notation (comments preserved) or `.ttl` RDF, which is [experimental](rdf-mapping.md#status-experimental) and reported as such on each save |
| `%query <oslc-query>` | Identify model elements using OSLC Query text |
| `%verbosity [level]` | Show or set output level: `quiet` (errors only), `normal`, `debug` (every diagnostic over the whole buffer) |
| `%trace [on\|off]` | Show or set execution tracing: each evaluation, calc invocation, action step and state transition |
| `%strict [on\|off]` | Show or set strict conformance: report notation no SysML v2 production admits as an error, and reprint the session's diagnostics under the new mode ([Strict conformance](../guide/03-command-line.md#strict-conformance)) |
| `%budget` | Show the five bounds one run may spend, each with the variable that raises it |
| **Library Discovery** | |
| `%search <substring>` | List the declared and library symbols whose qualified name contains the substring, with the kind of each |
| `%builtins` | List the library functions the runtime implements directly (`sqrt`, `abs`, `max`, `floor`, `x->isEmpty()`, `x->sum()` …), each with the package an `import` must name for its bare name to resolve; the qualified name (`RealFunctions::sqrt(2.0)`) resolves anywhere |
| `%view <name>` | Show what a view exposes: its own `expose` relationships plus the protected ones of the views it specializes, the views nested in it (each with its own exposed set), and its conformance to every viewpoint it satisfies. Conformance is a verdict of `conforms`, `violated` or `unevaluable` per viewpoint and per framed concern, with the reason, the exposed element a concern's condition failed for, and `(from <view>)` where the `satisfy` is inherited. Asking about an element that is not a view says so |
| `%render <name> [form]` | Render a view's exposed set in the kind its `render` member states: a containment tree with nested views as subtrees, an interconnection diagram of the exposed parts and the connections between them, a state machine's states and transitions, an action's nodes and successions, or a table of the exposed elements and what they declare. A view with no `render` member renders as a tree. Output is indented text by default, or the machine-readable form of the kind: a [Mermaid](#rendering-a-view) diagram with `mermaid`, a Markdown table with `markdown`. Asking for a form the kind cannot be written in tells you which form it uses. Read-only: it creates no object and leaves a `%action`/`%state` debugging session running. A view that exposes nothing renders empty and says so; a rendering kind this build does not produce is reported by kind and view rather than rendered as something else; an element the rendering cannot represent is reported, not dropped |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create an object of a part definition and start the behaviors its type exhibits or performs. Each object runs its own machine, initialized after its feature values are built and run until it is quiescent. A second `%instantiate` of the same name creates a new object, and the name then refers to that one. A later submission keeps the object's identity but restarts its behaviors from their initial states, and says so |
| `%features <name> [all\|depth <n>] [json]` | Show what an object holds for each feature of its type. Reading a feature value builds the objects it holds, so the listing is bounded by default — 200 lines, nesting 8 deep — and a listing cut short says which form shows the rest. `all` lifts both bounds and reads the whole tree out; `depth <n>` bounds nesting at `n` levels and lifts the size bound, naming what it did not expand (`machine : Machine (not expanded: depth 1)`). `json` writes the object and everything reachable from it as one document in the shape the API's `Instantiate` returns (`instance`, `instances`, `diagnostics`), bounded by default at 1000 objects, with a graph cut short reported as a `warning` diagnostic. `all`/`depth` and `json` combine (`%features ctx all json`); `all` and `depth` together, a missing or negative depth, and an unknown word are errors naming the usage |
| `%instances` | List all created objects |
| `%eval <expr>` | Evaluate expression, in the last namespace the session declared; a library function is reached by its bare name only where that namespace imports its package, as the checker resolves it, and by its qualified name anywhere |
| `%eval in <name> : <expr>` | Evaluate expression in the named element's own namespace, or, when an object has been instantiated under that name, on that object, so that a feature reads the value it holds after its behaviors ran. The object can also be given by id (`%eval in #1 : recv.got`) or by a path under a named object (`%eval in ctx.recv : got`), as `%features` takes it. The separator is the first `:` outside a quoted name that is not part of a `::`, so `%eval in Demo : Vehicle::mass` works |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%run-query <name> [<p>=<expr>...]` | Execute a document query (a `calc def` specializing `DocumentQueries::Query`) and print its rows and projected cells. A projection lists declared property names and may add computed columns: `Column(name = "<column>", expression = <expr>)` entries evaluated once per row over the row element's features, with arithmetic (`+`, `-`, `*`, `/`), string concatenation and `??` defaults for absent values. A column expression that fails (including a reference that resolves to no value and has no `??` default) fails the query with a typed error rather than producing an empty cell. Each binding is written as `<parameter>=<expression>`; a name binds the element it refers to, anything else is evaluated as an expression. Named query invocation and relationship traversal (`RelatedElements` over specialization, subsetting, redefinition, typing, connection, allocation, satisfaction and verification edges, outgoing or incoming) are supported; a parameter default written as an expression is reported as an error, since the engine does not evaluate those yet. See the [query cookbook](../manual/query-cookbook.md) |
| `%render-document <name>` | Compile a document definition (a `part def` specializing `DocumentQueries::Document`), run its queries against the model and print the rendered Markdown. A document binds its queries' parameters in the model, so the name is the whole invocation. The output is deterministic CommonMark: the title and sections as ATX headings; paragraphs from text runs (`Span` runs with a `plain`/`emphasis`/`strong`/`code` style, `Link` runs to a URL, `Ref` runs linking to another content block's anchor, and query-produced values styled through nested `SpanColumn`/`LinkColumn` column runs); GitHub-flavored pipe tables with the projected column names (one subtable per group value when the table has a `groupBy` column); bullet and numbered lists; diagram blocks as fenced ` ```mermaid ` blocks rendered through the view engine (a table-kind view as a pipe table), with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction. Markdown metacharacters in content are escaped. Markdown is the only form the REPL writes; the CLI's `-doc-form html` renders the same document tree as semantic HTML ([Rendering a document as HTML](cli.md#rendering-a-document-as-html)) and `-doc-form pdf` converts the Markdown to PDF ([Rendering a document as PDF](cli.md#rendering-a-document-as-pdf)). See the [document generation manual](../manual/README.md) |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%invoke <object> <op> [<p>=<expr>]` | Invoke an operation of an object's type (an action it owns), performed by that object, with each argument written as `<parameter>=<expression>`. The object is an [object argument](#object-arguments): a top-level name, a feature path such as `driver.r`, or an id such as `#3`. Assignments in the body write that object's feature values; declared outputs are reported. Not yet supported: an operation given as a `calc` or `constraint`, and positional arguments |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor) |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element |
| `%check <name>` | **Experimental.** Ask an external SMT solver whether a constraint, requirement or satisfaction assertion *can* be satisfied, and on `sat` print an assignment. Reports `sat`, `unsat` or `unknown`, kept distinct. Needs `z3` or `cvc5` on `PATH` (or `OPENSYSML_SMT`; see [installing a solver](../guide/01-install.md#installing-a-solver-optional)) and reports an error rather than a verdict when none is installed. Satisfiability is not evaluation: use `%constraint`/`%satisfy` to find out what holds for an object |
| `%explain <name>` | **Experimental.** When `%check` answers `unsat`, ask the solver *which* conditions conflict. Prints an unsat core, reduced to a minimal one, as the role, the condition as written, the declaring element and `file:line:col`, in the query's order. A declared domain (a `Natural` being non-negative) or a division well-definedness guard can be one of the conflicting conditions. On `sat` there is no conflict to explain and `%check` gives the assignment; on `unknown` no explanation is available. Same solver requirements as `%check` ([installing a solver](../guide/01-install.md#installing-a-solver-optional)); `OPENSYSML_SMT_CORE_BUDGET` bounds the reduction |
| `%solve <name>` | **Experimental.** Ask the solver for values that satisfy a constraint, requirement or satisfaction assertion, keeping what is already fixed: the values an object holds, or failing that the ones the model declares, stay fixed and the rest are synthesised. Prints what was fixed (and by what), the values chosen, and a reminder that they are *one* witness of possibly many. `unsat` here means no values exist that are consistent with what is fixed, and names the fixed values that conflict. Same solver requirements as `%check` |
| `%configure <name> [<variation>=<variant>...] [all [<count>]]` | **Experimental.** Ask which variants a constraint, requirement or satisfaction assertion permits. With no argument, one consistent selection is synthesised. With `<variation>=<variant>`, the chosen selection is checked and the conflict is named when it is not consistent. With `all`, the consistent selections are enumerated up to `OPENSYSML_SMT_MAX_CONFIGURATIONS` (`all <count>` for a smaller bound), and the report says whether the list is complete or was cut short, either at the bound or because the solver stopped deciding or ran out of time; the selections found so far are still reported. An element that reads no variation point is an error pointing at `%check`. Same solver requirements as `%check` |
| `%optimize <name>` | **Experimental.** Ask the solver for the best values an `analysis def` (or an analysis usage) admits. Each `objective` is improved as the trade-study definition typing it says (`TradeStudies::MinimizeObjective` or `MaximizeObjective`), over the value it gives for the library's `best` feature (`attribute :>> best = expression;`), within the conditions the case requires or assumes and the ones the objective states itself. Several objectives are improved lexicographically in declaration order. Prints each optimum with its declared unit and the assignment that attains it. An objective that improves without limit, or a bound no assignment attains, is reported as such and never as a number, and every optimum is verified before it is reported. **Needs `z3`**: optimization is a z3 extension, and a backend without it (cvc5) is an error rather than a plain satisfiability check presented as an optimum. Otherwise the same solver requirements as `%check` |
| **Action debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%action <name> [<object>]` | Start an action debugging session, optionally performed by an instantiated object |
| `%step` | Advance one token step |
| `%continue` | Run the action to completion |
| `%tokens` | Show the active tokens |
| `%break <node>` | Set a breakpoint at a node |
| `%stop` | Stop the current debugging session |
| **State machine debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%state <name> [<object>]` | Debug the machine an object exhibits (`%state <part>` after `%instantiate <part>` attaches to that object's own running machine), or start a state machine, optionally performed by an instantiated object. Naming the machine the object exhibits (`%state Rover::modes rover`) — a running machine that *is* or is *typed by* `<machine>` — attaches to that running machine too, with a note saying so, rather than performing it a second time against the same feature values, so the object never runs two of them; only a machine the object does not exhibit is started as a detached performance, and the report says that too. Naming an exhibited machine alone (`%state Rover::modes`, or its short name `modes`) attaches to the running machine of the one held object exhibiting it — the object `%instances` and `%features` show. Held objects are the ones the session has built: a nested part counts once something has reached it (`%features driver`), and `%state` builds none itself. When no held object exhibits it, or several do, `%state` refuses and names the objects (or, with none held, the types exhibiting it), so that you name one with `%state <object>` or `%state <machine> <object>`; it never guesses, and never performs a machine a type exhibits detached from any object. The machine is addressed by any binding on the way to its body — the exhibited usage, a usage it references, or the definition typing it — so `%state Blink` finds the object exhibiting `spare : Blink`. A machine no type exhibits (a `state def` alone) is started as a detached performance, as there is no object's performance of it to attach to. A definition the object exhibits as the body of several usages names no one machine, so `%state` refuses and names the exhibited usages to name instead. The object is an [object argument](#object-arguments): a top-level name, a feature path such as `driver.r`, or an id such as `#3`. `%step`, `%advance`, `%current` and `%events` then drive that object's machine, and `%features` shows what it wrote |
| `%send <signal>[(<p>=<expr>, ...)] [to <object>]` | Send a signal to an object's machine through the runtime's own message bus, as `send <signal>(...) to <object>` from an action would. `<signal>` is a definition the model declares (an `attribute def`, `item def` or other signal-like definition; qualified names allowed), or a bare name an active `accept` matches by name when no declaration types it. Each argument is written `<parameter>=<expression>` as for `%invoke`, evaluated at the prompt, and must name a feature the signal carries with a value that feature admits (its type and multiplicity, checked before anything is sent); a feature left out is left unset. Without `to`, the target is the object whose machine the current `%state` session is debugging, and with no session the command says so rather than guessing. The signal is refused, with the machine's current state, when no machine of the object accepts it there, or when the guard of every transition it triggers is false — decided as the dispatch would decide it, the payload bound, so a guard that reads it is honoured; a guard that cannot be evaluated is an error. A signal the current state defers rather than accepts is sent and reported as deferred: the step dispatching it holds it (`%events` lists it as held) until the machine reaches a state that accepts it, when it is recalled and fires. Otherwise it is in flight (shown by `%events`) until `%step` or `%advance` dispatches it, and the transition it triggers fires as it would for a send from an action; should the state or the data a guard reads change before the dispatch, the step that drops the signal says so. An object running several machines is sent the signal as a whole: `%send` reports each machine that would fire on or defer it, a machine whose guards would drop it leaves it in flight for a sibling that would not, and the report says when the machine being debugged is such a one |
| `%events` | Show the event queue and the signals in flight |
| `%current` | Show the current state and configuration |
| `%advance <time>` | Advance simulation time by `<time>` units, processing every event due |
| **Control** | |
| `%quit` | Exit the REPL |
| `Tab` | Complete meta commands, symbol names (after `%print`, `%instantiate`, `%features` …), the form after `%render <name>`, and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |

## Object arguments

`%features`, `%invoke` and `%state` take an object the session holds, written in any of three
ways:

- the name it was instantiated under (`rover`, `Fleet::rover`);
- a feature path from such a name, following the parts an object holds (`driver.r`,
  `driver.r.motor`, or `Fleet::driver::r`);
- the identity the prompt prints for it (`#3`, from `ID: 3` or `object #3`).

A qualified path is read as typed: with both `Fleet::Driver` and `Fleet::driver` instantiated,
`Fleet::driver::r` is the usage's part, not the definition's, although the feature `r` is
declared in `Fleet::Driver`. A member of a multi-valued part (`part bays : Rover[2]`) is reached
by no path, since `garage.bays` holds all of them: the prompt names such an object by its
identity alone, and `%state #3` debugs its machine.

A path that stops short of an object is an error naming the segment that reached none and
why: `Fleet::driver.x reaches no object at "x": object #1 of "Fleet::driver" has no feature
"x"`, or `… at "level": feature "level" of object #2 holds 10, which is not an object`. A
segment whose feature value the runtime could not materialize keeps the runtime's reason: `… at
"spare": feature "spare" of object #1 could not be materialized: … multiplicity violation …`. An
id the session holds no object under is `no object #99 in this session`.

An id denotes only an object the session holds: one it named, or one a materialized feature of
such an object holds. Instantiating a name a second time makes a new object and leaves the
earlier one unnamed (`object #1 is no longer named`), so `#1` then reaches nothing: `no object
#1 in this session: it was superseded, and nothing the session names reaches it`. A debugging
session over the superseded object ends with that `%instantiate`, with a `note:` saying so.

A name nothing was instantiated under says what to instantiate. A usage whose definition alone
has an object (`%instantiate Fleet::Rover` when `%state … Fleet::rover` wanted the usage) is
reported as `no instance of the usage "Fleet::rover": object #1 of "Fleet::Rover" is of its
definition "Fleet::Rover", not of the usage — use %instantiate Fleet::rover to create the
usage's object, or name Fleet::Rover to address it`; a definition whose only objects are
usages typed by it names those usages the same way. With no related object, the error is
`no instance of "Fleet::rover" (use %instantiate first)`.

## Rendering a view

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

sysml> %render Demo::summary
Demo::summary - tree rendering (the view states no rendering; a tree is the default)

part def Demo::Vehicle
  attribute mass (Real)
  part wheel (Wheel)
view Demo::summary::detail
  part def Demo::Wheel
    attribute diameter (Real)

sysml> %render Demo::summary mermaid
%% Demo::summary — tree rendering
flowchart TD
  n0["part def Demo::Vehicle"]
  n1["attribute mass (Real)"]
  n0 --- n1
  n2["part wheel (Wheel)"]
  n0 --- n2
  n3["view Demo::summary::detail"]
  n4["part def Demo::Wheel"]
  n5["attribute diameter (Real)"]
  n4 --- n5
  n3 --- n4
```

A view that states `render asElementTable;`, or is typed by `StandardViewDefinitions::GridView`,
renders as rows instead: the exposed elements, the elements declared in them, and the views nested
in the rendered one, as aligned columns at the prompt and as a Markdown table with `markdown`.

```
sysml> %render Demo::parts
Demo::parts - table rendering (render asElementTable)

Element        Kind       Type   Declared in
-------------  ---------  -----  -------------
Demo::Vehicle  part def
mass           attribute  Real   Demo::Vehicle
wheel          part       Wheel  Demo::Vehicle

sysml> %render Demo::parts markdown
<!-- Demo::parts — table rendering (render asElementTable) -->
| Element | Kind | Type | Declared in |
| --- | --- | --- | --- |
| Demo::Vehicle | part def |  |  |
| mass | attribute | Real | Demo::Vehicle |
| wheel | part | Wheel | Demo::Vehicle |
```

The rendering is **tool-defined output**: SysML v2 §10.2 specifies the notation a view is written
in, not how a tool draws it. Mermaid is the machine-readable form for the graph-shaped kinds because
it renders as-is in Markdown, documentation sites and editors without a separate rendering tool, and
has a dedicated state diagram grammar. A table is a Markdown table, since Mermaid has no grammar
for tables. A state rendering reads the lowered state graph and an action rendering reads the
lowered action graph, so what is drawn is what the runtime executes.

To render a view outside the prompt, use [`sysml -render`](cli.md#rendering-a-view).
