# SysML v2 Runtime REPL Demo

This demonstrates the integrated execution runtime in the REPL.

## Setup

Create a sample model file:

```sysml
// vehicle.sysml
part def Wheel {
    attribute diameter = 0.5;
    attribute pressure = 32.0;
}

part def Engine {
    attribute power = 300.0;
    attribute cylinders = 6;
}

part def Vehicle {
    attribute wheels = 4;
    part engine: Engine;
}
```

## REPL Session

```bash
$ go run ./cmd/sysml

SysML v2 REPL — %help for commands, Ctrl-D to exit

> %load vehicle.sysml
part def Wheel { ... }
part def Engine { ... }
part def Vehicle { ... }

> %instantiate Vehicle
✓ Created instance of Vehicle
  ID: 1
  Use %slots Vehicle to inspect

> %slots Vehicle
Instance: Vehicle (ID: 1)
Slots:
  wheels = 4
  engine = Instance(ID: 2)

> %instantiate Wheel
✓ Created instance of Wheel
  ID: 3
  Use %slots Wheel to inspect

> %slots Wheel
Instance: Wheel (ID: 3)
Slots:
  diameter = 0.50
  pressure = 32.00

> %instances
Instances:
  Vehicle (ID: 1)
  Wheel (ID: 3)

> %help
%help               show this help
%list               list current session declarations
%clear              reset the session
%load <file>        read a file and submit its contents

Runtime commands:
%instantiate <name> create an instance of a part def
%eval <expr>        evaluate an expression
%slots <name>       show instance slots and values
%instances          list all instantiated objects
```

## Features Demonstrated

- ✅ Load SysML files into session
- ✅ Instantiate part definitions  
- ✅ Inspect instance slots and default values
- ✅ Nested instances (Vehicle.engine → Instance(ID: 2))
- ✅ Lazy materialization (nested parts created automatically)
- ✅ Instance registry tracking

## What's Working

- Expression evaluation (arithmetic, reals, comparison)
- Part instantiation with Tier 1-3 runtime
- Slot access with default values
- Composite structure (nested parts)
- Instance ID tracking

## Current Limitations

- `%eval` only supports literal expressions (not feature references yet)
- No calc invocation from REPL yet
- No constraint evaluation commands yet
- Behavioral execution (actions/states) not implemented

## Next Steps

- Wire feature reference resolution into `%eval`
- Add `%run <calc_name>(args)` for calc execution
- Add `%check <constraint>` for constraint evaluation
- Integrate with LSP for hover/completion with runtime values
