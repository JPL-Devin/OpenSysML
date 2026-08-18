# SysML CLI Usage Examples

## Interactive Mode (Default)

Start the REPL with no arguments:

```bash
sysml
```

Load files and enter interactive mode:

```bash
sysml model.sysml
sysml types.sysml instances.sysml
```

## Non-Interactive Mode

Execute expressions and exit without entering interactive mode.

### Basic Evaluation

Evaluate an expression:

```bash
sysml -e "5 + 3"
# Output: ✓ 5 + 3
#           = 8
```

### Load Files and Evaluate

Load a model first, then evaluate:

```bash
sysml -e "someAttribute" model.sysml
```

**Note:** flags may be written before or after the files — `sysml model.sysml -e "x"`
and `sysml -e "x" model.sysml` do the same thing.

### Multiple Evaluations

Evaluate multiple expressions in sequence:

```bash
sysml -e "x" -e "y" -e "z" model.sysml
```

### Multiple Files

Load multiple files before evaluating:

```bash
sysml -e "result" types.sysml instances.sysml
```

## Real-World Examples

### 1. Quick Calculation

```bash
sysml -e "10 * 2 + 5"
# Output: ✓ 10 * 2 + 5
#           = 25
```

### 2. Validate Model and Check Constraint

```bash
sysml -e "speedLimit < 120" vehicle-model.sysml
```

### 3. Extract Calculated Values

```bash
# model.sysml contains: attribute totalCost = partCost + laborCost;
sysml -e "totalCost" model.sysml
# Output: ✓ totalCost
#           = 1500.00
```

### 4. Batch Processing

```bash
#!/bin/bash
for model in models/*.sysml; do
    result=$(sysml -e "result" "$model" 2>&1 | grep "=" | awk '{print $2}')
    echo "$model: $result"
done
```

### 5. CI/CD Integration

