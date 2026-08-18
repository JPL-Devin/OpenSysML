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
| `%print [name]` | Print the session model as SysML notation at the prompt, or just the named element and its body (`%print 'My Pkg'::Car`). Comments are kept, since the same writer `%save` writes notation with is used, and what is printed can be typed back in. Notation only: nothing about RDF is reported. Reading the model materializes nothing and leaves a debugging session running |
| `%save <file>` | Write the session model to a file: `.sysml` notation (comments preserved) or `.ttl` RDF, which is [experimental](rdf-mapping.md#status-experimental) and reported as such on each save |
| `%verbosity [level]` | Show or set output level: `quiet` (errors only), `normal`, `debug` (every diagnostic over the whole buffer) |
| `%trace [on\|off]` | Show or set execution tracing: each evaluation, calc invocation, action step and state transition |
| `%budget` | Show the five bounds one run may spend, each with the variable that raises it |
| **Library Discovery** | |
| `%search <substring>` | List the declared and library symbols whose qualified name contains the substring, with the kind of each |
| `%builtins` | List the library functions the runtime implements directly (`sqrt`, `abs`, `max`, `floor`, `x->isEmpty()`, `x->sum()` …) |
| `%view <name>` | Show what a view exposes — its own `expose` relationships and the protected ones of the views it specializes — the views nested in it, each of which is asked for its own exposed set, and its conformance to every viewpoint it satisfies: a verdict of `conforms`, `violated` or `unevaluable` per viewpoint and per framed concern, with the reason, the exposed element a concern's condition failed for, and `(from <view>)` where the `satisfy` is inherited. Asking it of an element that is no view says so |
| `%render <name> [form]` | Render a view: the exposed set as the rendering the view's `render` member states — a containment tree with nested views as subtrees, an interconnection diagram of the exposed parts and the connections between them, a state machine's states and transitions, an action's nodes and successions, or a table of the exposed elements and what they declare — and a tree where the view states none. Written as indented text, or in the machine-readable form of the kind: a [Mermaid](#rendering-a-view) diagram with `mermaid`, a Markdown table with `markdown`; a form the kind is not written in names the one it is. Read-only: it creates no object and leaves a `%action`/`%state` debugging session running. A view exposing nothing renders empty and says so; a rendering kind this build does not produce names the kind and the view rather than rendering something else; an element the rendering cannot represent is reported, not dropped |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create instance from part definition |
| `%features <name>` | Show what an object holds for each feature of its type |
| `%slots <name>` | Deprecated spelling of `%features`: the same listing, led by a note naming the command to write instead |
| `%instances` | List all created objects |
| `%eval <expr>` | Evaluate expression, in the last namespace the session declared |
| `%eval in <name> : <expr>` | Evaluate expression in the named element's own namespace, or — when an object was materialized under that name — on that object, so a feature reads its value. The separator is the first `:` outside a quoted name that is not part of a `::`, so `%eval in Demo : Vehicle::mass` works |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor) |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element |
| `%check <name>` | **Experimental.** Ask an external SMT solver whether a constraint, requirement or satisfaction assertion *can* be satisfied, and on `sat` print an assignment. Reports `sat`, `unsat` or `unknown`, kept distinct; needs `z3` or `cvc5` on `PATH` (or `OPENSYSML_SMT`) — [installing a solver](../guide/01-install.md#installing-a-solver-optional) — and reports an error rather than a verdict when none is installed. Satisfiability is not evaluation: use `%constraint`/`%satisfy` for what holds of an object |
| `%explain <name>` | **Experimental.** When `%check` answers `unsat`, ask the solver *which* conditions conflict: an unsat core, reduced to a minimal one, printed as the role, the condition as written, the declaring element and `file:line:col` in the query's order. A declared domain (a `Natural` being non-negative) or a division well-definedness guard can be a conflicting condition. On `sat` there is no conflict to explain and `%check` gives the assignment; on `unknown` no explanation is available. Same solver requirements as `%check` ([installing a solver](../guide/01-install.md#installing-a-solver-optional)), and `OPENSYSML_SMT_CORE_BUDGET` bounds the reduction |
| `%solve <name>` | **Experimental.** Ask the solver for values that satisfy a constraint, requirement or satisfaction assertion, keeping what is already fixed: the values an object holds, else the ones the model declares, are fixed and the rest are synthesised. Prints what was fixed (and by whom), the values chosen, and that they are *one* witness of possibly many. `unsat` here means no values exist consistent with what is fixed, and names the fixed values that conflict. Same solver requirements as `%check` |
| `%configure <name> [<variation>=<variant>...] [all [<count>]]` | **Experimental.** Ask which variants a constraint, requirement or satisfaction assertion permits. With no argument one consistent selection is synthesised; with `<variation>=<variant>` the chosen selection is checked and the conflict named when it is not consistent; with `all` the consistent selections are enumerated up to `OPENSYSML_SMT_MAX_CONFIGURATIONS` (`all <count>` for a smaller bound), and the report says whether they are all of them or were cut short — at the bound, or when the solver stopped deciding or ran out of time, in which case the selections found so far are still reported. An element reading no variation point is an error pointing at `%check`. Same solver requirements as `%check` |
| **Action debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%action <name> [<object>]` | Start an action debugging session, optionally performed by an instantiated object |
| `%step` | Advance one token step |
| `%continue` | Run the action to completion |
| `%tokens` | Show the active tokens |
| `%break <node>` | Set a breakpoint at a node |
| `%stop` | Stop the current debugging session |
| **State machine debugging** ([guide chapter 6](../guide/06-behavior.md)) | |
| `%state <name> [<object>]` | Start a state machine debugging session, optionally performed by an instantiated object |
| `%events` | Show the event queue |
| `%current` | Show the current state and configuration |
| `%advance <time>` | Advance simulation time by `<time>` units, processing every event due |
| **Control** | |
| `%quit` | Exit the REPL |
| `Tab` | Complete meta commands, symbol names (after `%print`, `%instantiate`, `%features` …), the form after `%render <name>`, and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |

## Rendering a view

```
sysml> %render Demo::summary
Demo::summary — tree rendering (the view states no rendering; a tree is the default)

part def Demo::Vehicle
part Demo::v (Vehicle)
view Demo::summary::detail
  part def Demo::Wheel

sysml> %render Demo::summary mermaid
%% Demo::summary — tree rendering
flowchart TD
  n0["part def Demo::Vehicle"]
  n1["part Demo::v (Vehicle)"]
  n2["view Demo::summary::detail"]
  n3["part def Demo::Wheel"]
  n2 --- n3
```

A view stating `render asElementTable;` — or typed by `StandardViewDefinitions::GridView` — renders
as rows instead: the exposed elements, the elements declared in them, and the views nested in the
rendered one, as aligned columns at the prompt and as a Markdown table with `markdown`.

```
sysml> %render Demo::parts
Demo::parts — table rendering (render asElementTable)

Element        Kind      Type   Declared in
-------------  --------  -----  -------------
Demo::Vehicle  part def
wheel          part      Wheel  Demo::Vehicle

sysml> %render Demo::parts markdown
<!-- Demo::parts — table rendering (render asElementTable) -->
| Element | Kind | Type | Declared in |
| --- | --- | --- | --- |
| Demo::Vehicle | part def |  |  |
| wheel | part | Wheel | Demo::Vehicle |
```

The rendering is **tool-defined output**: SysML v2 §10.2 specifies the notation a view is written
in, not how a tool draws it. Mermaid is the machine-readable form of the graph-shaped kinds because
it renders as-is in Markdown, documentation sites and editors without a separate rendering tool, and
has a dedicated state diagram grammar; a table is a Markdown table, which Mermaid has no grammar
for. A state rendering reads the lowered state graph and an action rendering the
lowered action graph, so what is drawn is what the runtime executes.

Rendering a view outside the prompt is [`sysml -render`](cli.md#rendering-a-view).
