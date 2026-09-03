# 5. Expressions, calculations, constraints and requirements

This chapter describes what the runtime evaluates and how each kind of check reports its result.
A constraint declared on a definition is checked against the object that carries it, so
instantiate the definition first if you want a verdict about a concrete value rather than a
default. When several objects the session holds carry it — two `%instantiate`s of one name leave
the first object reachable as `#<id>`, and a multi-valued part holds one carrier per element — the
check names them (`car::wheels[1]`, `car::wheels[2]`, `#1::wheels[1]`, …) and asks you to pick one,
with `%eval in car.wheels[2] : ...` or `%eval in #1 : ...`.

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

The nested engine is an object of its own. Reach it by a path from the object that holds it, or
by the id it was given, and read a value from it the same way:
```sysml
sysml> %features Car.engine
Instance: Car::engine (ID: 2)
Features:
  power = 250.0

sysml> %eval in #2 : power * 2
✓ power * 2 (on #2 ID: 2)
  = 500.0
```

An element of a multi-valued part is picked by index counted from 1, `System.wheels[3]`; see
[addressing an object](04-repl.md#addressing-an-object).

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

For more examples, see
[examples/repl-behavioral-demo.sysml](../../examples/repl-behavioral-demo.sysml).

---

Next: [6. Behavior: actions and state machines](06-behavior.md).
