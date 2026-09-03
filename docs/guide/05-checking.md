# 5. Expressions, calculations, constraints and requirements

This chapter describes what the runtime evaluates and how each kind of check reports its result.
A constraint declared on a definition is checked against the object that carries it, so
instantiate the definition first if you want a verdict about a concrete value rather than a
default.

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
    attribute power : Real default = 200.0;
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

**Library functions:**

The KerML function libraries (`RealFunctions::sqrt`, `SequenceFunctions::size`,
`NumericalFunctions::sum`, …) are ordinary library packages, and an expression reaches one of
their functions by the same rule the checker applies to every name: the qualified name resolves
anywhere, and the bare name resolves only where the model imports the package that declares it.
Evaluation follows the checker, so a call the checker reports as an unresolved reference does not
evaluate either; the error names the qualified spellings the call may have meant, and importing
one of those packages makes it resolve.

```sysml
sysml> package Demo {
  ...>     attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
  ...>     attribute wheelCount = wheels->size();
  ...> }
3:36: error: unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?
    attribute wheelCount = wheels->size();
                                   ^~~~

sysml> %eval Demo::wheelCount
error: evaluation failed: unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?

sysml> %eval SequenceFunctions::size(Demo::wheels)
✓ SequenceFunctions::size(Demo::wheels)
  = 4

sysml> package Demo {
  ...>     private import SequenceFunctions::*;
  ...>     attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
  ...>     attribute wheelCount = wheels->size();
  ...> }
✓ package Demo
note: added to the existing package Demo, replacing attribute wheels, attribute wheelCount

sysml> %eval Demo::wheelCount
✓ Demo::wheelCount
  = 4
```

A `calc` the model declares under a library function's name is what a call resolves to, even where
the library is also imported. `%builtins` lists every function the build evaluates, each with the
package an `import` must name for its bare name to resolve.

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
