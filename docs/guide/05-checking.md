# 5. Expressions, calculations, constraints and requirements

What the runtime evaluates, and how each kind of check reports. A constraint declared on a
definition is checked against the object carrying it, so instantiate first when you want a
verdict about a value rather than about a default.

## Expressions

**Literals:**
```sysml
attribute x = 42;              // Integer
attribute y = 3.14;            // Real
attribute flag = true;         // Boolean
attribute name = "System";     // String
```

**Operators:**
```sysml
attribute sum = 10 + 5;        // Arithmetic
attribute product = 3 * 7;
attribute comparison = x > 10; // Relational
attribute logic = flag and true; // Boolean
```

**Feature References:**
```sysml
part def Wheel {
    attribute diameter = 16.0;
}

part Vehicle {
    part wheel : Wheel;
    attribute wheelDiameter = wheel.diameter; // Feature chain
}
```

## Composite structures

```sysml
private import ScalarValues::*;

part def Engine {
    attribute power : Real = 200.0;
}

part def Car {
    part engine : Engine {
        :>> power = 250.0;  // Redefine nested feature
    }
}
```

Instantiate and inspect:
```sysml
sysml> %instantiate Car
✓ Created instance of Car
  ID: 1
  Use %features Car to inspect

sysml> %features Car
Instance: Car (ID: 1)
Features:
  engine = Instance(ID: 2)
    power = 250.0
```

## Multiplicity

```sysml
part System {
    part sensors : Sensor[0..10];  // 0 to 10 sensors
    part wheels : Wheel[4];         // Exactly 4 wheels
}
```

## Calculations, constraints and requirements

**Calculations:**
```sysml
sysml> calc distance {
  ...>     in x;
  ...>     in y;
  ...>     x * x + y * y
  ...> }
✓ calc distance

sysml> %calc distance 3 4
✓ distance(3, 4)
  = 25
```

**Constraints:**
```sysml
sysml> constraint ValidSpeed {
  ...>     65 > 0 and 65 <= 120
  ...> }
✓ constraint ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Requirements:**
```sysml
sysml> requirement SafetyReq {
  ...>     assume constraint { 65 > 0 }
  ...>     require constraint { 100 > 50 }
  ...> }
✓ requirement SafetyReq

sysml> %requirement SafetyReq
✓ Requirement SafetyReq satisfied
```

**See [examples/repl-behavioral-demo.sysml](../../examples/repl-behavioral-demo.sysml) for comprehensive examples.**

---

Next: [6. Behavior: actions and state machines](06-behavior.md).
