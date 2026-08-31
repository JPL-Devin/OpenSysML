# REPL meta-commands

Every command `sysml` answers at the prompt. The session model behind them — what a submission
replaces, what it drops — is [guide chapter 4](../guide/04-repl.md).

Every command taking a `<name>` accepts the quoted spelling the notation writes, including a
quoted segment holding a space and one in the middle of a chain: `%instantiate 'My Pkg'::Car`,
`%features Top::'My Pkg'::Car`.

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
| `%view <name>` | Show what a view exposes — its own `expose` relationships and the protected ones of the views it specializes — the views nested in it, each of which is asked for its own exposed set, and its conformance to every viewpoint it satisfies: a verdict of `conforms`, `violated` or `unevaluable` per viewpoint and per framed concern, with the reason, the exposed element a concern's condition failed for, and `(from <view>)` where the `satisfy` is inherited. Asking it of an element that is no view says so |
| `%render <name> [form]` | Render a view: the exposed set as the rendering the view's `render` member states — a containment tree with nested views as subtrees, an interconnection diagram of the exposed parts and the connections between them, a state machine's states and transitions, an action's nodes and successions, or a table of the exposed elements and what they declare — and a tree where the view states none. Written as indented text, or in the machine-readable form of the kind: a [Mermaid](#rendering-a-view) diagram with `mermaid`, a Markdown table with `markdown`; a form the kind is not written in names the one it is. Read-only: it creates no object and leaves a `%action`/`%state` debugging session running. A view exposing nothing renders empty and says so; a rendering kind this build does not produce names the kind and the view rather than rendering something else; an element the rendering cannot represent is reported, not dropped |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create an object of a part definition, and start the behaviors its type exhibits or performs: each object runs its own machine, initialized after its feature values are built and run until it is quiescent. A second `%instantiate` of the same name is a new object, and the name then denotes that one. A later submission keeps the object's identity but restarts its behaviors from their initial states, and says so |
| `%features <name>` | Show what an object holds for each feature of its type |
| `%instances` | List all created objects |
| `%eval <expr>` | Evaluate expression, in the last namespace the session declared |
| `%eval in <name> : <expr>` | Evaluate expression in the named element's own namespace, or — when an object was materialized under that name — on that object, so a feature reads its value. The separator is the first `:` outside a quoted name that is not part of a `::`, so `%eval in Demo : Vehicle::mass` works |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%run-query <name> [<p>=<expr>...]` | Execute a document query — a `calc def` specializing `DocumentQueries::Query` — and print its rows and projected cells. Each entry binding is written as `<parameter>=<expression>`; a name binds the element it denotes, anything else is evaluated as an expression. Named query invocation and named-relationship traversal (`RelatedElements` over specialization, subsetting, redefinition, typing, connection, allocation, satisfaction and verification edges, outgoing or incoming) execute; default-expression evaluation is reported as an error, since the engine does not execute it yet |
| `%render-document <name>` | Compile a document definition — a `part def` specializing `DocumentQueries::Document` — run its queries against the model and print the rendered Markdown. A document binds its queries' parameters in the model, so the name is the whole invocation. The output is deterministic CommonMark: the title and sections as ATX headings, paragraphs from text runs — statically-authored `Span` runs with a `plain`/`emphasis`/`strong`/`code` style, `Link` runs to a URL, and `Ref` runs linking to another content block's anchor — GitHub-flavored pipe tables with the projected column names (a table with a `groupBy` column as one subtable per group value), bullet and numbered lists, diagram blocks as fenced ` ```mermaid ` blocks rendered through the view engine (a table-kind view as a pipe table) with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction, and Markdown metacharacters in content escaped. Markdown is the only document form this build writes |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%invoke <object> <op> [<p>=<expr>]` | Invoke an operation of an object's type — an action it owns — performed by that object, with each argument written as `<parameter>=<expression>`. What the body writes is that object's feature value; declared outputs are reported. Not yet supported: an operation given as a `calc` or `constraint`, and positional arguments |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor) |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element |
| `%check <name>` | **Experimental.** Ask an external SMT solver whether a constraint, requirement or satisfaction assertion *can* be satisfied, and on `sat` print an assignment. Reports `sat`, `unsat` or `unknown`, kept distinct; needs `z3` or `cvc5` on `PATH` (or `OPENSYSML_SMT`) — [installing a solver](../guide/01-install.md#installing-a-solver-optional) — and reports an error rather than a verdict when none is installed. Satisfiability is not evaluation: use `%constraint`/`%satisfy` for what holds of an object |
| `%explain <name>` | **Experimental.** When `%check` answers `unsat`, ask the solver *which* conditions conflict: an unsat core, reduced to a minimal one, printed as the role, the condition as written, the declaring element and `file:line:col` in the query's order. A declared domain (a `Natural` being non-negative) or a division well-definedness guard can be a conflicting condition. On `sat` there is no conflict to explain and `%check` gives the assignment; on `unknown` no explanation is available. Same solver requirements as `%check` ([installing a solver](../guide/01-install.md#installing-a-solver-optional)), and `OPENSYSML_SMT_CORE_BUDGET` bounds the reduction |
| `%solve <name>` | **Experimental.** Ask the solver for values that satisfy a constraint, requirement or satisfaction assertion, keeping what is already fixed: the values an object holds, else the ones the model declares, are fixed and the rest are synthesised. Prints what was fixed (and by whom), the values chosen, and that they are *one* witness of possibly many. `unsat` here means no values exist consistent with what is fixed, and names the fixed values that conflict. Same solver requirements as `%check` |
| `%configure <name> [<variation>=<variant>...] [all [<count>]]` | **Experimental.** Ask which variants a constraint, requirement or satisfaction assertion permits. With no argument one consistent selection is synthesised; with `<variation>=<variant>` the chosen selection is checked and the conflict named when it is not consistent; with `all` the consistent selections are enumerated up to `OPENSYSML_SMT_MAX_CONFIGURATIONS` (`all <count>` for a smaller bound), and the report says whether they are all of them or were cut short — at the bound, or when the solver stopped deciding or ran out of time, in which case the selections found so far are still reported. An element reading no variation point is an error pointing at `%check`. Same solver requirements as `%check` |
| `%optimize <name>` | **Experimental.** Ask the solver for the best values an `analysis def` (or an analysis usage) admits: each `objective` is improved the way the trade-study definition typing it says — `TradeStudies::MinimizeObjective` or `MaximizeObjective` — over the value it states for the library's `best` feature (`attribute :>> best = expression;`), within the conditions the case requires or assumes and the ones the objective states itself. Several objectives are improved lexicographically in declaration order. Prints each optimum with its declared unit and the assignment attaining it. An objective improving without limit, or a bound no assignment attains, is reported as such and never as a number, and every optimum is verified before it is reported. **Needs `z3`**: optimization is a z3 extension, and a backend without it (cvc5) is an error rather than a plain satisfiability check presented as an optimum. Otherwise the same solver requirements as `%check` |
| **Action debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%action <name> [<object>]` | Start an action debugging session, optionally performed by an instantiated object |
| `%step` | Advance one token step |
| `%continue` | Run the action to completion |
| `%tokens` | Show the active tokens |
| `%break <node>` | Set a breakpoint at a node |
| `%stop` | Stop the current debugging session |
| **State machine debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%state <name> [<object>]` | Debug the machine an object exhibits — `%state <part>` after `%instantiate <part>` binds to that object's own running machine — or start a state machine, optionally performed by an instantiated object. `%step`, `%advance`, `%current`, `%events` then drive that object's machine, and `%features` shows what it wrote |
| `%events` | Show the event queue |
| `%current` | Show the current state and configuration |
| `%advance <time>` | Advance simulation time by `<time>` units, processing every event due |
| **Control** | |
| `%quit` | Exit the REPL |
| `Tab` | Complete meta commands, symbol names (after `%print`, `%instantiate`, `%features` …), the form after `%render <name>`, and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |

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

A view stating `render asElementTable;` — or typed by `StandardViewDefinitions::GridView` — renders
as rows instead: the exposed elements, the elements declared in them, and the views nested in the
rendered one, as aligned columns at the prompt and as a Markdown table with `markdown`.

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
in, not how a tool draws it. Mermaid is the machine-readable form of the graph-shaped kinds because
it renders as-is in Markdown, documentation sites and editors without a separate rendering tool, and
has a dedicated state diagram grammar; a table is a Markdown table, which Mermaid has no grammar
for. A state rendering reads the lowered state graph and an action rendering the
lowered action graph, so what is drawn is what the runtime executes.

Rendering a view outside the prompt is [`sysml -render`](cli.md#rendering-a-view).
