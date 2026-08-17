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
| `%check <name>` | **Experimental.** Ask an external SMT solver whether a constraint, requirement or satisfaction assertion *can* be satisfied, and on `sat` print an assignment. Reports `sat`, `unsat` or `unknown`, kept distinct; needs `z3` or `cvc5` on `PATH` (or `SYSTEMICA_SMT`), and reports an error rather than a verdict when none is installed. Satisfiability is not evaluation: use `%constraint`/`%satisfy` for what holds of an object |
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
| `Tab` | Complete meta commands, symbol names (after `%print`, `%instantiate`, `%features` …), and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |
