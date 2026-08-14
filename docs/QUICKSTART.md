# Quick Start Guide

Get up and running with Systemica in 5 minutes.

## Installation

### Option 1: Download Pre-built Binaries (Recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/Open-MBEE/Systemica/releases):

**Linux (x64):**
```bash
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/sysml-linux-amd64.tar.gz
tar xzf sysml-linux-amd64.tar.gz
sudo mv sysml-linux-amd64 /usr/local/bin/sysml
chmod +x /usr/local/bin/sysml
```

**macOS (Intel or Apple Silicon) — Homebrew is the recommended path:**
```bash
brew install Open-MBEE/tap/systemica
```
This avoids the Gatekeeper prompt described in [macOS: Gatekeeper](#macos-gatekeeper).

Use that fully-qualified name rather than tapping first. Homebrew 6 requires third-party taps
to be trusted before their code is loaded; installing by fully-qualified name trusts only this
formula, whereas the two-step form needs a trust step in between:
```bash
brew tap Open-MBEE/tap
brew trust --formula Open-MBEE/tap/systemica   # or: brew trust Open-MBEE/tap, for the whole tap
brew install systemica
```

**macOS, direct download (fallback):** use `curl`, not a browser.
```bash
# Apple Silicon; use systemica-darwin-amd64.tar.gz on Intel
curl -fL -o systemica.tar.gz https://github.com/Open-MBEE/Systemica/releases/latest/download/systemica-darwin-arm64.tar.gz
tar xzf systemica.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
```

**Windows:**
Download `systemica-windows-amd64.zip` from [releases](https://github.com/Open-MBEE/Systemica/releases/latest), extract, and add to PATH. Windows SmartScreen may warn that the publisher is unrecognized; the binaries are not Authenticode-signed.

**Available binaries:**
- `sysml` — Interactive REPL
- `sysml-lsp` — Language Server Protocol server

`sysml-grpc` — the service the Python bindings talk to — is published as a raw
`sysml-grpc-<os>-<arch>` file with a `.sha256` sidecar rather than in an archive, because
`pysysml` downloads and verifies it itself (see [python/README.md](../python/README.md)).
`make build-grpc` builds it from source.

**Archive layout:** `systemica-<os>-<arch>.tar.gz` bundles contain both binaries under their
plain names (`sysml`, `sysml-lsp`); the older single-binary `sysml-<os>-<arch>.tar.gz` and
`sysml-lsp-<os>-<arch>.tar.gz` archives are still published. The bundles and
`SHA256SUMS.txt` are published from v0.0.4 onward; for earlier releases use the
single-binary archives. The `sysml-grpc` binaries and their sidecars are published from the
next release onward, and `SHA256SUMS.txt` covers every archive and every published
`sysml-grpc` binary:

```bash
curl -fLO https://github.com/Open-MBEE/Systemica/releases/latest/download/SHA256SUMS.txt
shasum -a 256 -c SHA256SUMS.txt --ignore-missing   # macOS; use sha256sum -c on Linux
```

### macOS: Gatekeeper

When macOS refuses to run a downloaded binary with **"cannot be opened because the developer
cannot be verified"**, the cause is the `com.apple.quarantine` extended attribute that
browsers attach to downloads, combined with the fact that these binaries are not signed with
an Apple Developer ID or notarized. It is not a broken binary.

Ways to avoid it, best first:

1. **Install with Homebrew** (`brew install Open-MBEE/tap/systemica`). Homebrew
   downloads with `curl` and does not quarantine formula binaries. This is the recommended
   path, and the accepted stopgap until the releases are signed and notarized.
2. **Download with `curl` or `wget`** (as shown above). They do not set the quarantine
   attribute, so no prompt appears.
3. **Install with a Go toolchain** — built locally, never quarantined:
   ```bash
   go install github.com/Open-MBEE/Systemica/cmd/sysml@latest
   go install github.com/Open-MBEE/Systemica/cmd/sysml-lsp@latest
   ```
4. **Clear the attribute** if you already downloaded the archive in a browser. Verify the
   checksum first — you are turning off a security check, so make sure you have the file we
   published:
   ```bash
   shasum -a 256 systemica-darwin-arm64.tar.gz   # compare against SHA256SUMS.txt
   xattr -d com.apple.quarantine /usr/local/bin/sysml /usr/local/bin/sysml-lsp
   ```
   `xattr -d: No such xattr` simply means the file was not quarantined. Use
   `xattr -c <file>` to clear all attributes, or `xattr -dr com.apple.quarantine <dir>` for a
   directory.

See [MACOS_DISTRIBUTION.md](MACOS_DISTRIBUTION.md) for the root-cause analysis and for what
signing + notarizing the releases would require.

### Option 2: Build from Source

**Prerequisites:**
- Go 1.25 or later
- Git
- Make (optional but recommended)

**Build:**
```bash
git clone https://github.com/Open-MBEE/Systemica.git
cd Systemica
make build       # builds bin/sysml, bin/sysml-lsp, and bin/sysml-grpc
# OR
go build -o sysml ./cmd/sysml
go build -o sysml-lsp ./cmd/sysml-lsp
```

**Install (optional):**
```bash
make install     # installs to $GOPATH/bin
# OR
sudo mv bin/sysml bin/sysml-lsp bin/sysml-grpc /usr/local/bin/
```

---

## Your First SysML v2 Model

### Using the REPL

Launch the interactive REPL:

```bash
$ sysml
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

#### 1. Define a Simple Part

Library types such as `Real` are not in scope automatically — import them, exactly as a
`.sysml` file would:

```sysml
sysml> import ScalarValues::*;
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

#### 2. Define a Vehicle

```sysml
sysml> part def Vehicle {
  ...>     attribute mass : Real = 1500.0;
  ...>     part wheels : Wheel[4];
  ...> }
✓ part def Vehicle
```

#### 3. Instantiate and Inspect

```sysml
sysml> %instantiate Vehicle
✓ Created instance of Vehicle
  ID: 1
  Use %slots Vehicle to inspect

sysml> %slots Vehicle
Instance: Vehicle (ID: 1)
Slots:
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

#### 4. Evaluate Expressions

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

### Working with Files

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
  Use %slots MyModel::System to inspect

sysml> %slots MyModel::System
Instance: MyModel::System (ID: 1)
Slots:
  sensors = [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4)]
    reading = 0.00
    threshold = 100.00
    reading = 0.00
    threshold = 100.00
    reading = 0.00
    threshold = 100.00
```

A composite slot lists the features of each of its objects under it, in order.

---

## Checking a Model from the Command Line

The same checks the REPL runs are available as flags, so a model can be checked
from a script or a build step without a prompt. Take `checks.sysml`:

```sysml
package MyModel {
    part def Sensor {
        attribute reading = 0.0;
        attribute threshold = 100.0;
    }

    requirement def ReadingRequirement {
        subject sensor : Sensor;
        require sensor.reading <= sensor.threshold;
    }
    requirement healthy : ReadingRequirement;

    part hot : Sensor {
        attribute :>> reading = 140.0;
        constraint inRange { reading <= threshold }
    }
    part cold : Sensor {
        attribute :>> reading = 20.0;
        constraint inRange { reading <= threshold }
    }

    part checks {
        assert satisfy healthy by cold;
        assert satisfy healthy by hot;
    }

    calc def Margin {
        in reading;
        in threshold;
        return threshold - reading;
    }

    action calibrate {
        attribute offset = 0.0;
        first start;
        action adjust {
            assign offset := offset + 1.5;
        }
        done end;
        then start adjust;
        then adjust end;
    }

    state Monitor {
        initial off;
        state warming {
            accept after 10 then running;
        }
        state running;
        off then warming;
    }
}
```

Each flag may be repeated, and `-instantiate` runs first — whatever order the
flags are written in — so the verdicts after it are about that object:

```bash
$ sysml -instantiate MyModel::cold -constraint MyModel::cold::inRange checks.sysml
✓ package MyModel
✓ Created instance of MyModel::cold
  ID: 1
  Use %slots MyModel::cold to inspect
✓ Constraint MyModel::cold::inRange passed (on MyModel::cold ID: 1)

$ sysml -satisfy checks.sysml
✓ package MyModel
✓ satisfy healthy by cold holds (on MyModel::cold ID: 1)
✗ satisfy healthy by hot fails (on MyModel::hot ID: 2)
  Required condition evaluated to false: sensor.reading <= sensor.threshold
```

| Flag | Checks |
|------|--------|
| `-validate` | Nothing about the model's conditions: only that it analyses cleanly |
| `-constraint <name>` | One constraint, as `%constraint` does |
| `-requirement <name>` | One requirement, as `%requirement` does |
| `-satisfy` | Every satisfaction assertion the model states |
| `-satisfy=<name>` | Only the assertions the named element states |
| `-instantiate <name>` | Creates an object first, so the verdicts are about it |
| `-calc "<name>(<args>)"` | Invokes a calculation and reports what it computed |
| `-action "<name> [object]"` | Runs an action to completion and reports its outputs |
| `-state "<name> [object]"` | Runs a state machine and reports where it settled |
| `-advance <time>` | Simulated time units each `-state` machine is run for |
| `-json` | Reports the checks as one JSON document rather than as lines |

The exit status is what a build step gates on:

| Status | Meaning |
|--------|---------|
| `0` | Every check held |
| `1` | The model answered false for at least one check |
| `2` | A check was never decided: an unknown name, a subject with no object to evaluate against, a model that would not load, or a misused flag |

Status 2 is kept apart from 1 because an undecided check is not evidence against
the model — treat it as a broken check, not a failing one. A condition that
evaluated to false is the model's own answer; a name that does not resolve, or a
condition over a feature with no value, is not:

```bash
$ sysml -constraint MyModel::hot::inRange -instantiate MyModel::hot checks.sysml; echo "exit=$?"
✓ package MyModel
✓ Created instance of MyModel::hot
  ID: 1
  Use %slots MyModel::hot to inspect
✗ Constraint MyModel::hot::inRange failed (on MyModel::hot ID: 1)
  Assertion evaluated to false: reading <= threshold
exit=1

$ sysml -constraint MyModel::nosuch checks.sysml; echo "exit=$?"
✓ package MyModel
error: symbol "MyModel::nosuch" not found
exit=2

$ sysml -requirement MyModel::healthy checks.sysml; echo "exit=$?"
✓ package MyModel
✗ Requirement MyModel::healthy failed
  Error: requirement healthy: require condition evaluation failed: no value for feature sensor
exit=2
```

That last case is the shape of a requirement with a `subject`: nothing binds the
subject, so it has no object to be about. Write the intended pair into the model
as `assert satisfy healthy by hot;` and check it with `-satisfy`, which creates
the subject itself — `-requirement` decides a requirement whose conditions stand
on their own, or one carried by a part an `-instantiate` created.

A verdict is written to stdout and an undecided check to stderr, so
`sysml -satisfy checks.sysml > verdicts.txt` keeps the results and leaves what
went wrong on the terminal.

### Analysis as a Gate

`-validate` asks nothing of the model's conditions: it loads it and reports what
analysis found, which is the lint step to run before any verdict is trusted.

```bash
$ sysml -validate checks.sysml; echo "exit=$?"
✓ package MyModel
✓ checks.sysml: no errors
exit=0

$ sysml -validate bad.sysml; echo "exit=$?"   # a file with a syntax error in it
2:45: error: expected an expression
    part def Battery { attribute capacity = ; }
                                            ^
sysml: bad.sysml did not analyse cleanly; no check was made
exit=2
```

Every check mode is gated the same way, so a model with an error never reports a
verdict about itself — a condition read out of a model the tool could not fully
read would be an answer about a different model than the one you wrote.

### Running Behavior

The debuggers have non-interactive forms that run to completion and report the
values they produced. `-calc` takes the invocation, and `-action` and `-state`
take the behavior's name optionally followed by the object performing it:

```bash
$ sysml -calc "MyModel::Margin(20.0, 100.0)" checks.sysml
✓ package MyModel
✓ MyModel::Margin(20.0, 100.0)
  = 80.00

$ sysml -action MyModel::calibrate checks.sysml
✓ package MyModel
✓ Started action executor for "MyModel::calibrate"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  Results:
    offset = 1.50

$ sysml -state MyModel::Monitor -advance 15 checks.sysml
✓ package MyModel
✓ Started state machine executor for "MyModel::Monitor"
  Current state: off
  Time: 0.00
  Events: 1
✓ Advanced to 15.00 (2 event(s) processed)
  Current state: running
  Last event at: 10.00
  Remaining events: 0
```

A state machine only takes its initial transition unless `-advance` says how much
simulated time to run it for. An action that stopped short of completing — a
deadlock, or the step budget reached — is reported as a check that was never
decided, i.e. status 2, since it produced no outputs to judge.

### Machine-Readable Results

`-json` reports the same run as one document, so a build step reads structure
rather than parsing `✓`/`✗`. It reports the checks, not the model: use `-convert`
to serialize the model itself.

```bash
$ sysml -satisfy -json checks.sysml; echo "exit=$?"
{
  "status": "fails",
  "exit": 1,
  "checks": [
    {
      "subject": "satisfy healthy by cold",
      "status": "holds",
      "values": null,
      "lines": [
        "✓ satisfy healthy by cold holds (on MyModel::cold ID: 1)"
      ]
    },
    {
      "subject": "satisfy healthy by hot",
      "status": "fails",
      "values": null,
      "lines": [
        "✗ satisfy healthy by hot fails (on MyModel::hot ID: 2)",
        "  Required condition evaluated to false: sensor.reading \u003c= sensor.threshold"
      ]
    }
  ],
  "diagnostics": null,
  "output": [
    "✓ package MyModel"
  ],
  "errors": null
}
exit=1
```

`status` is the worst verdict reached and `exit` the status the process exits
with. A calculation's or a machine's values are reported as `values` entries, the
findings of analysis as `diagnostics`, and whatever stopped a check from being
made as `errors` — so the whole document goes to stdout and nothing needs to be
read off stderr.

---

## Saving and Converting

`%save` writes the session out. The format follows the extension — `.sysml` for
notation, `.ttl` for RDF Turtle:

```bash
sysml> %save my_model.sysml
saved 181 bytes of sysml to my_model.sysml (replaced the existing file)

sysml> %save my_model.ttl
saved 1872 bytes of ttl to my_model.ttl
```

A leading `~` is expanded, an existing file is replaced and the replacement is
stated, and the write is atomic — an interrupted save leaves the previous file
intact. A file that already exists keeps its permissions, and a symlink is
written through rather than replaced.

A session that does not fully parse is still saved as notation: that file is
your own text re-indented, so the syntax errors are reported as warnings and the
work is never trapped in the REPL.

```bash
sysml> %save my_model.sysml
warning: <session>: 1 syntax error(s):
  4:6: expected a namespace member
warning: the file is saved as typed; fix these and save again
saved 181 bytes of sysml to my_model.sysml (replaced the existing file)
```

`.ttl` keeps the refusal, because a graph built from a tree the parser only
partly recovered would be quietly missing declarations. So does
`sysml -convert`, where the source already exists on disk.

The same conversion is available without starting the REPL:

```bash
$ sysml -convert my_model.sysml -o my_model.ttl   # notation to RDF
$ sysml -convert my_model.ttl -o back.sysml       # RDF to notation
$ sysml -convert my_model.sysml                   # to stdout, in the other format
```

See [RDF_INTEROP.md](RDF_INTEROP.md) for the vocabulary, the round-trip
guarantees, and what the mapping does not cover.

---

## REPL Commands

| Command | Description |
|---------|-------------|
| `%help` | Show help message |
| `%list` | List all declarations in current session |
| `%clear` | Clear session (reset all declarations) |
| `%load <file>` | Load .sysml file into session |
| `%save <file>` | Write the session model to a file: `.sysml` notation (comments preserved) or `.ttl` RDF |
| `%verbosity [level]` | Show or set output level: `quiet` (errors only), `normal`, `debug` (every diagnostic over the whole buffer) |
| `%trace [on\|off]` | Show or set execution tracing: each evaluation, calc invocation, action step and state transition |
| `%budget` | Show the five bounds one run may spend, each with the variable that raises it |
| **Instantiation & Inspection** | |
| `%instantiate <name>` | Create instance from part definition |
| `%slots <name>` | Show instance slots and values |
| `%instances` | List all created instances |
| `%eval <expr>` | Evaluate expression |
| **Behavioral Execution** | |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%constraint <name>` | Evaluate constraint (assert/assume) |
| `%requirement <name>` | Evaluate requirement (subject/assume/require/actor) |
| `%satisfy [name]` | Evaluate satisfaction assertions of the model, or of one element |
| **Control** | |
| `Ctrl-D` | Exit REPL |

---

## Runtime Features

### 1. Expressions & Calculations

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
part Wheel {
    attribute diameter = 16.0;
}

part Vehicle {
    part wheel : Wheel;
    attribute wheelDiameter = wheel.diameter; // Feature chain
}
```

### 2. Composite Structures

```sysml
import ScalarValues::*;

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
  Use %slots Car to inspect

sysml> %slots Car
Instance: Car (ID: 1)
Slots:
  engine = Instance(ID: 2)
    power = 250.00
```

### 3. Multiplicity

```sysml
part System {
    part sensors : Sensor[0..10];  // 0 to 10 sensors
    part wheels : Wheel[4];         // Exactly 4 wheels
}
```

### 4. Behavioral Execution

**Calculations:**
```sysml
sysml> calc distance {
  ...>     in x;
  ...>     in y;
  ...>     return (x * x + y * y);
  ...> }
✓ calc distance

sysml> %calc distance 3 4
✓ distance(3, 4)
  = 25
```

**Constraints:**
```sysml
sysml> constraint ValidSpeed {
  ...>     assert 65 > 0;
  ...>     assert 65 <= 120;
  ...> }
✓ constraint ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Requirements:**
```sysml
sysml> requirement SafetyReq {
  ...>     assume 65 > 0;
  ...>     require 100 > 50;
  ...> }
✓ requirement SafetyReq

sysml> %requirement SafetyReq
✓ Requirement SafetyReq satisfied
```

**See [examples/repl-behavioral-demo.sysml](../examples/repl-behavioral-demo.sysml) for comprehensive examples.**

### 5. Action & State Machine Debugging

**Action execution (step-by-step):**
```sysml
sysml> action SimpleWorkflow {
  ...>     attribute result = 0;
  ...>     first start;
  ...>     action compute { assign result := 42; }
  ...>     done end;
  ...>     then start compute;
  ...>     then compute end;
  ...> }
✓ action SimpleWorkflow

sysml> %action SimpleWorkflow
✓ Started action executor for "SimpleWorkflow"
  State: Running
  Tokens: 1

Use %step to advance, %tokens to inspect, %continue to run to completion

sysml> %step
✓ Step complete
  State: Running
  Tokens: 1

sysml> %tokens
Active tokens (1):
  Token 1 @ compute
    result = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    result = 42
```

**State machine execution:**
```sysml
sysml> state TrafficLight {
  ...>     initial start;
  ...>     state green { accept after 25 then yellow; }
  ...>     state yellow { accept after 5 then red; }
  ...>     state red { accept after 30 then off; }
  ...>     final off;
  ...>     start then green;
  ...> }
✓ state TrafficLight

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: start
  Time: 0.00
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

sysml> %advance 25
✓ Advanced to 25.00 (2 event(s) processed)
  Current state: yellow
  Last event at: 25.00
  Remaining events: 1

sysml> %current
Current state: yellow
Time: 25.00
Last event at: 25.00
Execution state: Running

sysml> %advance 5
✓ Advanced to 30.00 (1 event(s) processed)
  Current state: red
  Last event at: 30.00
  Remaining events: 1

sysml> %advance 30
✓ Advanced to 60.00 (1 event(s) processed)
  Current state: off
  Last event at: 60.00
  Remaining events: 0

✓ State machine completed (final state reached)
```

**Action debugging commands:**
- `%action <name> [<object>]` — Start action debugging session, optionally performed by an instantiated object
- `%step` — Advance all tokens one step
- `%continue` — Run to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with data
- `%break <node>` — Set breakpoint on a named node; `%continue` stops when a token reaches it
- `%stop` — Stop debugging

**State machine debugging commands:**
- `%state <name> [<object>]` — Start state machine debugging; naming an instantiated object performs the machine for it, so what it sends routes over that object's connections
- `%events` — Show event queue
- `%current` — Show current state, stack, data
- `%advance <time>` — Advance simulation time by `<time>` units, processing every event due
- `%stop` — Stop debugging

**See [examples/action-executor-demo.sysml](../examples/action-executor-demo.sysml) and [examples/state-machine-demo.sysml](../examples/state-machine-demo.sysml) for complete workflows.**

---

## Language Server (IDE Support)

### VS Code Setup

1. Build the LSP server:
```bash
go build -o sysml-lsp ./cmd/sysml-lsp
```

2. Install a generic LSP extension (e.g., "Generic LSP Client")

3. Configure in `.vscode/settings.json`:
```json
{
  "genericLanguageServer.servers": [
    {
      "name": "SysML v2",
      "command": "/absolute/path/to/sysml-lsp",
      "args": [],
      "filetypes": ["sysml", "kerml"]
    }
  ]
}
```

4. Associate file extensions in `.vscode/settings.json`:
```json
{
  "files.associations": {
    "*.sysml": "sysml",
    "*.kerml": "kerml"
  }
}
```

**LSP features (all implemented):**
- ✅ Document synchronization (incremental updates)
- ✅ Diagnostics (syntax + semantic errors, real-time)
- ✅ Hover (symbol info, type, multiplicity)
- ✅ Go-to-definition (cross-document navigation)
- ✅ Find references (workspace-wide search)
- ✅ Completion (trigger on `:`, `.`)
- ✅ Document symbols (outline view)
- ✅ Workspace symbols (global search)

**Test the server:**
```bash
# Check version
./sysml-lsp --version

# Test with example file
echo 'part Wheel { attribute diameter = 16.0; }' > test.sysml
# Open test.sysml in VS Code, hover over "Wheel" to see symbol info
```

---

## Environment Variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `SYSML_LIBRARY_PATH` | unset (use the bundled standard library) | Directory to load the SysML/KerML standard library from instead of the embedded copy |
| `SYSML_MAX_STEPS` | `10000000` | Evaluation step budget: the number of expression evaluations one run may spend before it is reported as a runaway |
| `SYSML_MAX_ACTION_STEPS` | `1000000` | Token-flow steps one action run may perform |
| `SYSML_MAX_EVENTS` | `1000000` | Events one state machine run may dispatch, and the events one `%advance` drains |
| `SYSML_MAX_DO_STEPS` | `5000000` | Do-activity actions one state machine run may perform, and the ones one `%advance` drains |
| `SYSML_MAX_ELEMENTS` | `1000000` | Collection elements one evaluation may hold — the bound on the memory a run holds rather than on the work it does |

Each budget is what turns a non-terminating run into a reported error instead of
a hang. They count incommensurable things — expression evaluations, action token
steps, dispatched events, do-activity actions, materialized collection elements —
so raising one says nothing about the others, and each has its own variable.

A budget bounds **one run** — one `%eval`, one `%instantiate`, one `%calc`, one
action, one state machine — not a whole session, so a long REPL session of small
operations never runs out. A run started inside another, an action invoked from
an expression say, shares the outer run's budget rather than getting a fresh
one, and so does a run stepped through with `%step`/`%advance`.

The step and event defaults are set by how long a runaway takes to report rather
than by memory — those steps allocate nothing that outlives them (peak RSS is ~34
MB whether a run spends ten thousand steps or fifty million), and the only thing
they make grow is a `%trace`, at 34–83 bytes an entry. At the measured ~13.6M
evaluation steps/s and ~1.9M events/s each default reports a runaway within about
a second, and a fully traced run at those four ceilings holds ~320 MB.

Collection elements are the exception, and `SYSML_MAX_ELEMENTS` is the budget
that reads as memory: a materialized element is a 104-byte value living as long as
the collection holding it, and `1..10000000` conjures one per step. Every way of
materializing a sequence is charged against it — a range, a sequence literal,
`->collect` and the other collection operations — so the default bounds the
elements held at once at ~104 MB, in the same band as the figures above:

```
error: evaluation failed: collection element limit exceeded
(1000000 elements; raise SYSML_MAX_ELEMENTS to allow more)
```

Because it bounds memory and not work, the count is what a statement's evaluation
holds: a loop building a ten-element collection a million times never approaches
it, while a single `1..2000000` exceeds it at once.

The evaluation step budget:

```
error: execution failed: eval assignment RHS: evaluation step limit exceeded
(10000000 steps; raise SYSML_MAX_STEPS to allow more)
```

A legitimately long run — a numeric integration in an action body, say — needs a
higher ceiling, so raise it for that run:

```bash
SYSML_MAX_STEPS=200000000 sysml descent.sysml
```

Unset or empty means the default. Anything that is not a positive integer is
reported at startup (and at gRPC service construction) rather than silently
ignored:

```bash
$ SYSML_MAX_STEPS=lots sysml model.sysml
sysml: SYSML_MAX_STEPS="lots" is not an integer: set it to a positive number of evaluation steps (default 10000000)
```

The other budgets behave identically, and their errors name the variable that
raises them:

```
execution exceeded max steps (1000000 steps; raise SYSML_MAX_ACTION_STEPS to allow more), possible infinite loop
state machine exceeded max events (1000000 events; raise SYSML_MAX_EVENTS to allow more), possible infinite loop
state machine exceeded max do activity steps (5000000 steps; raise SYSML_MAX_DO_STEPS to allow more), possible non-terminating do behavior
```

A long simulation therefore raises the state machine bounds rather than the
evaluation one:

```bash
SYSML_MAX_EVENTS=20000000 SYSML_MAX_DO_STEPS=100000000 sysml descent.sysml
```

---

## Examples

Check `examples/` directory:
- `repl-behavioral-demo.sysml` — REPL behavioral walkthrough
- `ACTION-EXECUTOR-DEMO.md` — Action executor walkthrough
- `CLI_USAGE.md` — CLI usage reference

---

## Next Steps

1. **Read the architecture:** [`docs/ARCHITECTURE.md`](ARCHITECTURE.md)
2. **Explore runtime tiers:** See ARCHITECTURE.md § Execution Runtime
3. **Check test fixtures:** `testdata/*.sysml` for language examples
4. **Run tests:** `go test ./...` to see the system in action

---

## Troubleshooting

**REPL doesn't show prompt:**
- Check terminal supports readline (most Unix shells do)
- History stored in `$TMPDIR/sysml-repl.history`

**Import errors after build:**
- Run `go mod tidy`
- Verify Go version: `go version` (need 1.25+)

**Execution stops with "limit exceeded" or "exceeded max":**
- The run spent one of its budgets; the message names the variable that raises it (see [Environment Variables](#environment-variables))
- If the model does not terminate, the budget is reporting a real bug — raising it only delays the error

**Syntax errors:**
- SysML v2 textual notation only (no graphical/XMI)
- Keywords are case-sensitive
- Multiplicity goes after relationships: `part x subsets y [0..1];`

---

## Getting Help

- **GitHub Issues:** Report bugs or request features
- **Discussions:** Ask questions about SysML v2 usage
- **Spec Reference:** [OMG SysML v2.1 Beta 1 Specification](https://www.omg.org/spec/SysML/2.0) (2026-05 release)

---

**Happy modeling!** 🚀
