# 2. Your first model

This chapter declares a part, gives it values, instantiates it and inspects the result, first
at the interactive prompt and then from a file. Every construct shown is the same notation a
`.sysml` file contains; the REPL reports the result more quickly.

## At the prompt

Launch the interactive REPL:

```bash
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

### Define a Simple Part

Library types such as `Real` are not in scope automatically. Import them exactly as a `.sysml`
file would:

```sysml
sysml> private import ScalarValues::*;
✓ import ScalarValues::*

sysml> part def Wheel {
  ...>     attribute diameter : Real = 16.0;
  ...>     attribute width : Real = 7.5;
  ...> }
✓ part def Wheel
```

Each accepted declaration is echoed back as `✓ <kind> <name>`. An opening brace begins a
continuation (`...>`) that runs to the matching closing brace. A **blank line ends the
submission**, so a declaration being typed must not contain one.

Re-typing a namespace **adds to** the one already in the session: `package P { part def B; }`
submitted after `package P { part def A; }` leaves both declared. An empty body
(`package P { }`) clears the namespace. Anything a submission drops is reported on a `note:`
line: the members it no longer declares, the instances it invalidated (whose IDs restart with
the new model), and any `%action` or `%state` debugging session it ended. A debugging session
over a declaration the submission did not touch continues to run.

### Define a Vehicle

```sysml
sysml> part def Vehicle {
  ...>     attribute mass : Real = 1500.0;
  ...>     part wheels : Wheel[4];
  ...> }
✓ part def Vehicle
```

### Instantiate and Inspect

```sysml
sysml> %instantiate Vehicle
✓ Created instance of Vehicle
  ID: 1
  Use %features Vehicle to inspect

sysml> %features Vehicle
Instance: Vehicle (ID: 1)
Features:
  mass = 1500.0
  wheels = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4), Instance(ID: 5)]
    diameter = 16.0
    width = 7.5
    diameter = 16.0
    width = 7.5
    diameter = 16.0
    width = 7.5
    diameter = 16.0
    width = 7.5

sysml> %instances
Instances:
  Vehicle (ID: 1)
```

### Evaluate Expressions

```sysml
sysml> attribute wheelCount = 4;
✓ attribute wheelCount

sysml> attribute totalDiameter = wheelCount * 16.0;
✓ attribute totalDiameter

sysml> %eval totalDiameter
✓ totalDiameter
  = 64.0
```

---

## From a file

Create a file `my_model.sysml`:

```sysml
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    part def System {
        part sensors : Sensor[3];
    }
}
```

Load the file in the REPL. `%load` submits the file's contents as though they had been typed,
so it reports the same `✓` lines. `%list` echoes everything the session currently holds:

```bash
$ sysml
sysml> %load my_model.sysml
✓ package MyModel

sysml> %list
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    part def System {
        part sensors : Sensor[3];
    }
}

sysml> %instantiate MyModel::System
✓ Created instance of MyModel::System
  ID: 1
  Use %features MyModel::System to inspect

sysml> %features MyModel::System
Instance: MyModel::System (ID: 1)
Features:
  sensors = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4)]
    reading = 0.0
    threshold = 100.0
    reading = 0.0
    threshold = 100.0
    reading = 0.0
    threshold = 100.0
```

A composite feature lists the features of each of its objects under it, in order.

---

Next: [3. From the command line](03-command-line.md), which performs these same checks without
a prompt. The prompt itself is covered in [4. The REPL](04-repl.md).
