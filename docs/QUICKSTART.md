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
`SHA256SUMS.txt` are produced from the next tagged release onward; for earlier releases use
the single-binary archives. `SHA256SUMS.txt` covers every archive and every published
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
  ...>     attribute diameter : Real;
  ...>     attribute width : Real;
  ...> }
✓ part def Wheel
```

Each accepted declaration is echoed back as `✓ <kind> <name>`.

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
```

---

## Saving and Converting

`%save` writes the session out. The format follows the extension — `.sysml` for
notation, `.ttl` for RDF Turtle:

```bash
sysml> %save my_model.sysml
saved 148 bytes of sysml to my_model.sysml

sysml> %save my_model.ttl
saved 1540 bytes of ttl to my_model.ttl
```

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
part Engine {
    attribute power : Real = 200.0;
}

part Car {
    part engine : Engine {
        :>> power = 250.0;  // Redefine nested feature
    }
}
```

Instantiate and inspect:
```sysml
%instantiate Car
%slots Car
Instance: Car (ID: 1)
  engine: Instance(ID: 2)
    power: 250.0
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
✓ distance

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
✓ ValidSpeed

sysml> %constraint ValidSpeed
✓ Constraint ValidSpeed passed
```

**Requirements:**
```sysml
sysml> requirement SafetyReq {
...>     assume 65 > 0;
...>     require 100 > 50;
...> }
✓ SafetyReq

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
✓ SimpleWorkflow

sysml> %action SimpleWorkflow
✓ Started action executor for "SimpleWorkflow"
  State: Running
  Tokens: 1

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
...>
...>     start then green;
...> }

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: start
  Time: 0.00
  Events: 1

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
- `%action <name>` — Start action debugging session
- `%step` — Advance all tokens one step
- `%continue` — Run to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with data
- `%break <node>` — Set breakpoint on a named node; `%continue` stops when a token reaches it
- `%stop` — Stop debugging

**State machine debugging commands:**
- `%state <name>` — Start state machine debugging
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
