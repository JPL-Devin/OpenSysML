# Mars 2020 Perseverance rover

[`perseverance.sysml`](perseverance.sysml) models the Mars 2020 Perseverance rover: its
physical decomposition, the interfaces its subsystems connect through, its mission and
surface-operations behavior, the requirements the design carries, a trade study the solver can
optimize, the individual mission that landed in Jezero crater, and the views a stakeholder
reads it through. Figures follow the published mission characteristics (a 1025 kg rover, a
110 W MMRTG, 43 sample tubes, seven science instruments).

The model spans nine packages:

| Package | What it holds |
| --- | --- |
| `MarsSurfaceEnvironment` | metadata definitions, an attribute definition, an enumeration |
| `RoverFlows` | the items that flow: telemetry, commands, power, samples, atmosphere, oxygen |
| `RoverInterfaces` | port definitions (with conjugation) and the power/data/space-link interface definitions |
| `RoverStructure` | every subsystem as a part definition — MMRTG, batteries, compute elements, telecom, mobility, arm, sample caching, the seven instruments, Ingenuity — assembled and connected in `Rover`, with quantities in SI units and a computed payload mass |
| `RoverBehavior` | hierarchical state machines for the mission phases and surface operations, a sol-plan action with a fork/join and a decision, the sampling action, and a calculation |
| `RoverRequirements` | four requirements with `require constraint` bodies, the mission use case, a satisfaction assertion, and a verification case |
| `RoverAnalyses` | a payload-mass trade study for `%optimize` |
| `Mars2020Mission` | the mission as an occurrence, an individual, a snapshot of landing and a timeslice of the prime mission |
| `RoverViews` | rendered views, including recursive metadata-filtered exposure of the mission-critical and PI-delivered elements |

## Exercising it

Everything below runs against the built `bin/sysml` from the repo root.

Load and check, instantiate the rover, evaluate its mass, and run the MOXIE calculation:

```bash
./bin/sysml -instantiate RoverStructure::Rover \
    -eval "RoverStructure::Rover::totalMass" \
    -calc "RoverBehavior::OxygenProduced(10.0, 5.0)" \
    examples/mars2020-perseverance/perseverance.sysml
```

Run the sampling sequence and the sol plan to completion:

```bash
./bin/sysml -action "RoverBehavior::AcquireSample" \
    -action "RoverBehavior::ExecuteSolPlan" \
    examples/mars2020-perseverance/perseverance.sysml
```

In the REPL, check the requirements and the satisfaction assertion, optimize the trade study
(needs z3), step the surface state machine, and read the filtered views:

```
sysml> %load examples/mars2020-perseverance/perseverance.sysml
sysml> %satisfy
sysml> %requirement RoverRequirements::powerHolds
sysml> %optimize RoverAnalyses::PayloadMassTrade
sysml> %state RoverBehavior::SurfaceStates
sysml> %view RoverViews::criticalItems
sysml> %render RoverViews::structure
```
