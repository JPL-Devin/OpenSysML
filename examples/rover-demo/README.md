# Rover demo

[`rover.sysml`](rover.sysml) is one rover modelled far enough to drive every part
of OpenSysML from a single file: its structure, the behavior it performs, the
budget a solver can answer about, and the views it is presented through. This
walkthrough runs it from the CLI, then from the REPL, then from Python, and every
output below is what the commands print.

The solver sections need `z3` or `cvc5` on `PATH` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional).
`%optimize` needs z3 specifically.

The four packages:

| Package | What it holds |
| --- | --- |
| `Rover` | the platform: battery, solar array, mobility with six wheels, mast and camera, arm and drill, computer, antenna, the power line and data link between them, and two rovers of it |
| `RoverBehavior` | three calculations, the `SurfaceOps` action a sol runs, the `Modes` state machine, and a controller that exhibits it |
| `RoverSolver` | the sol energy budget, a sol nobody can plan, three variation points, and three optimization cases |
| `RoverViews` | one view per rendering kind, a viewpoint the rovers are checked against, and a view reaching only the critical parts |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/rover-demo/rover.sysml -validate
```

```
✓ package Rover
✓ package RoverBehavior
✓ package RoverSolver
✓ package RoverViews
✓ examples/rover-demo/rover.sysml: no errors
```

**Plan a traverse.** `-calc` invokes a calculation with positional arguments: the
distance between two waypoints, and what driving it costs at the rover's draw.

```bash
./bin/sysml examples/rover-demo/rover.sysml \
  -calc "RoverBehavior::Traverse(120.0, 90.0)" \
  -calc "RoverBehavior::energyForDrive(150.0, 0.042, 120.0)"
```

```
✓ RoverBehavior::Traverse(120.0, 90.0)
  = 150.0
✓ RoverBehavior::energyForDrive(150.0, 0.042, 120.0)
  = 119.04761904761904
```

**Run the sol.** `SurfaceOps` forks imaging and sampling, joins them, tallies what
the day cost, and downlinks only what the power budget left room for. The sampling
branch is a single node stating a flow of its own — the drill, then the stow — so
the join waits for that whole flow, not just for the node to be entered.

```bash
./bin/sysml examples/rover-demo/rover.sysml -action RoverBehavior::SurfaceOps
```

```
✓ Action completed
  Final state: Completed
  Results:
    chargeAvailable = 1200.0
    chargeLeft = 1040.0
    coreFrames = 3
    downlinked = 15
    framesHeld = 15
    imageFrames = 12
    imagingCost = 40.0
    samplingCost = 120.0
```

**Run the modes.** The transitions are timed, so `-advance` says how much
simulated time to run for; without it only the initial transition is taken.

```bash
./bin/sysml examples/rover-demo/rover.sysml -state RoverBehavior::Modes -advance 40
```

```
✓ Advanced to 40.0 (4 event(s) processed)
  Current state: done
✓ State machine completed (a transition reached `done`)
```

Add `-instantiate RoverBehavior::controller -state "RoverBehavior::Modes controller"`
to run the machine as that object's own, and `-trace` to see every step.

**Check what holds.** A verdict is about an object, so instantiate the rover the
question is about; `-satisfy=<name>` evaluates the assertions one element states.

```bash
./bin/sysml examples/rover-demo/rover.sysml \
  -instantiate Rover::heavyMockup -constraint Rover::Platform::withinMassBudget
./bin/sysml examples/rover-demo/rover.sysml -satisfy=RoverSolver::plans
```

```
✗ Constraint Rover::Platform::withinMassBudget failed (on Rover::heavyMockup ID: 1)
  Assertion evaluated to false: mass <= 1000.0
✓ satisfy solIsBudgeted by sol42 holds (on RoverSolver::sol42 ID: 1)
✗ satisfy solIsBudgeted by sol43 fails (on RoverSolver::sol43 ID: 2)
  Required condition evaluated to false: plan.driveEnergy + plan.scienceEnergy + plan.housekeepingEnergy <= 1200
