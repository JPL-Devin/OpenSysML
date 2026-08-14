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

A pipeline cannot gate on the exit status of an evaluation: an expression that
could not be evaluated still exits `0` (see [Exit status](#exit-status)). So the
check reads the output, and has to notice the `error:` line — a missing result
otherwise looks exactly like a wrong one:

```bash
# Check that a calculated value matches what is expected
expected=42
output=$(sysml -e "designParameter" design.sysml) || exit 2   # the file would not load
if printf '%s\n' "$output" | grep -q '^error:'; then
    printf '%s\n' "$output" >&2
    echo "✗ designParameter was not evaluated" >&2
    exit 2
fi
actual=$(printf '%s\n' "$output" | awk '/^ *=/ {print $2}')
if [ "$actual" = "$expected" ]; then
    echo "✓ Design parameter validated"
else
    echo "✗ Design parameter mismatch: expected $expected, got $actual" >&2
    exit 1
fi
```

Constraint, requirement and satisfy verdicts — `%constraint`, `%requirement`,
`%satisfy` — have no non-interactive form today: they are reachable only from the
REPL, so a pipeline that needs them drives it over a pipe as in
[§ 6](#6-use-repl-meta-commands) and reads the `✓`/`✗` lines.

### 6. Use REPL Meta Commands

Load a file and use meta commands:

```bash
echo "%load model.sysml
%instantiate Vehicle
%slots Vehicle
%eval speedLimit" | sysml
```

## Command Reference

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--eval <expr>` | `-e` | Evaluate expression and exit (repeatable) |
| `--debug` | | Report every diagnostic over the whole session buffer, with the pass that produced it |
| `--quiet` | | Report errors only, suppressing warnings |
| `--trace` | | Report each execution step: expression evaluation, calc invocation, action tokens, state transitions |
| `--convert <format>` | | Convert the model instead of running it: `sysml`, `kerml`, `ttl`, `turtle` or `rdf` (see [RDF_INTEROP.md](../docs/RDF_INTEROP.md)) |
| `--from <format>` | | Input format for `--convert` (default: from the input's extension) |
| `--output <file>` | `-o` | Write the conversion to a file instead of stdout |
| `--version` | `-v` | Show version information |
| `--help` | `-h` | Show usage information |

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

**Model diagnostics and failed evaluations go to stdout, and do not change the
exit status.** Only the command's own failures — a file it could not read, a
conversion it could not make, a misused flag — are written to stderr.

A file that cannot be read is reported on stderr and exits `1` (the message
appears twice, once from the loader and once from the command):

```bash
$ sysml missing.sysml
error loading missing.sysml: load missing.sysml: open missing.sysml: no such file or directory
sysml: load missing.sysml: open missing.sysml: no such file or directory
$ echo $?
1
```

A model that does not analyse cleanly is still loaded, and its diagnostics are
part of the normal output — redirecting stderr does not capture them:

```bash
$ sysml -e "1+1" bad.sysml 2>/dev/null
2:27: error: expected an expression
  attribute mass : Real = ;
                          ^
✓ 1+1
  = 2
$ echo $?
0
```

An expression that cannot be evaluated reports itself the same way, on stdout
with status `0`:

```bash
$ sysml -e "Demo::Vehicle::nope" model.sysml
error: symbol "Demo::Vehicle::nope" not found
$ echo $?
0
```

So `2> errors.log` collects the command's own failures only. A script that needs
to know whether a model analysed, or whether an expression produced a value, has
to read stdout for an `error:` line.

## Exit status

The whole contract, as the command behaves today. This is the one place it is
written down; [docs/QUICKSTART.md](../docs/QUICKSTART.md) links here.

| Status | Means |
|--------|-------|
| `0` | The command ran: every file named was loaded and every `-e` expression was submitted. Diagnostics in the model, and an expression that could not be evaluated, are reported on stdout and leave the status `0`. |
| `1` | A file could not be loaded, or a `-convert` could not be made. Reported on stderr; a refused conversion writes nothing. |
| `2` | The command line or the environment was wrong: an unknown flag, `-debug` with `-quiet`, the retired `-to`, or an invalid `SYSML_MAX_*` value. Reported on stderr; nothing is loaded. |

```bash
$ sysml -debug -quiet model.sysml; echo $?
sysml: -debug and -quiet are mutually exclusive
2

$ SYSML_MAX_STEPS=abc sysml -e "1+1"; echo $?
sysml: SYSML_MAX_STEPS="abc" is not an integer: set it to a positive number of evaluation steps (default 10000000)
2

$ sysml examples/state-machine-demo.sysml -convert ttl; echo $?
sysml: cannot convert the *ast.SubstateMember at examples/state-machine-demo.sysml:7:13
1
```

Note what is **not** in this table: no status means "a constraint answered false"
or "the model did not analyse". Checking a model from a script therefore means
reading output, as [§ 5](#5-cicd-integration) does.

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
  command ran, not that the model was sound
- Model diagnostics and failed evaluations are on **stdout**; only the command's
  own failures are on stderr, so redirecting stderr does not separate them
- Use shell pipes for REPL automation: `echo "%load file.sysml" | sysml`
