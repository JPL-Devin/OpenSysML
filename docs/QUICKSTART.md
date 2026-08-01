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

**macOS (Apple Silicon):**
```bash
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/sysml-darwin-arm64.tar.gz
tar xzf sysml-darwin-arm64.tar.gz
sudo mv sysml-darwin-arm64 /usr/local/bin/sysml
chmod +x /usr/local/bin/sysml
```

**macOS (Intel):**
```bash
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/sysml-darwin-amd64.tar.gz
tar xzf sysml-darwin-amd64.tar.gz
sudo mv sysml-darwin-amd64 /usr/local/bin/sysml
chmod +x /usr/local/bin/sysml
```

**Windows:**
Download `sysml-windows-amd64.zip` from [releases](https://github.com/Open-MBEE/Systemica/releases/latest), extract, and add to PATH.

**Available binaries:**
- `sysml` — Interactive REPL
- `sysml-lsp` — Language Server Protocol server

### Option 2: Build from Source

**Prerequisites:**
- Go 1.25 or later
- Git

**Build:**
```bash
git clone https://github.com/Open-MBEE/Systemica.git
cd Systemica
go build -o sysml ./cmd/sysml
go build -o sysml-lsp ./cmd/sysml-lsp
```

**Install (optional):**
```bash
sudo mv sysml sysml-lsp /usr/local/bin/
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

```sysml
sysml> part Wheel {
...>     attribute diameter : Real;
...>     attribute width : Real;
...> }
✓ Wheel
```

#### 2. Define a Vehicle

```sysml
sysml> part Vehicle {
...>     part engine {
...>         attribute power : Real = 150.0;
...>     }
...>     part wheels : Wheel[4] {
...>         :>> diameter = 16.0;
...>         :>> width = 7.5;
...>     }
...> }
✓ Vehicle
```

#### 3. Instantiate and Inspect

```sysml
sysml> %instantiate Vehicle
Created instance: Vehicle (ID: 1)

sysml> %slots Vehicle
Instance: Vehicle (ID: 1)
  engine: Instance(ID: 2)
  wheels: [Instance(ID: 3), Instance(ID: 4), Instance(ID: 5), Instance(ID: 6)]

sysml> %instances
Instances:
  Vehicle (ID: 1)
```

#### 4. Evaluate Expressions

```sysml
sysml> attribute wheelCount = 4;
✓ wheelCount

sysml> attribute totalDiameter = wheelCount * 16.0;
✓ totalDiameter

sysml> %eval totalDiameter
totalDiameter = 64.0
```

---

### Working with Files

Create a file `my_model.sysml`:

```sysml
package MyModel {
    part Sensor {
        attribute reading : Real;
        attribute threshold : Real = 100.0;
        
        calc def isTriggered : Boolean {
            reading > threshold
        }
    }
    
    part System {
        part sensors : Sensor[3];
    }
}
```

Load it in the REPL:

```bash
$ sysml
sysml> %load my_model.sysml
Loaded: my_model.sysml

sysml> %list
Declarations:
  MyModel (package)
  MyModel::Sensor (part def)
  MyModel::System (part def)

sysml> %instantiate MyModel::System
Created instance: MyModel::System (ID: 1)

sysml> %slots MyModel::System
Instance: MyModel::System (ID: 1)
  sensors: [Instance(ID: 2), Instance(ID: 3), Instance(ID: 4)]
```

---

## REPL Commands

| Command | Description |
|---------|-------------|
| `%help` | Show help message |
| `%list` | List all declarations in current session |
| `%clear` | Clear session (reset all declarations) |
| `%load <file>` | Load .sysml file into session |
| `%instantiate <name>` | Create instance from part definition |
| `%slots <name>` | Show instance slots and values |
| `%instances` | List all created instances |
| `%eval <expr>` | Evaluate expression |
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

### 4. Behavioral Models (Parsed, not yet executable)

```sysml
action TrafficLight {
    first start startNode;
    done end endNode;
    
    action green;
    action yellow;
    action red;
    
    then start green;
    then green yellow;
    then yellow red;
    then red end;
}
```

**Status:** Action bodies are parsed into AST (control flow nodes, succession edges). Execution coming in Tier 4-5.

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
      "command": "/path/to/sysml-lsp",
      "args": [],
      "filetypes": ["sysml", "kerml"]
    }
  ]
}
```

**Current LSP capabilities:**
- Document synchronization
- Basic diagnostics (syntax errors)

**Coming soon:**
- Hover (type info)
- Go to definition
- Completion
- Workspace symbols
- References

---

## Examples

Check `examples/` directory:
- `runtime_repl_demo.md` — Full runtime walkthrough
- `behavior_demo.sysml` — Action body examples

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
- History stored in `/tmp/sysml.history`

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
- **Spec Reference:** [OMG SysML v2.0 Specification](https://www.omg.org/spec/SysML/2.0)

---

**Happy modeling!** 🚀
