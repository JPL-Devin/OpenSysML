# SysML CLI Usage Examples

## Interactive Mode (Default)

Start the REPL with no arguments:

```bash
sysml
```

## Non-Interactive Mode

Execute commands and exit without entering interactive mode.

### Basic Evaluation

Load a model and evaluate an expression:

```bash
sysml --load model.sysml --eval someAttribute
# Output: value of someAttribute
```

### Short Flags

Use abbreviated flags for convenience:

```bash
sysml -l model.sysml -e x
```

### Multiple Evaluations

Evaluate multiple expressions in sequence:

```bash
sysml -l model.sysml -e x -e y -e z
```

### Multiple Files

Load multiple files before evaluating:

```bash
sysml -l types.sysml -l instances.sysml -e result
```

### Quiet Mode

Suppress prompts and decorations for clean output (useful for scripting):

```bash
sysml --quiet --load model.sysml --eval result
# Output: only the value

sysml -q -l model.sysml -e output > result.txt
```

## Real-World Examples

### 1. Validate Model and Check Constraint

```bash
sysml -l vehicle-model.sysml -e "speedLimit < 120"
```

### 2. Extract Calculated Values

```bash
# model.sysml contains: attribute totalCost = partCost + laborCost;
sysml -q -l model.sysml -e totalCost
# Output: 1500.00
```

### 3. Batch Processing

```bash
#!/bin/bash
for model in models/*.sysml; do
    result=$(sysml -q -l "$model" -e result)
    echo "$model: $result"
done
```

### 4. CI/CD Integration

```bash
# Check that calculated value matches expected
expected=42
actual=$(sysml -q -l design.sysml -e designParameter)
if [ "$actual" = "$expected" ]; then
    echo "✓ Design parameter validated"
    exit 0
else
    echo "✗ Design parameter mismatch: expected $expected, got $actual"
    exit 1
fi
```

### 5. Semantic Layer Demo

```bash
# Evaluate operators and expressions
sysml -l examples/semantic-layer/demo.sysml -e intEquality -e usePi -e expr1

# Quiet mode for scripting
sysml -q -l examples/semantic-layer/demo.sysml -e intEquality
# Output:   = true
```

## Command Reference

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--load <file>` | `-l` | Load a SysML file (repeatable) |
| `--eval <expr>` | `-e` | Evaluate an expression (repeatable) |
| `--quiet` | `-q` | Suppress prompts and decorations |
| `--version` | `-v` | Show version information |
| `--help` | | Show usage information |

## Output Modes

### Normal Mode (default)

Includes prompts, checkmarks, and decorative output:

```bash
$ sysml -l demo.sysml -e x
package Demo
✓ x
  = 42
```

### Quiet Mode

Only essential output:

```bash
$ sysml -q -l demo.sysml -e x
  = 42
```

## Error Handling

Errors are written to stderr:

```bash
sysml -l missing.sysml -e x
# stderr: sysml: load missing.sysml: no such file or directory
# exit code: 1

sysml -q -l model.sysml -e nonexistent 2> errors.log
# stderr written to errors.log
```

## Piping (Alternative to --load/--eval)

You can still use stdin piping for interactive-style input:

```bash
echo "%load model.sysml" | sysml
echo -e "%load model.sysml\n%eval x" | sysml
```

## Use Cases

1. **Automated Testing**: Run model evaluations in CI/CD pipelines
2. **Scripting**: Extract calculated values for other tools
3. **Validation**: Check model properties without manual interaction
4. **Batch Processing**: Process multiple models programmatically
5. **Integration**: Use SysML as a calculation engine for other systems

## Tips

- Use `-q` for clean output when piping to other tools
- Combine multiple `-e` flags to evaluate related expressions
- Load common definitions with `-l` before custom models
- Check exit codes: 0 = success, 1 = error
- Redirect stderr separately from stdout for error handling
