# REPL meta-commands

Every command `sysml` accepts at the prompt. The session model behind them (what a submission
replaces, what it drops) is explained in [guide chapter 4](../guide/04-repl.md).

Every command that takes a `<name>` accepts the quoted spelling the notation uses, including a
quoted segment containing a space and a quoted segment in the middle of a chain:
`%instantiate 'My Pkg'::Car`, `%features Top::'My Pkg'::Car`.

Every command that takes an `<object>` — `%features`, `%invoke`, `%eval in`, and the object
`%action` and `%state` are performed by or attached to — reads one
[object reference](#object-references): the name an object was instantiated under
(`car`, `Demo::car`), its id as `%instantiate` printed it (`#3`), or either followed by a path
into the parts it holds (`car.fl.hub`, `#3.fl`, `car.wheels[2]`).

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
| `%builtins` | List the library functions the runtime implements directly (`sqrt`, `abs`, `max`, `floor`, `x->isEmpty()`, `x->sum()` …) |
| `%view <name>` | Show what a view exposes: its own `expose` relationships plus the protected ones of the views it specializes, the views nested in it (each with its own exposed set), and its conformance to every viewpoint it satisfies. Conformance is a verdict of `conforms`, `violated` or `unevaluable` per viewpoint and per framed concern, with the reason, the exposed element a concern's condition failed for, and `(from <view>)` where the `satisfy` is inherited. Asking about an element that is not a view says so |
| `%render <name> [form]` | Render a view's exposed set in the kind its `render` member states: a containment tree with nested views as subtrees, an interconnection diagram of the exposed parts and the connections between them, a state machine's states and transitions, an action's nodes and successions, or a table of the exposed elements and what they declare. A view with no `render` member renders as a tree. Output is indented text by default, or the machine-readable form of the kind: a [Mermaid](#rendering-a-view) diagram with `mermaid`, a Markdown table with `markdown`. Asking for a form the kind cannot be written in tells you which form it uses. Read-only: it creates no object and leaves a `%action`/`%state` debugging session running. A view that exposes nothing renders empty and says so; a rendering kind this build does not produce is reported by kind and view rather than rendered as something else; an element the rendering cannot represent is reported, not dropped |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create an object of a part definition and start the behaviors its type exhibits or performs. Each object runs its own machine, initialized after its feature values are built and run until it is quiescent. A second `%instantiate` of the same name creates a new object, and the name then refers to that one. A later submission keeps the object's identity but restarts its behaviors from their initial states, and says so |
| `%features <object>` | Show what an object holds for each feature of its type. The object is named, addressed by id, or reached by a path: `%features car`, `%features #3`, `%features car.fl.hub`, `%features car.wheels[2]` |
| `%instances` | List all created objects: the named ones, and the ones a second `%instantiate` of their name left addressable only by id |
| `%eval <expr>` | Evaluate expression, in the last namespace the session declared |
| `%eval in <name> : <expr>` | Evaluate expression in the named element's own namespace, or, when the name is an [object reference](#object-references) (`car`, `#3`, `car.fl`), on that object, so that a feature reads its value. The separator is the first `:` outside a quoted name that is not part of a `::`, so `%eval in Demo : Vehicle::mass` works |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%run-query <name> [<p>=<expr>...]` | Execute a document query (a `calc def` specializing `DocumentQueries::Query`) and print its rows and projected cells. A projection lists declared property names and may add computed columns: `Column(name = "<column>", expression = <expr>)` entries evaluated once per row over the row element's features, with arithmetic (`+`, `-`, `*`, `/`), string concatenation and `??` defaults for absent values. A column expression that fails (including a reference that resolves to no value and has no `??` default) fails the query with a typed error rather than producing an empty cell. Each binding is written as `<parameter>=<expression>`; a name binds the element it refers to, anything else is evaluated as an expression. Named query invocation and relationship traversal (`RelatedElements` over specialization, subsetting, redefinition, typing, connection, allocation, satisfaction and verification edges, outgoing or incoming) are supported; a parameter default written as an expression is reported as an error, since the engine does not evaluate those yet. See the [query cookbook](../manual/query-cookbook.md) |
| `%render-document <name>` | Compile a document definition (a `part def` specializing `DocumentQueries::Document`), run its queries against the model and print the rendered Markdown. A document binds its queries' parameters in the model, so the name is the whole invocation. The output is deterministic CommonMark: the title and sections as ATX headings; paragraphs from text runs (`Span` runs with a `plain`/`emphasis`/`strong`/`code` style, `Link` runs to a URL, `Ref` runs linking to another content block's anchor, and query-produced values styled through nested `SpanColumn`/`LinkColumn` column runs); GitHub-flavored pipe tables with the projected column names (one subtable per group value when the table has a `groupBy` column); bullet and numbered lists; diagram blocks as fenced ` ```mermaid ` blocks rendered through the view engine (a table-kind view as a pipe table), with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction. Markdown metacharacters in content are escaped. Markdown is the only form the REPL writes; the CLI's `-doc-form html` renders the same document tree as semantic HTML ([Rendering a document as HTML](cli.md#rendering-a-document-as-html)) and `-doc-form pdf` converts the Markdown to PDF ([Rendering a document as PDF](cli.md#rendering-a-document-as-pdf)). See the [document generation manual](../manual/README.md) |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%invoke <object> <op> [<p>=<expr>]` | Invoke an operation of an object's type (an action it owns), performed by that object (`%invoke car start`, `%invoke #3 start`, `%invoke car.engine start`), with each argument written as `<parameter>=<expression>`. Assignments in the body write that object's feature values; declared outputs are reported. Not yet supported: an operation given as a `calc` or `constraint`, and positional arguments |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor) |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element |
| `%check <name>` | **Experimental.** Ask an external SMT solver whether a constraint, requirement or satisfaction assertion *can* be satisfied, and on `sat` print an assignment. Reports `sat`, `unsat` or `unknown`, kept distinct. Needs `z3` or `cvc5` on `PATH` (or `OPENSYSML_SMT`; see [installing a solver](../guide/01-install.md#installing-a-solver-optional)) and reports an error rather than a verdict when none is installed. Satisfiability is not evaluation: use `%constraint`/`%satisfy` to find out what holds for an object |
| `%explain <name>` | **Experimental.** When `%check` answers `unsat`, ask the solver *which* conditions conflict. Prints an unsat core, reduced to a minimal one, as the role, the condition as written, the declaring element and `file:line:col`, in the query's order. A declared domain (a `Natural` being non-negative) or a division well-definedness guard can be one of the conflicting conditions. On `sat` there is no conflict to explain and `%check` gives the assignment; on `unknown` no explanation is available. Same solver requirements as `%check` ([installing a solver](../guide/01-install.md#installing-a-solver-optional)); `OPENSYSML_SMT_CORE_BUDGET` bounds the reduction |
| `%solve <name>` | **Experimental.** Ask the solver for values that satisfy a constraint, requirement or satisfaction assertion, keeping what is already fixed: the values an object holds, or failing that the ones the model declares, stay fixed and the rest are synthesised. Prints what was fixed (and by what), the values chosen, and a reminder that they are *one* witness of possibly many. `unsat` here means no values exist that are consistent with what is fixed, and names the fixed values that conflict. Same solver requirements as `%check` |
| `%configure <name> [<variation>=<variant>...] [all [<count>]]` | **Experimental.** Ask which variants a constraint, requirement or satisfaction assertion permits. With no argument, one consistent selection is synthesised. With `<variation>=<variant>`, the chosen selection is checked and the conflict is named when it is not consistent. With `all`, the consistent selections are enumerated up to `OPENSYSML_SMT_MAX_CONFIGURATIONS` (`all <count>` for a smaller bound), and the report says whether the list is complete or was cut short, either at the bound or because the solver stopped deciding or ran out of time; the selections found so far are still reported. An element that reads no variation point is an error pointing at `%check`. Same solver requirements as `%check` |
| `%optimize <name>` | **Experimental.** Ask the solver for the best values an `analysis def` (or an analysis usage) admits. Each `objective` is improved as the trade-study definition typing it says (`TradeStudies::MinimizeObjective` or `MaximizeObjective`), over the value it gives for the library's `best` feature (`attribute :>> best = expression;`), within the conditions the case requires or assumes and the ones the objective states itself. Several objectives are improved lexicographically in declaration order. Prints each optimum with its declared unit and the assignment that attains it. An objective that improves without limit, or a bound no assignment attains, is reported as such and never as a number, and every optimum is verified before it is reported. **Needs `z3`**: optimization is a z3 extension, and a backend without it (cvc5) is an error rather than a plain satisfiability check presented as an optimum. Otherwise the same solver requirements as `%check` |
| **Action debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%action <name> [<object>]` | Start an action debugging session, optionally performed by an instantiated object, given as an [object reference](#object-references) (`%action tally car`, `%action tally #3`) |
| `%step` | Advance one token step |
| `%continue` | Run the action to completion |
| `%tokens` | Show the active tokens |
| `%break <node>` | Set a breakpoint at a node |
| `%stop` | Stop the current debugging session |
| **State machine debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%state <name> [<object>]` | Debug the machine an object exhibits (`%state <object>` after `%instantiate` attaches to that object's own running machine, whether the object is named, `#3`, or `car.controller`), or start a state machine — named, or the object of one the session holds (`%state #2`, `%state monitor.modes`), which exhibits none and so runs afresh — optionally performed by an instantiated object. `%step`, `%advance`, `%current` and `%events` then drive that object's machine, and `%features` shows what it wrote |
| `%events` | Show the event queue |
| `%current` | Show the current state and configuration |
| `%advance <time>` | Advance simulation time by `<time>` units, processing every event due |
| **Control** | |
| `%quit` | Exit the REPL |
| `Tab` | Complete meta commands, symbol names (after `%print`, `%instantiate`, `%features` …; a name that needs quoting is offered in quotes, `Q::'the ra` completing to `Q::'the rack'`), object references where a command takes one (`#` offers the ids there are; `car.` offers the parts `car` holds, a multi-valued one as `car.wheels[1]`, `car.wheels[2]` …; completing reads and materializes nothing, so a part no command has reached yet is offered by type, and only the elements its lower bound guarantees), the form after `%render <name>`, and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |

## Object references

An object reference names one object the session holds. It is one of:

| Form | Denotes |
|------|---------|
| `car`, `Demo::car` | the object `%instantiate car` created, by the name it was created under (unqualified or qualified, quoted segments included) |
| `#3` | the object whose id `%instantiate` printed as `ID: 3`. Ids count up from 1 and never change: an object keeps its id when a later submission carries it over, and when a second `%instantiate` of its name creates a new object — the name then denotes the new one, and `#3` is how the old one is reached (`%instances` lists it as `#3 (ID: 3, formerly Demo::car)`) |
| `car.fl`, `#3.fl`, `car.fl.hub` | a path from a named object or an id through the part features it holds, one nested object per segment. `.` and `::` are interchangeable in a path, so `car::fl` and `car.fl` are the same object; `.` is the shorter spelling and `::` is what the named root itself may contain |
| `car.wheels[2]` | one element of a multi-valued part feature (`part wheels : Wheel[4]`), counted from 1 in the order the feature holds them |

Reading a path materializes the nested parts it passes through, exactly as `%features car` does.

Every command reports a bad reference in the same words:

```
sysml> %features #9
error: no object has id #9 (the objects are #1, #2)
sysml> %features car.nope
error: Demo::car has no feature "nope" (its features are fl, mass, wheels)
sysml> %features car.mass
error: mass of Demo::car holds a value (1500.0), not an object
sysml> %features car.wheels
error: wheels of Demo::car holds 4 objects: pick one by index, wheels[1] to wheels[4]
sysml> %features car.wheels[5]
error: wheels of Demo::car holds 4 objects, so wheels[5] names none (indexes run from 1 to 4)
sysml> %features Wheel
error: no instance of "Demo::Wheel" (use %instantiate first)
```

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
