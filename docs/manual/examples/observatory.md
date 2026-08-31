# Telescope Mass Report

Mass rollup and requirement status for the telescope assembly.

This report is *generated* from the model by `sysml -render-document` [(OpenSysML)](<https://opensysml.org/>) — masses are tabulated in [Subsystem Masses](#breakdown)

<a id="breakdown"></a>

## Subsystem Masses

*All subsystems by mass*

| name | mass |
| --- | --- |
| mount | 15 |
| optics | 8.5 |
| segmentControl | 20 |

*Subsystems grouped by zone*

**zone: support**

| zone | name | mass |
| --- | --- | --- |
| support | mount | 15 |

**zone: payload**

| zone | name | mass |
| --- | --- | --- |
| payload | optics | 8.5 |
| payload | segmentControl | 20 |

### Heavy Subsystems

Subsystems at or above 10 kg:

1. mount
2. segmentControl

## Mass Requirement

*Parts satisfying the mass requirement*

| name | qualifiedName |
| --- | --- |
| telescope | Observatory::telescope |

*Verifications of the mass requirement*

| qualifiedName |
| --- |
| Observatory::massVerification |

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

*Telescope part tree, left to right*

```mermaid
%% tree rendering (the diagram states kind "tree")
flowchart LR
  n0["part Observatory::telescope"]
  n1["part optics (Subsystem)"]
  n2["attribute mass"]
  n1 --- n2
  n3["attribute zone"]
  n1 --- n3
  n0 --- n1
  n4["part segmentControl (Subsystem)"]
  n5["attribute mass"]
  n4 --- n5
  n6["attribute zone"]
  n4 --- n6
  n0 --- n4
  n7["part mount (Subsystem)"]
  n8["attribute mass"]
  n7 --- n8
  n9["attribute zone"]
  n7 --- n9
  n0 --- n7
```
