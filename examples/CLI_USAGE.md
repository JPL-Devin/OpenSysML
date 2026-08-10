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

**Note:** Options (`-e`) must come BEFORE positional arguments (files).

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

```bash
# Check that calculated value matches expected
expected=42
output=$(sysml -e "designParameter" design.sysml)
actual=$(echo "$output" | grep "=" | awk '{print $2}')
if [ "$actual" = "$expected" ]; then
    echo "✓ Design parameter validated"
    exit 0
else
    echo "✗ Design parameter mismatch: expected $expected, got $actual"
    exit 1
fi
```

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
| `--version` | `-v` | Show version information |
| `--help` | `-h` | Show usage information |

**Arguments:**
- `[file...]` - SysML files to load (loaded in order)

**Usage pattern:**
```
sysml [options] [file...]
```

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

Errors are written to stderr with exit code 1:

```bash
sysml missing.sysml
# stderr: error loading missing.sysml: open missing.sysml: no such file or directory
# exit code: 1

sysml -e "nonexistent" model.sysml 2> errors.log
# stderr written to errors.log
```

## Use Cases

1. **Quick Calculations**: Use as a calculator with `-e`
2. **Automated Testing**: Run model evaluations in CI/CD pipelines
3. **Scripting**: Extract calculated values for other tools
4. **Validation**: Check model properties without manual interaction
5. **Batch Processing**: Process multiple models programmatically
6. **Interactive Development**: Load files and explore in REPL

## Tips

- Put options (`-e`, `-v`) **before** file arguments
- Use `%help` in REPL to see all meta commands
- Combine multiple `-e` flags to evaluate related expressions
- Load common definitions before custom models
- Check exit codes: 0 = success, 1 = error
- Redirect stderr separately from stdout for error handling
- Use shell pipes for REPL automation: `echo "%load file.sysml" | sysml`