```

A failing check exits nonzero, so these run in CI as they stand.

**Render the views.** `-render` writes one view in the form its `render` member
states, `-render-form` asks for another, and `-render-all` writes every view of
the model into a directory.

```bash
./bin/sysml examples/rover-demo/rover.sysml -render RoverViews::interfaces
./bin/sysml examples/rover-demo/rover.sysml -render RoverViews::partsTable -render-form text
./bin/sysml examples/rover-demo/rover.sysml -render-all rendered
```

```
%% RoverViews::interfaces — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["part def Rover::Platform"]
    n3["part battery (Battery)"]
    n5["part mobility (Mobility)"]
    …
  end
  n3 ---|"drivePower"| n5
  n6 ---|"imageryLink"| n8
  n3 -.->|"of Charge"| n5
  n6 -.->|"of Frame"| n8
```

```
wrote rendered/RoverViews.interfaces.mmd (mermaid, 547 bytes)
wrote rendered/RoverViews.overview.mmd (mermaid, 914 bytes)
wrote rendered/RoverViews.overview.interfaceSubview.mmd (mermaid, 563 bytes)
wrote rendered/RoverViews.modes.mmd (mermaid, 627 bytes)
wrote rendered/RoverViews.sol.mmd (mermaid, 678 bytes)
wrote rendered/RoverViews.partsTable.md (markdown, 833 bytes)
wrote rendered/RoverViews.criticalParts.mmd (mermaid, 881 bytes)
```

## From the REPL

```bash
./bin/sysml
```

```
%load examples/rover-demo/rover.sysml
```

### The rover as an object

`%instantiate` builds the object and starts the behaviors its type exhibits;
`%features` shows what it holds, down to the six wheels and both ends of every
connection.

```
%instantiate Rover::curiosity
%features Rover::curiosity
%eval in Rover::curiosity : mass
```

### Stepping the sol

`%action` starts a debugging session, `%tokens` shows where the tokens are,
`%step` advances one step and `%continue` runs to the end.

```
%action RoverBehavior::SurfaceOps
%tokens
%step
%continue
```

```
Active tokens (1):
  Token 1 @ start
  Values:
    chargeAvailable = 1200.0
    …
✓ Action completed
  Results:
    chargeLeft = 1040.0
    downlinked = 15
    framesHeld = 15
```

`%break <node>` stops at a node — `%break sync` to see the join wait for both
branches, one of them still inside the sampling flow:

```
%action RoverBehavior::SurfaceOps
%break sync
%continue
%tokens
```

```
Active tokens (2):
  Token 2 @ sync
  Token 3 @ drill
```

### Driving the machine of an object

`%instantiate` starts the machine the controller exhibits, `%state <part>` binds
to that object's own running machine, and `%advance` dispatches the timed
triggers. `%current` shows the active configuration, which is nested while the
rover is driving.

```
%instantiate RoverBehavior::controller
%state RoverBehavior::controller
%advance 5
%current
```

```
✓ Advanced to 5.0 (1 event(s) processed)
  Current state: rolling
Current state: rolling
State stack (active configuration):
  0. driving
  1. rolling

State data:
  faults = 0
  framesCaptured = 0
  odometer = 21.0
```

Entering `rolling` entered `driving` around it and ran the entry action that
turned the odometer. Carry on to the end of the mission, then call an operation
of the object with `%invoke`:

```
%advance 10
%advance 20
%current
%invoke RoverBehavior::controller recordPass frames=15
%features RoverBehavior::controller
%eval in RoverBehavior::controller : modes.odometer
%eval in RoverBehavior::controller : log.frames
```

```
✓ State machine completed (a transition reached `done`)
✓ Invoked recordPass on object #1 of "RoverBehavior::controller"
Instance: RoverBehavior::controller (ID: 1)
Features:
  odometer = 0.0
  framesHeld = 15
  log = Instance(ID: 3)
    frames = 15
    passes = 1
  modes = Instance(ID: 2)
    odometer = 21.0
    framesCaptured = 12
    faults = 0
    idle = <unknown>
    driving = Instance(ID: 4)
      rolling = <unknown>
      holding = <unknown>
    science = <unknown>
    safing = <unknown>
    idle_to_rolling = <unknown>
    rolling_to_holding = <unknown>
    rolling_to_safe = <unknown>
    driving_to_science = <unknown>
  recordPass = Instance(ID: 5)
    frames = <unset>
    store = <unknown>
