# OOSEM demo

[`oosem-demo.sysml`](oosem-demo.sysml) works a small Earth-observation mission — a satellite
that spots wildfire ignitions early — through the six activities of the Object-Oriented Systems
Engineering Method, using the bundled [`OOSEM`](../../docs/project/oosem-library.md) library.
Each package is stamped with the activity it belongs to, and each artefact carries the method
keyword that both types it and adds it to the method's base usage:

```sysml
@OOSEMPackage { kind = OOSEMPackageKind::systemRequirements; }
package 'System Requirements' {
    #system part def 'FireWatch Satellite';
    #systemRequirement requirement def 'Revisit Time' {
        subject satellite : 'FireWatch Satellite';
        #mop attribute revisit : DurationValue = 2 [h];
    }
}
```

| Package | OOSEM activity | What it holds |
| --- | --- | --- |
| `Enterprise Model` | analyse stakeholder needs | the enterprise and its stakeholders, the `#asIs` aerial-patrol enterprise and the `#toBe` satellite enterprise, the `#causalAnalysis` of late detection, and the `#enterpriseUseCase` |
| `Stakeholder Needs` | analyse stakeholder needs | a `#stakeholderNeed` with its `#moe` |
| `Mission Requirements` | analyse stakeholder needs | a `#missionRequirement` and the `#derivation` that traces it to the need |
| `System Requirements` | analyse system requirements | the `#system` of interest, `#user`, `#environment`, `#ioEntity` and `#store` items, the `#systemContext`, the `#systemUseCase`, and `#systemRequirement`s with `#mop`s derived from the mission requirement |
| `Logical Architecture` | define logical architecture | `#logical` components, a `#logicalScenario` decomposing the use case into actions, and a `#componentRequirement` |
| `Physical Architecture` | synthesise physical architecture | `#hardware`, `#software`, `#data` and `#operationalProcedure` components distributed over `#node`s |
| `Trade Studies` | optimise and evaluate alternatives | two camera alternatives to weigh with `TradeStudies` |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/oosem-demo/oosem-demo.sysml -validate
```

The model also validates cleanly under the OMG pilot implementation once the library file is
alongside it, so what it writes is standard SysML v2 textual notation.
