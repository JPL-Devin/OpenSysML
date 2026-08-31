# Telescope Mass Report

Mass rollup for the telescope assembly. Masses are in kg \| not \#grams, \*not\* \_lbs\_, \`raw\`, \<b>\&plain\</b>.

## Subsystem Masses \| by \*name\*

*All subsystems by mass*

| name | mass |
| --- | --- |
| baffle\|shroud \*tricky\* | 1.5 |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |

### Heavy Subsystems

mount segmentControl

1. mount
2. segmentControl

### Missing Subsystems

| name | mass |
| --- | --- |

## Diagrams

*Imaging chain interconnection*

```mermaid
%% Observatory::interconnectView — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["part Observatory::imagingChain"]
    n1["part camera (Camera)"]
    n2["part recorder (Recorder)"]
  end
  n1 ---|"link"| n2
```

*Observatory states, left to right*

```mermaid
%% state rendering (the diagram states kind "state")
stateDiagram-v2
  direction LR
  state "state Observatory::operatingStates (ObservatoryStates)" as n0 {
    state "state idle (initial)" as n1
    [*] --> n1
    state "state observing" as n2
  }
  n1 --> n2
  n2 --> n1
```

## Declared Types

The declared type of the telescope, by relationship traversal.

*Type of telescope*

| element |
| --- |
| Observatory::Assembly \*frame\* |

- Assembly \*frame\*