✓ modes.odometer (on RoverBehavior::controller ID: 1)
  = 21.0
✓ log.frames (on RoverBehavior::controller ID: 1)
  = 15
```

An entry action writes the attribute of the machine it is declared in, not the
like-named one of the object exhibiting it. `%current` reports the executor's
current state data, while `%features` and `%eval` read the same value through
the materialized `modes` performance occurrence. The controller's distinct
`odometer` remains `0.0`.

`recordPass` writes two things: the controller's own tally, and — through the
chain its assignment names — the `log` part beside it.

A write has to fit what it is written to, so the wrong kind of value is a refusal
rather than a stored surprise — and the refusal comes from analysis, before
anything runs. Editing `store` to say `assign framesHeld := "fifteen";` and
reloading reports:

```
examples/rover-demo/rover.sysml:351:38: error: cannot bind String value to a feature typed by Integer
                assign framesHeld := "fifteen";
                                     ^~~~~~~~~
```

### Evaluating and solving the sol budget

`%constraint` and `%satisfy` evaluate what *does* hold of an object; `%check` and
the commands after it ask the solver what *can* hold.

```
%calc RoverBehavior::downlinkable 15 8.0
%instantiate RoverSolver::sol42
%constraint RoverSolver::sol42::fitsTheBattery
%satisfy RoverSolver::plans
```

`%check` answers whether the whole budget can hold at all, and `%solve` fills in
a plan that satisfies it:

```
%check RoverSolver::SolBudget
%solve RoverSolver::SolBudget
```

```
✓ Requirement SolBudget is satisfiable (z3, 8ms)
  RoverSolver::SolBudget::'plan.driveEnergy' = 200
  RoverSolver::SolBudget::'plan.housekeepingEnergy' = 300
  RoverSolver::SolBudget::'plan.scienceEnergy' = 150
✓ Requirement SolBudget has values satisfying it (z3, 7ms)
  Synthesised:
    RoverSolver::SolBudget::'plan.driveEnergy' = 200
    RoverSolver::SolBudget::'plan.housekeepingEnergy' = 300
    RoverSolver::SolBudget::'plan.scienceEnergy' = 150
  One witness: a solver may answer with any of the assignments that satisfy it.
```

`OverbookedSol` asks for more than the battery holds, so `%explain` names the
conditions that conflict, minimally:

```
%check RoverSolver::OverbookedSol
%explain RoverSolver::OverbookedSol
```

```
✗ Requirement OverbookedSol is unsatisfiable: 4 conditions conflict (z3, 37ms)
  Every condition below is needed: dropping any one leaves the rest satisfiable.
  1. required condition: `plan.driveEnergy + plan.scienceEnergy + plan.housekeepingEnergy <= 1200` …
  2. required condition: `plan.driveEnergy >= 600` …
  3. required condition: `plan.scienceEnergy >= 500` …
  4. required condition: `plan.housekeepingEnergy >= 300` …
```

`%configure` asks which rovers the build constraint permits — one consistent
selection, a selection you choose, or every one of them:

```
%configure RoverSolver::roverFamily::buildIsConsistent
%configure RoverSolver::roverFamily::buildIsConsistent instrument=drill
%configure RoverSolver::roverFamily::buildIsConsistent all
```

```
✓ the chosen variants are consistent with Constraint buildIsConsistent (z3, 9ms)
  Already fixed:
    RoverSolver::roverFamily::instrument = …::instrument::drill  (chosen)
  Synthesised:
    RoverSolver::roverFamily::antenna = …::antenna::xBand
    RoverSolver::roverFamily::wheels = …::wheels::titanium
