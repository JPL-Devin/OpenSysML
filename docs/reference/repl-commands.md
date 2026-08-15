# REPL meta-commands

Every command `sysml` answers at the prompt. The session model behind them — what a submission
replaces, what it drops — is [guide chapter 4](../guide/04-repl.md).

| Command | Description |
|---------|-------------|
| `%help` | Show help message |
| `%list` | List all declarations in current session |
| `%clear` | Clear session (reset all declarations) |
| `%load <path>...` | Submit the contents of files, directories or globs |
| `%save <file>` | Write the session model to a file: `.sysml` notation (comments preserved) or `.ttl` RDF |
| `%verbosity [level]` | Show or set output level: `quiet` (errors only), `normal`, `debug` (every diagnostic over the whole buffer) |
| `%trace [on\|off]` | Show or set execution tracing: each evaluation, calc invocation, action step and state transition |
| `%budget` | Show the five bounds one run may spend, each with the variable that raises it |
| **Library Discovery** | |
| `%search <substring>` | List the declared and library symbols whose qualified name contains the substring, with the kind of each |
| `%builtins` | List the library functions the runtime implements directly (`sqrt`, `abs`, `max`, `floor`, `x->isEmpty()`, `x->sum()` …) |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create instance from part definition |
| `%slots <name>` | Show instance slots and values |
| `%instances` | List all created instances |
| `%eval <expr>` | Evaluate expression |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor) |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element |
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
| `Tab` | Complete meta commands, symbol names, and file paths after `%load` and `%save` |
| `Ctrl-D` | Exit REPL |
