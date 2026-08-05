# Grammar Reference

**This directory does not contain grammar files.** The parser in `internal/core/parser/` is hand-written recursive descent.

## OMG Grammar Reference

For reference, the official Xtext grammar files from OMG are available at:

**KerML Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext

**SysML v2 Grammar:**
- https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/blob/master/org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext

These files are licensed under Eclipse Public License 2.0 (EPL-2.0) and are not included in this repository to avoid license mixing.

## Validation

Grammar conformance is validated through parsing **OMG's own files**:

1. **Stdlib Conformance Gate** - 94 OMG standard library files must parse with zero diagnostics
   - See: `internal/core/libs/stdlib_conformance_test.go`
   - These files are the **source of truth** for correct parsing

2. **Training Examples** - 100 OMG training files (63/100 parse clean)
   - See: `docs/TRAINING_EXAMPLES.md`

3. **Golden AST Tests** - 16 fixtures with expected AST output
   - See: `internal/core/parser/testdata/parse/`

4. **Negative Tests** - 15 test cases for error recovery
   - See: `internal/core/parser/negative_test.go`

## Hand-Written Parser

The parser is hand-written for:
- **Performance** - 10-100x faster than generated parsers
- **Error Recovery** - Custom ErrorNode insertion for fault tolerance
- **Control** - Full control over diagnostic messages
- **Incremental Parsing** - Future LSP support
