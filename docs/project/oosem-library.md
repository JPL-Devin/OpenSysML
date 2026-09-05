# OOSEM library — design

Status: **implemented.** `internal/core/libs/stdlib/OpenSysML Libraries/OOSEM.sysml` is bundled
with the other OpenSysML extensions, enters the same conformance, strict-notation and snapshot
gates, and is exercised by `examples/oosem-demo/oosem-demo.sysml`
(`TestExamplesAnalyseCleanly`). Both the library and the example also validate cleanly under the
pinned OMG pilot implementation, so the notation they use is standard SysML v2.

## The problem

The Object-Oriented Systems Engineering Method (OOSEM, INCOSE OOSEM Working Group; Friedenthal,
Moore & Steiner, *A Practical Guide to SysML*, ch. 17) is a top-down, scenario-driven method:
analyse stakeholder needs, analyse system requirements, define the logical architecture,
synthesise candidate physical architectures, optimise and evaluate alternatives, validate and
verify. In SysML v1 it was carried by a UML profile of stereotypes (`«system»`, `«logical»`,
`«node»`, `«moe»`, …) and a package template. SysML v2 has no profiles; a method vocabulary is
a *library*: definitions, base usages and semantic metadata.

Within Open-MBEE the only prior SysML v2 material is the stub `OOSEM.sysml` in
[DesertKite.sysml](https://github.com/Open-MBEE/DesertKite.sysml) — `MOE`/`MoP` abstract
attributes and three empty view placeholders. The closest worked library-development pattern is
[structured-use-cases](https://github.com/Open-MBEE/structured-use-cases), which extends native
`UseCase`/`Action`/`ConstraintCheck` and attaches a `SemanticMetadata` keyword to each
extension. This library follows that pattern and keeps the stub's names reachable.

## Design

**One shape for every artefact.** Each OOSEM concept is three declarations:

```sysml
part def SystemOfInterest :> Part { doc /* ... */ }
abstract part systemsOfInterest : SystemOfInterest[0..*] nonunique :> parts;
metadata def <system> SystemOfInterestMetadata :> SemanticMetadata {
    :> annotatedElement : SysML::PartDefinition;
    :> annotatedElement : SysML::PartUsage;
    :>> baseType = systemsOfInterest meta SysML::Usage;
}
```

A modeller may write `part sat : SystemOfInterest;` and get the type alone, or
`#system part sat : Satellite;` and get the type *and* membership in `systemsOfInterest`
(SysML v2 §7.27.4 semantic metadata: the annotated usage implicitly subsets `baseType`, a
definition implicitly specialises its type). Every base usage is a subset of the native
`parts`/`items`/`actions`/`requirementChecks`/`useCases`/`analysisCases`, so `%search` and the
document queries reach OOSEM artefacts by method role without any OOSEM-specific tooling.

**Reuse before invention.** Nothing is redeclared that the OMG domain libraries already hold;
the library re-exports them instead so that `import OOSEM::*` is the whole method:

| OOSEM concept | Comes from |
| --- | --- |
| measures of effectiveness / performance (`#moe`, `#mop`) | `ParametersOfInterestMetadata` |
| trade study, objective, evaluation function | `TradeStudies` |
| requirement derivation (`#derivation`, `#original`, `#derive`) | `RequirementDerivation` |
| causal analysis chains (`#cause`, `#effect`) | `CauseAndEffect` |
| requirement text, subject, actors, stakeholders, categories | `Requirements` |
| use cases, actors, subjects, included use cases | `UseCases` |
| verification, analysis | `VerificationCases`, `AnalysisCases` |

The DesertKite stub's `'OOSEM Measures'::MOE` and `MoP` survive as aliases of the standard base
usages, so a model written against the stub resolves the same features.

**What the library declares.**

| Activity | Definitions / keywords |
| --- | --- |
| Enterprise & stakeholder needs | `Enterprise` `#enterprise`, `Stakeholder` `#stakeholderRole`, `CausalAnalysis` `#causalAnalysis`, `EnterpriseUseCase` `#enterpriseUseCase`, `StakeholderNeed` `#stakeholderNeed`, `MissionRequirement` `#missionRequirement` |
| System requirements | `SystemOfInterest` `#system`, `ExternalSystem` `#externalSystem`, `User` `#user`, `Environment` `#environment`, `SystemContext` `#systemContext` (with `systemOfInterest`, `externalSystems`, `users`, `environment` parts), `SystemUseCase` `#systemUseCase`, `IOEntity` `#ioEntity`, `Store` `#store`, `SystemRequirement` `#systemRequirement` |
| Logical architecture | `LogicalComponent` `#logical`, `LogicalScenario` `#logicalScenario`, `Node` `#node`, `ComponentRequirement` `#componentRequirement` |
| Physical architecture | `PhysicalComponent` `#physical`, `HardwareComponent` `#hardware`, `SoftwareComponent` `#software`, `OperationalProcedure` `#operationalProcedure`, `DataComponent` `#data` |
| Lifecycle | `#asIs` / `#toBe` marking (`EnterpriseStateKind`) |
| Package template | `@OOSEMPackage { kind = OOSEMPackageKind::…; }` — the activity a package holds, for navigating a model by method rather than by package name |

Naming follows the SysML v1 OOSEM profile where the word is not a SysML v2 reserved keyword.
`stakeholder`, `analysis` and `verification` are keywords, hence `#stakeholderRole`,
`OOSEMPackageKind::analysisModel` and `::verificationModel`; the pilot's grammar rejects them as
names where OpenSysML's would not, and the library is written so both accept it.

## Alternatives considered

- **Views and viewpoints for the OOSEM diagram set** (the DesertKite stub's placeholders). Left
  out: `Views`/`StandardViewDefinitions` already cover the rendering kinds, and OOSEM's diagrams
  are the SysML diagrams applied to the artefacts above; a view library would restate them.
- **Requirement categories as `RequirementCheck` subtypes vs. metadata only.** Subtypes were
  chosen so a requirement can be typed by its level with no keyword, and so the level is an
  ordinary specialisation the checker and RDF export already understand.
- **Redeclaring MOE/MOP.** Rejected; `ParametersOfInterestMetadata` is normative and its
  `#moe`/`#mop` are what other libraries and tools recognise.

## Known limitations

- The library marks *what* an artefact is, not *whether the method was followed*: there is no
  validation pass that, say, requires every `#systemRequirement` to derive from a
  `#missionRequirement`. That is a natural follow-on as a `DocumentQueries` query or a
  constraint-tier pass.
- Views for the OOSEM package template are not provided (see above).

## Parser fix made along the way

Prefix metadata before the two-word kind `use case` (`#systemUseCase use case def …`) was
rejected as `expected a namespace member`, because the prefix lookahead in
`parser.leadingPrefixIsDefUsage` only recognised single-token kind keywords. Fixed, with the
golden fixture `internal/core/parser/testdata/parse/prefix_metadata_use_case.sysml`.
