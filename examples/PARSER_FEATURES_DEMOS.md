# Parser Features Demos

Demonstrates new SysML v2 / KerML parser features added in recent development sessions.

## Overview

These demos showcase parser improvements that enable parsing 81%+ of the official SysML v2 standard library. Each demo file focuses on a specific category of features and includes working examples extracted from real stdlib usage patterns.

## Demo Files

### 1. Relationships Demo (`parser_features_demo_relationships.kerml`)

**New relationship keywords:**
- `inverse of` - Bidirectional relationship inverses
- `unions` - Union relationships that combine multiple features
- `chains` - Feature chaining for multi-level navigation

**Example:**
```kerml
feature parent: Node[1];
feature child: Node[0..*] inverse of parent;

feature dataPath chains source.target;
```

### 2. Modifiers Demo (`parser_features_demo_modifiers.kerml`)

**Visibility modifiers:**
- `public` / `protected` / `private` - Access control
- `readonly` - Immutable features
- `constant` - Compile-time constants

**Post-multiplicity modifiers:**
- `ordered` - Maintains insertion order
- `nonunique` - Allows duplicates

**Example:**
```kerml
protected ref thisParticipant :>> self;
readonly feature version: String[1];
abstract constant ref maxConnections: Integer[1];
port connections: Connection[1..*] nonunique;
```

### 3. Binding Demo (`parser_features_demo_binding.kerml`)

**Binding usage patterns:**
- Basic: `binding [mult] name = value`
- With source: `binding [mult] name[mult2] source = value`
- With 'of' keyword: `binding name of [mult] target = value`
- Feature chains: `binding data = source.field`

**Example:**
```kerml
// Complex binding pattern from stdlib
binding [1] bind [0..*] base.edges = [0..*] boundaryEdges;

// Feature chain binding
binding startTime = startEvent.timestamp;
```

### 4. Connectors & Succession Demo (`parser_features_demo_connectors.kerml`)

**Connection patterns:**
- Named connections with typing
- `connect` keyword: `connection :Type connect [1] a to [1] b`

**Succession patterns:**
- Named: `succession name first [1] x then [1] y`
- Anonymous: `succession [1] x then [1] y`
- With `first` keyword

**Example:**
```kerml
// Connection with connect keyword
connection :MatesWith connect [1] partA to [1] partB;

// Named succession with first/then
succession eventOrder first [1] startEvent then [1] endEvent;

// Anonymous succession
succession [1] initStep then [1] processStep;
```

### 5. Default Values Demo (`parser_features_demo_defaults.kerml`)

**Default keyword:**
- Alternative to `=` for value assignment
- Clearer intent for default values
- Works with multiplicity

**Example:**
```kerml
// Traditional assignment
feature timeout: Integer[1] = 30;

// Default keyword (clearer intent)
feature retryCount: Integer[1] default 3;

// Configuration with defaults
feature host: String[1] default "localhost";
feature port: Integer[1] default 8080;
```

## Running the Demos

Test all demos parse correctly:

```bash
go test ./examples -run '^TestParserFeaturesDemos$' -v
```

All demos should parse cleanly with no errors.

## Development Context

These features were implemented across multiple development sessions:

- **Session 2 (Tasks 47-52):** Inverse of, unions, protected/readonly, nonunique, default keyword, binding syntax
- **Session 3 (Tasks 53-57):** Binding name[mult] pattern, constant modifier, succession first/then, chains relationship, connection connect keyword

**Parser Coverage:** 81.1% of official SysML v2 standard library (77 of 95 files parse cleanly)

## Real-World Usage

These patterns appear throughout the official standard library:

- **Base.kerml:** chains relationship, visibility modifiers
- **Occurrences.kerml:** inverse of, succession patterns
- **Interfaces.sysml:** protected references, nonunique ports
- **ShapeItems.sysml:** complex binding patterns, connection with connect
- **CausationConnections.sysml:** named succession with first/then

## Notes

- The parser prioritizes correctness over permissiveness
- Some edge cases (connector end + feature hybrids, constraint statements) remain unsupported due to architectural constraints
- All syntax matches official SysML v2 specification patterns