✓ Constraint buildIsConsistent permits 5 selections of variants, which are all of them (z3, 11ms)
```

Choosing the drill forced the X-band antenna and the heavier wheels: of the eight
builds the three variation points spell, five are consistent.

`%optimize` improves an analysis case's objectives, lexicographically when it
states several:

```
%optimize RoverSolver::BestSol
%optimize RoverSolver::LightestRover
%optimize RoverSolver::FarthestThenScience
```

```
✓ Analysis BestSol is optimized (z3, 7ms)
  maximize mostScience = `scienceEnergy`: 700
  RoverSolver::BestSol::driveEnergy = 200
  RoverSolver::BestSol::housekeepingEnergy = 300
  RoverSolver::BestSol::scienceEnergy = 700
✓ Analysis FarthestThenScience is optimized (z3, 7ms)
  maximize farthest = `traverse`: 120
  maximize mostScience = `scienceEnergy`: 420
```

The best sol drives the least the plan allows and spends the rest on science; the
two-objective case drives as far as it can first, and takes the most science
among the plans that reach that far. `LightestRover`'s optimum is a quantity, and
is reported in grams, the unit the runtime normalizes mass to.

### Views

`%view` shows what a view exposes and how it conforms to the viewpoints it
satisfies; `%render` draws it.

```
%view RoverViews::overview
```

```
view RoverViews::overview
  exposes
    Rover::curiosity (partUsage)
    Rover::heavyMockup (partUsage)
  nested views
    RoverViews::overview::interfaceSubview (viewUsage)
  viewpoint conformance
    satisfy operationsPerspective: violated
      concern mass: violated
        Rover::heavyMockup: … require condition evaluated to false: rover.mass <= maxMass
```

```
%render RoverViews::modes
%render RoverViews::sol mermaid
%render RoverViews::criticalParts
%render RoverViews::partsTable markdown
```

```
RoverViews::modes - state rendering (view def StateTransitionView)

state def RoverBehavior::Modes
  state idle (initial)
  state driving
    state rolling (entry)
    state holding
  state science (entry)
  state safing (entry)
  state done (completes)

transitions:
  idle -> rolling: idle_to_rolling: after 5 [s]
  driving -> science: driving_to_science: after 20 [s]
  rolling -> holding: rolling_to_holding: after 10 [s]
  rolling -> safing: rolling_to_safe: [faults > 0]
  science -> done
```

`criticalParts` exposes `Rover::**[@Rover::Critical]`, so it reaches only the
parts marked with the `Critical` metadata: the battery, the wheels, the mobility
system and the computer.

## From Python

[`rover_demo.py`](rover_demo.py) asks the same questions through the `opensysml`
client, which talks to the `sysml-grpc` service
([9. From Python](../../docs/guide/09-python.md) covers installing both):

```bash
pip install opensysml
python examples/rover-demo/rover_demo.py
```

```
== the rover as an object
mass          899.0 kg
drive power   120.0 W at 0.042 m/s
wheels        6
battery       1200.0 of 1600.0 Wh

== planning a traverse
150.0 m costs 119.0 Wh of the 1200.0 Wh aboard

== running the sol
frames held   15
core frames   3 (the sampling branch ran through)
downlinked    15
charge left   1040.0 Wh

== running the mode machine
states        idle -> driving -> rolling -> holding -> science -> done
wrote         faults=0, framesCaptured=12, odometer=21.0

== checking the sol budget
✓ satisfy solIsBudgeted by sol42 holds (on RoverSolver::sol42 ID: 1)
✗ satisfy solIsBudgeted by sol43 fails (on RoverSolver::sol43 ID: 2): condition evaluated to false: …
both hold     False
heavy mockup  over the mass budget
```

The client covers loading, instances, calculations, action and state execution,
and verification. The solver commands and view rendering are CLI and REPL only,
so a script that needs them shells out to `sysml`.