A pipeline gates on the exit status: an expression that could not be evaluated
exits `2`, and only a value on stdout is left to compare (see
[Exit status](#exit-status)):

```bash
# Check that a calculated value matches what is expected
expected=42
actual=$(sysml -e "designParameter" design.sysml | awk '/^ *=/ {print $2}') || exit $?
if [ "$actual" = "$expected" ]; then
    echo "✓ Design parameter validated"
else
    echo "✗ Design parameter mismatch: expected $expected, got $actual" >&2
    exit 1
fi
```

Constraint, requirement and satisfy verdicts gate the same way, and report
themselves:

```bash
sysml -satisfy -constraint MassBudget design.sysml   # 0 held, 1 answered false, 2 undecided
```

### 6. Use REPL Meta Commands

Load a file and use meta commands:

```bash
echo "%load model.sysml
%instantiate Vehicle
%features Vehicle
%eval speedLimit" | sysml
```

## Command Reference

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--eval <expr>` | `-e` | Evaluate expression and exit (repeatable) |
| `--debug` | | Report every diagnostic over the whole session buffer, with the pass that produced it |
| `--quiet` | | Report errors only, suppressing warnings |
| `--trace` | | Report each execution step: expression evaluation, calc invocation, action tokens, state transitions |
| `--convert <format>` | | Convert the model instead of running it: `sysml`, `kerml`, `ttl`, `turtle` or `rdf`. RDF is [experimental](rdf-mapping.md#status-experimental) and every run that converts it says so on stderr (see [the RDF mapping](rdf-mapping.md)) |
| `--from <format>` | | Input format for `--convert` (default: from the input's extension) |
| `--render <view>` | | Render this view of the model instead of running it, in the form its `render` member states (see [Rendering a view](#rendering-a-view)) |
| `--render-form <form>` | | Form `--render` writes: `mermaid` (default) or `text` |
| `--output <file>` | `-o` | Write the conversion or the rendering to a file instead of stdout |
| `--version` | `-v` | Show version information |
| `--help` | `-h` | Show usage information |

Check flags, each repeatable. `-instantiate` runs first whatever order they are
written in, so the verdicts after it are about that object:

| Flag | Checks |
|------|--------|
| `-validate` | Nothing about the model's conditions: only that it analyses cleanly, and that the objects `-instantiate` asked for materialized |
| `-constraint <name>` | One constraint, as `%constraint` does |
| `-requirement <name>` | One requirement, as `%requirement` does |
| `-satisfy` | Every satisfaction assertion the model states |
| `-satisfy=<name>` | Only the assertions the named element states (`-satisfy=false` asks for none) |
| `-instantiate <name>` | Creates an object first, so the verdicts are about it |
| `-calc "<name>(<args>)"` | Invokes a calculation and reports what it computed |
| `-action "<name> [object]"` | Runs an action to completion and reports its outputs |
| `-state "<name> [object]"` | Runs a state machine and reports where it settled |
| `-advance <time>` | Simulated time units each `-state` machine is run for |
| `-json` | Reports the checks as one JSON document rather than as lines |

**Arguments:**
- `[file...]` - SysML files to load (loaded in order)

**Usage pattern:**
```
sysml [options] [file...]
```

Flags may be written before or after the files. A file named like a flag is read
as a file after `--`, which ends the flags: `sysml -trace -- -m.sysml`.

## Examples

```bash
# Interactive REPL
sysml

# Load file and start REPL
sysml model.sysml

# Evaluate and exit
sysml -e "5 + 3"

# Load file, evaluate, and exit
sysml -e "expr" file.sysml

# Multiple evaluations
sysml -e "x" -e "y" file.sysml

# Multiple files
sysml -e "result" file1.sysml file2.sysml
```

## Rendering a view

`-render <view>` renders one view of the model and exits. The rendering is the one the view's
`render` member states, and a containment tree where it states none; the kinds this build produces
are a tree, an interconnection diagram, a state machine and an action flow.

```bash
# The view as a Mermaid diagram, on stdout
sysml model.sysml -render Views::vehicleView

# The indented text form a person reads
sysml model.sysml -render Views::vehicleView -render-form text

# Write the rendering to a file
sysml model.sysml -render Views::vehicleView -o view.mmd
```

The artifact is the run's result, so it goes on stdout alone — what was loaded, what the model
analysed to, an empty rendering, and any element the rendering cannot represent all go on stderr,
and `-o` writes the artifact only. A view exposing nothing renders an empty artifact and says so; a
name that is no view, a rendering kind this build does not produce, and a model that did not
analyse cleanly each stop the run with status 2. Rendering decides nothing about the model, so it
is not asked for together with a check flag or with `-convert`.

The rendering is **tool-defined output**: SysML v2 §10.2 specifies the notation a view is written
in, not how a tool draws it. Mermaid is the machine-readable form because it renders as-is in
Markdown, documentation sites and editors without a separate rendering tool, and has a dedicated
state diagram grammar.

## Output Format

All evaluations include checkmark and result:

```bash
$ sysml -e "10 * 2"
✓ 10 * 2
  = 20
```

For file loads, declarations are summarized:

```bash
$ sysml demo.sysml
package Demo
  part def Vehicle {
    attribute speed : Real;
  }
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

## Error Handling

**On every run that is not a prompt, what was asked for goes to stdout and what
went wrong goes to stderr.** Evaluated values, conversion output, verdict lines
and the `✓` echoes of what a load declared are results; model diagnostics —
errors and warnings alike — and anything that stopped the run are findings, and
so is the `wrote <file> …` note a `-convert -o` prints when it succeeds, which is
kept off stdout so a conversion can be piped.

A file that cannot be read ends the run:

```bash
$ sysml missing.sysml
sysml: cannot read missing.sysml: no such file or directory
$ echo $?
2
```

A model that does not analyse cleanly answers nothing, so its diagnostics end the
run rather than an evaluation being reported against a model nobody could read:

```bash
$ sysml -e "1+1" bad.sysml 2>/dev/null
$ echo $?
2
$ sysml -e "1+1" bad.sysml 2>&1 >/dev/null
bad.sysml:2:39: error: expected an expression
  part def Vehicle { attribute mass = ; }
                                      ^
sysml: bad.sysml did not analyse cleanly
```

An expression that cannot be evaluated is reported the same way, leaving what the
load declared on stdout:

```bash
$ sysml -e "Demo::Vehicle::nope" model.sysml
✓ package Demo
sysml: unresolved reference: Demo::Vehicle::nope
$ echo $?
2
```

So `2> errors.log` collects everything a script would otherwise have to read the
results for, plus the `wrote …` note of a successful `-convert -o` and any
warning the model raised — neither of which changes the status, so a non-empty
log is not by itself a failure. The status is.

## Exit status

The whole contract, which is the same whatever the run was asked to do. This is
the one place it is written down; [the guide](../guide/)
links here.

| Status | Means |
|--------|-------|
| `0` | What was asked for was done: every file loaded and analysed cleanly, every `-e` expression produced a value, every check held, a conversion was written. Warnings leave the status `0`. |
| `1` | The model answered false: a constraint, requirement or satisfaction assertion the model decided did not hold. Only a verdict reports this status. |
| `2` | What was asked for could not be done, so the model answered nothing: a file that could not be read, a model that did not analyse cleanly, an object whose feature values did not materialize, an unresolved name, a check that could not be made, a conversion that could not be written, a misused flag or an invalid `SYSML_MAX_*` value. |

```bash
$ sysml -constraint MassBudget model.sysml; echo $?      # a verdict the model decided
✗ Constraint MassBudget failed
1

$ sysml -debug -quiet model.sysml; echo $?
sysml: -debug and -quiet are mutually exclusive
2

$ SYSML_MAX_STEPS=abc sysml -e "1+1"; echo $?
sysml: SYSML_MAX_STEPS="abc" is not an integer: set it to a positive number of evaluation steps (default 10000000)
2

$ sysml examples/state-machine-demo.sysml -convert ttl; echo $?
sysml: cannot convert the substate member at examples/state-machine-demo.sysml:7:13: save to .sysml or .kerml instead, which writes the source exactly; see docs/reference/rdf-mapping.md § Limitations
2
```

Materializing an object is part of the run, so what it finds is a diagnostic
about the model: `-instantiate` reports every feature value it could not materialize —
a default whose value count does not conform to the multiplicity governing its
feature, which is the assumed `1..1` for a feature that declares none — and
`-validate` reports `no errors` only for a run that found none. The prompt surface
follows the same rule: a command that rendered a feature value it could not materialize —
a `%features` listing carrying `<error: …>`, or an `%eval` of such a value, pinned to
a context (`%eval in <name> : <expr>`) or not — answered nothing about it, so a
session driven from a pipe exits `2` rather than reporting success, whatever
analysis found. A name that is no feature of the object is a request the command got
wrong, not a feature value that failed to materialize, and does not change the status.

```bash
$ sysml model.sysml -instantiate test::craft -validate; echo $?
error: feature value craft.volumes: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1
sysml: model.sysml did not materialize cleanly
2

$ printf '%%instantiate test::craft\n%%features test::craft\n' | sysml model.sysml; echo $?
Instance: test::craft (ID: 1)
Features:
  volumes: <error: feature value craft.volumes: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1>
2
```

Nesting multiplies, and reading a feature value materializes the objects it holds, so the
check is bounded, as the `%features` listing is: a model wide enough to spend that
budget, deeper than the walk descends, or one whose part holds its own kind is
reported as checked in part (`warning: … materialization is bounded; not every
feature value was checked`, and `no errors in the feature values checked`) rather than read to the
end. Being no model error, that leaves the status `0`.

The prompt is the exception: a line it could not carry out is reported and the
session goes on, and `%quit` or Ctrl-D exits `0`. `sysml model.sysml` at a
terminal loads the model, reports what analysis found, and opens the prompt with
status `0` — the prompt is where the model gets fixed. The same command with its
lines coming from a pipe or a file gates: it exits `2` for a model that did not
analyse cleanly, and for one whose feature values a command could not materialize.

## Use Cases

1. **Quick Calculations**: Use as a calculator with `-e`
2. **Automated Testing**: Run model evaluations in CI/CD pipelines
3. **Scripting**: Extract calculated values for other tools
4. **Validation**: Check model properties without manual interaction
5. **Batch Processing**: Process multiple models programmatically
6. **Interactive Development**: Load files and explore in REPL

## Tips

- Use `%help` in REPL to see all meta commands
- Combine multiple `-e` flags to evaluate related expressions
- Load common definitions before custom models
- Read [Exit status](#exit-status) before gating a pipeline on one: `0` means the
  model answered what was asked, `1` that it answered false, `2` that it answered
  nothing
- Results are on **stdout** and findings — diagnostics, warnings, whatever
  stopped the run — on **stderr**, so `> model.ttl` and `2> errors.log` separate
  them
- Use shell pipes for REPL automation: `echo "%load file.sysml" | sysml`
