# 2. Your first model

Declare a part, give it values, instantiate it and look inside — first by typing at the prompt,
then from a file. Everything here is notation you would write in a `.sysml` file; the REPL is
just a faster way to see it answer.

## At the prompt

Launch the interactive REPL:

```bash
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

### Define a Simple Part

Library types such as `Real` are not in scope automatically — import them, exactly as a
`.sysml` file would:

```sysml
sysml> private import ScalarValues::*;
✓ import ScalarValues::*

sysml> part def Wheel {
  ...>     attribute diameter : Real = 16.0;
  ...>     attribute width : Real = 7.5;
  ...> }
✓ part def Wheel
```

Each accepted declaration is echoed back as `✓ <kind> <name>`. A brace opens a
continuation (`...>`) that runs to the matching one — but a **blank line ends the
submission**, so leave none inside a declaration you are typing.

Re-typing a namespace **adds to** the one already in the session, so
`package P { part def B; }` after `package P { part def A; }` leaves both
declared; an empty body (`package P { }`) is how you clear one. Anything a
submission does drop is reported as a `note:` line — the members it no longer
declares, the instances it invalidated (their IDs restart with the new model),
and any `%action`/`%state` debugging session it ended. A debugging session over a
declaration the submission did not touch keeps running.

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
  mass = 1500.00
  wheels = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4), Instance(ID: 5)]
    diameter = 16.00
    width = 7.50
    diameter = 16.00
    width = 7.50
    diameter = 16.00
    width = 7.50
    diameter = 16.00
    width = 7.50

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
  = 64.00
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

Load it in the REPL. `%load` submits the file's contents as if you had typed them, so it
reports the same `✓` lines; `%list` echoes everything the session currently holds:

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
    reading = 0.00
    threshold = 100.00
    reading = 0.00
    threshold = 100.00
    reading = 0.00
    threshold = 100.00
```

A composite feature lists the features of each of its objects under it, in order.

---

Next: [3. From the command line](03-command-line.md), which runs these same checks without a
prompt. The prompt itself is covered in [4. The REPL](04-repl.md).
