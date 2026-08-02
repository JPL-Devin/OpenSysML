# SysML v2 Training Examples

Official training examples from the [OMG SysML v2 Pilot Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) (2026-05 release).

These examples demonstrate real-world SysML v2 modeling patterns and are maintained by the OMG SysML v2 specification team.

## Contents

- **01. Packages** - Basic package organization and namespacing
- **02. Part Definitions** - Defining reusable part structures
- **07. Parts** - Part usages and instantiation
- **09. Connections** - Connecting parts together
- **10. Ports** - Interface points for parts
- **15. Actions** - Action definitions and execution
- **24. States** - State machine modeling
- **29. Expressions** - Value expressions and calculations
- **30. Calculations** - Calc definitions and invocations
- **31. Constraints** - Constraint definitions and usage
- **32. Requirements** - Requirement modeling
- **33. Analysis** - Analysis case definitions

## Usage

### Interactive REPL

```bash
# Load and explore an example
sysml
sysml> %load examples/sysml-v2-training/07.\ Parts/Parts\ Example.sysml
sysml> %list
sysml> %eval <expression>
```

### Non-Interactive

```bash
# Parse and validate
sysml -l "examples/sysml-v2-training/30. Calculations/Calculation Example.sysml"

# Execute calculations
sysml -l "examples/sysml-v2-training/30. Calculations/Calculation Example.sysml" \
      -e "someCalculation"
```

### Testing Parser Coverage

```bash
# Test all training examples parse correctly
go test ./internal/core/parser -v -run TestTrainingExamples
```

## Source

These examples are from the OMG SysML v2 Pilot Implementation 2026-05 release:
- **License:** Eclipse Public License 2.0 (EPL-2.0)
- **Source:** https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation
- **Release:** https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/releases/tag/2026-05

## Systemica Compatibility

Systemica aims for 100% compatibility with these examples. Current status:

- **Parser:** ✅ All examples parse cleanly (structural syntax complete)
- **Semantic layer:** ✅ Name resolution, type checking, validation rules
- **Runtime:** ✅ Expression evaluation, calculations, constraints
- **Actions/States:** ✅ Behavioral execution (token-flow semantics, state machines)

Report parsing issues or semantic gaps as GitHub issues with the specific example file.

## Learning Path

Recommended order for learning SysML v2:

1. **01. Packages** - Understand namespace organization
2. **02. Part Definitions** - Learn structural modeling
3. **07. Parts** - Practice part instantiation
4. **09. Connections** - Connect parts together
5. **29. Expressions** - Work with values and expressions
6. **30. Calculations** - Define and invoke calculations
7. **31. Constraints** - Add validation rules
8. **15. Actions** - Model behavior with actions
9. **24. States** - Create state machines
10. **32. Requirements** - Specify requirements
11. **33. Analysis** - Perform trade studies and analyses

## Additional Resources

- **Full training set:** All 42 modules available in pilot implementation
- **Validation examples:** `sysml/src/validation/` in pilot implementation
- **Spec examples:** Direct examples from SysML v2 specification
- **OMG Specification:** https://www.omg.org/spec/SysML/2.0

## Contributing

Found an example that doesn't work in Systemica? Please report:
1. The specific `.sysml` file path
2. Command you ran
3. Expected vs actual behavior
4. Any error messages

This helps us track spec compliance and improve Systemica.
