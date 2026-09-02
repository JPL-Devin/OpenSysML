# Validation-Constraint Census

**Pilot:** [SysML v2 Pilot Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) release `2026-07`, commit `c7fc737d56da9e2d78f9d7df6d38efbec2e7e965`, artifact `jupyter-sysml-kernel 0.61.0` — the pin in `scripts/pilot-pin.sh`
**Jar:** `jupyter-sysml-kernel-0.61.0-all.jar` (`sha256:602b53fa64d5af84480aa00e06e590a140d7d5c3651d4d76ac0e0055f89f0079`), provisioned by `./scripts/download-pilot-validator.sh`
**Run:** `go run ./cmd/validation-census` (restates the summary line below from the baseline); `go run ./cmd/validation-census -check` (the gate); `go run ./cmd/validation-census -update` (re-extracts the names from the jar, keeping every recorded status)
**Baseline:** [validation-constraints-baseline.json](validation-constraints-baseline.json) — the constraint names read from the pinned jar, with the pin, the jar digest, the extraction method and each name's census status
**Evidence:** `cmd/validation-census/testdata/probes/` — one minimal violating model per implemented row, run by `go test ./cmd/validation-census`

**Labels:** the file and type names quoted in the Implementation column carry the wave a pass was
built in (`w8c_`, `W10B…`); they are code identifiers, not product terms, and a reader who only
wants the verdicts can ignore them.

The pilot validators name every constraint they check (`validateNamespaceDistinguishability`,
`validateUsageType`, …), OpenSysML does not: its diagnostics are worded for the reader and its
checks are grouped by pass, so before this census 157 of the pilot's names occurred nowhere in this repository. This
page is the map between the two. *Not named* is not *not implemented* — `Duplicate of other
owned member name` is `validateNamespaceDistinguishablity` — and *implemented* is not *named*:
a row is ✅ or ⚠️ only when a model that violates the constraint was run through OpenSysML and
the diagnostic it reports was recorded as that row's probe.

## Summary

**Census:** 108 of 217 named constraints are reported by OpenSysML — 97 ✅ faithful and 11 ⚠️ approximate; 27 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 82 ❔ unknown.

The figures on that line are written by `go run ./cmd/validation-census` from the baseline's
statuses; `-check` fails on a hand-edited figure, on a table row the baseline does not record, on
a baseline name the table lacks, on an implemented row without a probe, and — when the pinned
jar is present, or always with `-require-jar` — on a baseline that no longer lists what the jar
contains. It runs under `make docs-counts`; the jar comparison runs in the scheduled
[oracle reproduction](../../.github/workflows/oracle-reproduction.yml).

### Statuses

Same vocabulary as [spec-compliance.md](spec-compliance.md), plus one value for what has not been adjudicated:

- **✅ faithful** — OpenSysML reports the violation with the pilot's meaning; the wording may differ (the *Our message* column says how).
- **⚠️ approximate** — OpenSysML reports the violation, but narrower, wider, or from a different layer than the pilot (a parser diagnostic where the pilot validates, a warning where it errors, a subset of the cases).
- **❌ not implemented** — a model the pilot rejects under this constraint passes OpenSysML silently. This includes constraints the pilot *grammar* enforces (a syntax error there) where OpenSysML's parser accepts the form and its passes report nothing.
- **⛔ deliberate** — OpenSysML declines the constraint on purpose (none at this recording).
- **🚧 known failure** — implemented and known wrong (none at this recording).
- **❔ unknown — no case and no identifiable pass yet** — no textual model was found that makes the *pilot* report the constraint, so there is nothing to map yet. Most of these are structural constraints on the abstract syntax that the textual notation cannot violate (an operator that is not `collect`, a result parameter that is not owned), or constraints whose violation is a syntax error in *both* tools before either validator runs. They are not claimed either way.

### Evidence

Every ✅ and ⚠️ row has a probe under `cmd/validation-census/testdata/probes/<constraint>.{kerml,sysml}`:
a minimal model that violates the constraint, headed by the constraint name and the severity and
message fragment OpenSysML must report for it. `go test ./cmd/validation-census` runs each probe
through the workspace and fails if the diagnostic is missing; `-check` fails if an implemented
row has no probe or a probe names a row that is not implemented. The probes were also run through
the pinned pilot validators (`build/pilot-sysml-validator/validate-sysml-batch`,
`build/pilot-kerml-validator/validate-kerml`) to confirm the pilot reports the constraint the
row names on the same model; where it reports a different one, the *Checks* column says so.

The **Negative case** column names the file in `cmd/pilot-reject/testdata/negative/` (see
[pilot-rejection.md](pilot-rejection.md)) that exercises the constraint against the pilot
oracle, or `none` where the corpus has no case yet. This census adds no corpus cases.

### How the names were read

`cmd/validation-census/jar.go` opens the pinned jar, reads the two validator classes
(`org/omg/kerml/xtext/validation/KerMLValidator.class`,
`org/omg/sysml/xtext/validation/SysMLValidator.class`), parses each class file's constant pool
and keeps every `CONSTANT_String` matching `^(in)?validate[A-Za-z]+_?$`. The Xtend sources
declare each constraint as a public constant whose string value is its name; the compiled
strings carry one trailing underscore for most SysML constraints (`validateActionUsageType_`)
and one KerML constant is misspelled `invalidateMetadataFeatureBody`, so the baseline records
the normalized name (the `in` prefix and one trailing underscore dropped) and, where they differ,
the raw compiled string. A name found in both classes has source `both`. The result is the same
217 names, with the same sources, as the hand-extracted list this census was commissioned
against. Four names are misspelled in the pilot itself and are recorded as spelled:
`validateNamespaceDistinguishablity`, `validateEndFeatureMembershpIsEnd`,
`validateViewDefinitionOnlyOnvViewRendering`, `validateStateSubactionMembershioOwningType`.

## Census

Constraints are grouped by language and listed in the baseline's (alphabetical) order.
*Checks* paraphrases the pilot's check and, where the pilot's message is not obvious from it,
quotes the message. *Implementation* is `internal/core/passes/<file>.go:<func>` or the
parser/resolver location. *Our message* is given only where OpenSysML's wording differs.

| Constraint | Language | Checks | Implementation | Our message | Negative case | Status |
|---|---|---|---|---|---|---|
| `validateAnnotationAnnotatedElementOwnership` | KerML | An annotating element's annotations either own or are owned by the annotated element consistently | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAnnotationAnnotatingElement` | KerML | An annotation has exactly one of an owned or an owning annotating element | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAssociationBinarySpecialization` | KerML | An association with more than two ends cannot specialize a binary association (`Cannot have more than two ends`) | internal/core/passes/constraint.go:checkConnectorEndRedefinition | `end c redefines no end of BinaryLink, which declares 2 end(s)` — the third end is rejected because it redefines nothing, rather than the end count itself | none | ⚠️ approximate |
| `validateAssociationEndTypes` | KerML | Each association end has exactly one type | internal/core/passes/w8c_association_end_types.go:AssociationEndTypesPass.Run | — | `xpect/p02-association-end-two-types.kerml` | ✅ faithful |
| `validateAssociationRelatedTypes` | KerML | An association has at least two related types (`Must have at least two related elements`) | — | — | none | ❌ not implemented |
| `validateAssociationStructureIntersection` | KerML | An association that is also a structure must be an association structure (`Must be an association structure`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateBehaviorSpecialization` | KerML | A behavior cannot specialize a structure | internal/core/passes/w11a_kerml_specialization.go:W11AKerMLSpecializationPass.Run | — | none | ✅ faithful |
| `validateBindingConnectorArgumentTypeConformance` | KerML | A binding connector's output feature conforms to its input feature (`Output feature must conform to input feature`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateBindingConnectorIsBinary` | KerML | A binding connector is binary | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateBindingConnectorTypeConformance` | KerML | The two bound features of a binding connector have conforming types (warning) | internal/core/passes/w9c_bound_feature_types.go:W9CBoundFeatureTypesPass.Run; internal/core/passes/typecheck_value.go:exprChecker.checkValueConformance | same wording for `bind`/`binding` (feature-chain ends included); the implicit binding of a feature value `part x : D = a.b.c;` is reported as the error `cannot bind a value of type C to a feature typed by D` where the pilot warns | none | ⚠️ approximate |
| `validateClassSpecialization` | both | A class (KerML) or an occurrence definition (SysML) cannot specialize a data type or association | internal/core/passes/w11a_kerml_specialization.go:W11AKerMLSpecializationPass.Run; internal/core/passes/typecheck.go:compatMessage | KerML: same wording; SysML: `item cannot specialize attributeDef (kind mismatch)` where the pilot says `Cannot specialize attribute definition` | none | ⚠️ approximate |
| `validateClassifierDefaultSupertype` | KerML | A classifier directly or indirectly specializes its kind's default supertype (`Must directly or indirectly specialize {supertype}`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateClassifierMultiplicityDomain` | KerML | A classifier's multiplicity has no featuring type | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateCollectExpressionOperator` | KerML | A collect expression's operator is `collect` | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateConnectorBinarySpecialization` | KerML | A connector with more than two ends cannot specialize a binary connector (`Cannot have more than two ends`) | — | — | none | ❌ not implemented |
| `validateConnectorRelatedFeatures` | KerML | A connector has at least two related features | internal/core/passes/w10b_related_elements.go:W10BRelatedElementsPass.Run | same wording; a parenthesized KerML end list with one end is a parse error (`expected at least two connector ends in a parenthesized end list`) | `xpect/p20-connection-with-one-end.sysml` | ✅ faithful |
| `validateConnectorTypeFeaturing` | KerML | A connector's related features are accessible from its featuring type | internal/core/passes/w8d_connector_featuring.go:W8DConnectorFeaturingPass.Run | — | none | ✅ faithful |
| `validateConstructorExpressionNoDuplicateFeatureRedefinition` | KerML | A constructor expression binds each feature of the instantiated type at most once (`Feature already bound`) | — | — | none | ❌ not implemented |
| `validateConstructorExpressionOwnedFeatures` | KerML | Each argument of a constructor expression corresponds to one feature of the instantiated type | — | — | none | ❌ not implemented |
| `validateCrossSubsettingCrossedFeature` | KerML | A cross subsetting chains through an opposite end feature | internal/core/passes/w10b_cross_features.go:checkW10BCrossFeatures | — | none | ✅ faithful |
| `validateCrossSubsettingCrossingFeature` | KerML | A cross subsetting is owned by one of two or more end features | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateDataTypeSpecialization` | both | A data type (KerML) or an attribute definition (SysML) cannot specialize a class or association | internal/core/passes/w11a_kerml_specialization.go:W11AKerMLSpecializationPass.Run; internal/core/passes/typecheck.go:compatMessage | KerML: same wording; SysML: `attribute cannot specialize itemDef (kind mismatch)` where the pilot says `Cannot specialize item definition` | none | ⚠️ approximate |
| `validateElementFilterMembershipIsBoolean` | KerML | A filter condition has a Boolean result | internal/core/passes/filter.go:filterChecker.check | — | `xpect/p12-filter-integer-condition.sysml`, `xpect/p21-filter-constructor-condition.sysml` | ✅ faithful |
| `validateElementFilterMembershipIsModelLevelEvaluable` | KerML | A filter condition is model-level evaluable | internal/core/passes/filter.go:filterChecker.check | — | `xpect/p22-filter-not-model-level-evaluable.sysml` | ✅ faithful |
| `validateElementIsImpliedIncluded` | KerML | An element has no implied relationships included (not expressible in the textual notation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateEndFeatureMembershpIsEnd` | KerML | The feature of an end feature membership is an end feature (the pilot's constant is spelled `Membershp`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateExpressionResultExpressionMembership` | KerML | An expression owns or inherits at most one result expression | internal/core/passes/w8c_result_expression.go:ResultExpressionPass.Run | — | none | ✅ faithful |
| `validateExpressionResultParameterMembership` | KerML | An expression owns at most one return parameter | internal/core/passes/w10b_structural.go:W10BStructuralPass.Run | — | none | ✅ faithful |
| `validateFeatureChainExpressionFeatureConformance` | KerML | The target feature of a feature-chain expression is a feature of the source's type (`Must be a valid feature`); a probe fails name resolution first in both tools | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureChainExpressionOperator` | KerML | A feature-chain expression's operator is `.` | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureChainingFeatureConformance` | KerML | Each chaining feature is featured within the previous one (`Must be a valid feature`); a probe fails name resolution first in both tools | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureChainingFeatureNotOne` | KerML | A feature chain has more than one chaining feature | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | `xpect/p05-single-chaining-feature.kerml` | ✅ faithful |
| `validateFeatureChainingFeaturesNotSelf` | KerML | A feature is not one of its own chaining features; a probe fails name resolution first in both tools | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureConstantIsVariable` | KerML | Only a variable feature can be constant | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureCrossFeatureSpecialization` | KerML | A cross feature specializes the cross features of the ends it redefines | internal/core/passes/w10b_cross_features.go:checkW10BCrossFeatures | — | none | ✅ faithful |
| `validateFeatureCrossFeatureType` | KerML | A cross feature has the same type as its feature | internal/core/passes/w10b_cross_features.go:checkW10BCrossFeatures | — | none | ✅ faithful |
| `validateFeatureEndFeatureMultiplicity` | KerML | An end feature has multiplicity 1 (warning `End feature must have multiplicity 1`) | — | — | none | ❌ not implemented |
| `validateFeatureEndIsConstant` | KerML | An end feature is constant (the pilot grammar only admits `const end`, so no textual violation was found) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureEndNoDirection` | KerML | An end feature has no direction (the pilot grammar admits no direction on an end feature) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureEndNotDerivedAbstractCompositeOrPortion` | KerML | An end feature is not derived, abstract, composite or portion (the pilot grammar admits none of these on an end feature) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureHasType` | KerML | A feature has at least one type (`Features must have at least one type`); the pilot only reported it on a model with a syntax error | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureIsVariable` | KerML | A variable feature is owned by an occurrence type | internal/core/passes/w8c_variable_feature.go:VariableFeaturePass.Run | — | `xpect/p03-variable-in-datatype.kerml` | ✅ faithful |
| `validateFeatureMultiplicityDomain` | KerML | A feature's multiplicity has the same featuring types as the feature | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureOwnedCrossSubsetting` | KerML | A feature owns at most one cross subsetting; the pinned pilot crashes on the probe (see `omg-issues.md`), OpenSysML accepts it silently | — | — | none | ❌ not implemented |
| `validateFeatureOwnedReferenceSubsetting` | KerML | A feature owns at most one reference subsetting | internal/core/passes/w8c_reference_subsetting.go:ReferenceSubsettingPass.Run | — | none | ✅ faithful |
| `validateFeaturePortionNotVariable` | KerML | A portion feature is not variable | internal/core/passes/w8c_variable_feature.go:VariableFeaturePass.Run | — | none | ✅ faithful |
| `validateFeatureReferenceExpressionReferentIsFeature` | KerML | The referent of a feature reference expression is a feature | internal/core/passes/w8c_feature_reference.go:FeatureReferencePass.Run | — | none | ✅ faithful |
| `validateFeatureReferenceExpressionResult` | KerML | A feature reference expression owns its result parameter | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFeatureValueIsInitial` | KerML | A feature with an initial value is variable (`Initialized feature must be variable`) | — | — | none | ❌ not implemented |
| `validateFeatureValueOverriding` | KerML | A feature value does not override a binding feature value inherited from a redefined feature (`Cannot override a binding feature value`) | — | — | none | ❌ not implemented |
| `validateFlowEndImplicitSubsetting` | KerML | A flow end whose owned feature is identified implicitly should use dot notation (warning `Flow ends should use dot notation`); no model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFlowEndIsEnd` | KerML | A flow end is an end feature | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFlowEndNestedFeature` | KerML | A flow end has exactly one nested input or output feature; no model made the pilot report it (it reports `validateFlowEndSubsetting` instead) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFlowEndOwningType` | KerML | A flow end is owned by a flow | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFlowEndSubsetting` | KerML | A flow end names the feature the payload flows from or to (`Cannot identify flow end (use dot notation)`) | internal/core/passes/w8d_flow_end.go:W8DFlowEndPass.Run | same wording; a flow end without dot notation is additionally reported by `internal/core/passes/constraint.go:checkFlowEndSubsetting` | none | ✅ faithful |
| `validateFlowItemFeature` | KerML | A flow has at most one item feature | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFunctionResultExpressionMembership` | KerML | A function owns or inherits at most one result expression | internal/core/passes/w8c_result_expression.go:ResultExpressionPass.Run | — | none | ✅ faithful |
| `validateFunctionResultParameterMembership` | KerML | A function owns at most one return parameter | internal/core/passes/w10b_structural.go:W10BStructuralPass.Run | — | none | ✅ faithful |
| `validateImportTopLevelVisibility` | KerML | A top-level import is private | internal/core/passes/w8c_import_visibility.go:TopLevelImportPass.Run | — | `xpect/p01-public-root-import.kerml` | ✅ faithful |
| `validateIndexExpressionOperator` | KerML | An index expression's operator is `#` | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateInstantiationExpressionInstantiatedType` | KerML | An instantiation expression names an instantiated type; a probe fails name resolution first in both tools | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateInstantiationExpressionResult` | KerML | An instantiation expression owns its result parameter | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateInvocationExpressionInstantiatedType` | KerML | An invocation expression invokes a behavior or a behavioral feature | internal/core/passes/typecheck_expr.go:inferInvocation | — | none | ✅ faithful |
| `validateInvocationExpressionNoDuplicateParameterRedefinition` | KerML | An invocation expression binds each parameter at most once (`Parameter already bound`) | — | — | none | ❌ not implemented |
| `validateInvocationExpressionOwnedFeatures` | KerML | Each owned feature of an invocation expression is an in parameter (`Must be an in parameter`); no model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateInvocationExpressionParameterRedefinition` | KerML | Each named argument of an invocation redefines one input parameter of the invoked type (`Must correspond to one input parameter of the invoked type`) | internal/core/passes/typecheck_expr.go:checkArguments | `F has no parameter named "y"` | none | ⚠️ approximate |
| `validateLibraryPackageNotStandard` | KerML | A user library package is not marked standard (warning) | internal/core/passes/w9c_owned_name_and_library.go:W9CUserStandardLibraryPass.Run | — | none | ✅ faithful |
| `validateMetadataFeatureAnnotatedElement` | KerML | A metadata feature's annotated element conforms to the metaclass's `annotatedElement` type (`Cannot annotate {kind}`) | internal/core/passes/w8c_metadata_annotation.go:MetadataAnnotationPass.Run | — | none | ✅ faithful |
| `validateMetadataFeatureBody` | KerML | A feature in a metadata feature's body redefines a feature of its owning type (the pilot's constant is spelled `invalidateMetadataFeatureBody`) | internal/core/passes/w8c_metadata_annotation.go:MetadataAnnotationPass.Run; internal/core/passes/w8d_metadata_usage.go:W8DMetadataUsagePass.Run | — | `xpect/p23-metadata-body-feature-not-redefining.sysml` | ✅ faithful |
| `validateMetadataFeatureMetadata` | KerML | A metadata feature is typed by exactly one metaclass (the grammar admits one type) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateMetadataFeatureMetadataNotAbstract` | KerML | A metadata feature's metaclass is not abstract | internal/core/passes/w8c_metadata_type.go:MetadataTypePass.Run | — | `xpect/p24-metadata-abstract-type.sysml` | ✅ faithful |
| `validateMultiplicityRangeBounds` | KerML | A multiplicity range's bound expressions are its first two owned members (structural; not expressible in text) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateMultiplicityRangeResultTypes` | KerML | Multiplicity bounds have Natural values | internal/core/passes/w8c_multiplicity_bounds.go:MultiplicityBoundsPass.Run | — | none | ✅ faithful |
| `validateNamespaceDistinguishablity` | KerML | Owned member names of a namespace are distinguishable (warning; the pilot's constant is spelled `Distinguishablity`) | internal/core/resolve/distinguishability.go:checkOwnedNames | — | none | ✅ faithful |
| `validateOperatorExpressionBracketOperator` | KerML | `[` as an operator should be `#(...)` indexing (warning `Use #(...) for indexing`) | — | — | none | ❌ not implemented |
| `validateOperatorExpressionCastConformance` | KerML | A cast (`as`) argument has a type conforming to the target (warning `Cast argument should have conforming types`) | — | — | none | ❌ not implemented |
| `validateOwnedDifferencingNotOne` | KerML | A type does not difference exactly one type | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | none | ✅ faithful |
| `validateOwnedIntersectingNotOne` | KerML | A type does not intersect exactly one type | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | none | ✅ faithful |
| `validateOwnedUnioningNotOne` | KerML | A type does not union exactly one type | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | none | ✅ faithful |
| `validateParameterMembershipDirection` | KerML | A parameter has the direction its membership requires | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateParameterMembershipOwningType` | KerML | A parameter membership is owned by a behavior or a step (`Parameter membership not allowed`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateRedefinitionDirectionConformance` | KerML | A redefining feature has a direction compatible with the redefined feature | internal/core/passes/w8b_redefinition_conformance.go:RedefinitionDirectionPass.Run | — | `xpect/p31-redefinition-incompatible-direction.sysml` | ✅ faithful |
| `validateRedefinitionEndConformance` | KerML | A feature redefining an end feature is an end feature | internal/core/passes/w10b_redefinition.go:checkW10BRedefinition | — | none | ✅ faithful |
| `validateRedefinitionFeaturingTypes` | KerML | A redefined feature is not package-level and the redefining and redefined features have different featuring types | internal/core/passes/w10b_redefinition.go:checkW10BRedefinition | — | `xpect/p28-package-level-feature-redefined.sysml`, `xpect/p32-redefinition-same-featuring-type.sysml` | ✅ faithful |
| `validateRedefinitionMultiplicityConformance` | KerML | A redefining feature does not weaken the redefined feature's lower bound (warning) | internal/core/passes/multiplicity_conformance.go:constraintChecker.checkMultiplicityConformance | — | none | ✅ faithful |
| `validateResultExpressionMembershipOwningType` | KerML | A result expression membership is owned by a function or expression (the grammar admits a result expression nowhere else) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateReturnParameterMembershipOwningType` | KerML | A return parameter membership is owned by a function or expression (`Return parameter membership not allowed`); the pilot grammar rejects `return` elsewhere, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateSelectExpressionOperator` | KerML | A select expression's operator is `select` | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateSpecializationSpecificNotConjugated` | KerML | A conjugated type is not the specific type of a specialization | internal/core/passes/w11e_conjugated_specialization.go:W11EConjugatedSpecializationPass.Run | same wording; the pilot grammar cannot express a type that both conjugates and specializes, OpenSysML's parser accepts the form and reports it | none | ⚠️ approximate |
| `validateStructureSpecialization` | KerML | A structure cannot specialize a behavior | internal/core/passes/w11a_kerml_specialization.go:W11AKerMLSpecializationPass.Run | — | none | ✅ faithful |
| `validateSubsettingConstantConformance` | KerML | A feature subsetting a constant feature is constant | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateSubsettingFeaturingTypes` | KerML | A subsetted feature is accessible from the subsetting feature's featuring types | internal/core/passes/subsetting_featuring.go:checkSubsettingFeaturingTypes | — | none | ✅ faithful |
| `validateSubsettingMultiplicityConformance` | KerML | A subsetting feature does not widen the subsetted feature's upper bound (warning) | internal/core/passes/multiplicity_conformance.go:constraintChecker.checkMultiplicityConformance | — | none | ✅ faithful |
| `validateSubsettingPortionConformance` | KerML | A feature subsetting a portion feature is a portion | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateSubsettingUniquenessConformance` | KerML | A nonunique feature does not subset or redefine a unique feature | internal/core/passes/w8b_redefinition_conformance.go:RedefinitionConformancePass.Run | — | `xpect/p04-nonunique-subsets-unique.kerml` | ✅ faithful |
| `validateTypeAtMostOneConjugator` | KerML | A type has at most one conjugator; the pilot grammar rejects a second `~`, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateTypeDifferencingTypesNotSelf` | KerML | A type does not difference itself | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | none | ✅ faithful |
| `validateTypeIntersectingTypesNotSelf` | KerML | A type does not intersect itself | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | none | ✅ faithful |
| `validateTypeOwnedMultiplicity` | KerML | A type owns at most one multiplicity | internal/core/passes/at_most_one_member.go:checkAtMostOneMultiplicity | — | none | ✅ faithful |
| `validateTypeUnioningTypesNotSelf` | KerML | A type does not union itself | internal/core/passes/w8c_type_relationships.go:TypeRelationshipsPass.Run | — | none | ✅ faithful |
| `validateAcceptActionUsageParameters` | SysML | An accept action has a payload parameter (the grammar requires one; a missing payload is a syntax error in both tools) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateActionUsageType` | SysML | An action is typed by action definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateActorMembershipOwningType` | SysML | Only requirements and cases have actors; the pilot grammar rejects `actor` elsewhere, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateAllocationUsageType` | SysML | An allocation is typed by allocation definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateAnalysisCaseUsageType` | SysML | An analysis case is typed by one analysis case definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateAssertConstraintUsageReference` | SysML | An assert constraint usage references a constraint (`Must reference a constraint.`) | — | — | none | ❌ not implemented |
| `validateAssignmentActionUsageArguments` | SysML | An assignment action has two arguments (the grammar requires both; a syntax error in both tools otherwise) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAssignmentActionUsageReferent` | SysML | An assignment action has a referent; a probe fails name resolution first in both tools | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAssignmentActionUsageReferentIsTimeVarying` | SysML | The referent of an assignment action is time-varying | internal/core/passes/w8d_assignment_referent.go:AssignmentReferentPass.Run | — | none | ✅ faithful |
| `validateAttributeDefinitionFeatures` | SysML | Features of an attribute definition are referential; the pilot grammar rejects a composite usage inside an attribute definition | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAttributeUsageEnumerationType` | SysML | An attribute typed by an enumeration definition has no other type | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateAttributeUsageFeatures` | SysML | Features of an attribute usage are referential; the pilot grammar rejects a composite usage inside an attribute usage | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAttributeUsageIsReferential` | SysML | An attribute usage is referential (the grammar admits no composite attribute) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateAttributeUsageType` | SysML | An attribute is typed by attribute definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | `xpect/p15-attribute-typed-by-part-def.sysml` | ✅ faithful |
| `validateCalculationUsageType` | SysML | A calculation is typed by one calculation definition | internal/core/passes/one_type.go:checkOneType | — | `xpect/p17-calc-two-return-parameters.sysml` | ✅ faithful |
| `validateCaseDefinitionOnlyOneObjective` | SysML | A case definition has at most one objective | internal/core/passes/at_most_one_member.go:checkAtMostOneObjective | — | none | ✅ faithful |
| `validateCaseDefinitionOnlyOneSubject` | SysML | A case definition has at most one subject | internal/core/passes/at_most_one_member.go:checkAtMostOneMember | — | none | ✅ faithful |
| `validateCaseDefinitionSubjectParameterPosition` | SysML | A case definition's subject is its first parameter | internal/core/passes/at_most_one_member.go:checkSubjectParameterPosition | — | none | ✅ faithful |
| `validateCaseUsageOnlyOneObjective` | SysML | A case usage has at most one objective | internal/core/passes/at_most_one_member.go:checkAtMostOneObjective | — | none | ✅ faithful |
| `validateCaseUsageOnlyOneSubject` | SysML | A case usage has at most one subject | internal/core/passes/at_most_one_member.go:checkAtMostOneMember | — | none | ✅ faithful |
| `validateCaseUsageSubjectParameterPosition` | SysML | A case usage's subject is its first parameter | internal/core/passes/at_most_one_member.go:checkSubjectParameterPosition | — | none | ✅ faithful |
| `validateCaseUsageType` | SysML | A case is typed by one case definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateConjugatedPortDefinitionConjugatedPortDefinition` | SysML | A conjugated port definition has no conjugated port definition of its own (implicit element; not expressible in text) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateConnectionUsageType` | SysML | A connection is typed by connection definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateControlNodeIncomingSuccessions` | SysML | Incoming successions of a control node have target multiplicity 1; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateControlNodeOutgoingSuccessions` | SysML | Outgoing successions of a control node have source multiplicity 1; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateControlNodeOwningType` | SysML | A control node is owned by an action definition or usage (the grammar admits control nodes nowhere else; a syntax error in both tools) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateDecisionNodeIncomingSuccessions` | SysML | A decision node has at most one incoming succession; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateDecisionNodeOutgoingSuccessions` | SysML | Outgoing successions of a decision node have target multiplicity 0..1; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateDefinitionVariationIsAbstract` | SysML | A variation definition is abstract (implied by `variation`; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateDefinitionVariationMembership` | SysML | An owned usage of a variation definition is a variant | internal/core/passes/w8d_variability.go:W8DVariabilityPass.Run | — | `xpect/p09-variation-member-not-variant.sysml` | ✅ faithful |
| `validateDefinitionVariationSpecialization` | SysML | A variation definition does not specialize another variation | internal/core/passes/w8d_variability.go:W8DVariabilityPass.Run | same wording; reported for `variation` definitions only, an enumeration definition specializing another is not reported | `xpect/p10-variation-specializes-variation.sysml` | ⚠️ approximate |
| `validateEnumerationDefinitionIsVariation` | SysML | An enumeration definition is a variation (implied by `enum def`; the pilot reports `validateDefinitionVariationSpecialization` for the enum case instead) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateEnumerationUsageType` | SysML | An enumeration usage is typed by one enumeration definition | internal/core/passes/one_type.go:checkOneType | — | `xpect/p18-enum-two-types.sysml` | ✅ faithful |
| `validateEventOccurrenceUsageIsReference` | SysML | An event occurrence usage is referential (the grammar admits no composite `event`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateEventOccurrenceUsageReferent` | SysML | An event occurrence usage references an occurrence | internal/core/passes/w8d_occurrence_typing.go:W8DOccurrenceTypingPass.Run | — | none | ✅ faithful |
| `validateExhibitStateUsageReference` | SysML | An exhibit state usage references a state | internal/core/passes/w11a_usage_typing.go:W11AUsageTypingPass.Run | — | none | ✅ faithful |
| `validateExposeIsImportAll` | SysML | An expose imports all (implied by the grammar; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateExposeOwningNamespace` | SysML | Only view usages expose elements | internal/core/passes/expose.go:checkExposeOwners | `expose is only allowed in a view usage body`; the pilot grammar rejects `expose` elsewhere as a syntax error | none | ⚠️ approximate |
| `validateFlowDefinitionConnectionEnds` | SysML | A flow definition has at most two ends | internal/core/passes/w10b_ends.go:W10BEndKindPass.Run | — | none | ✅ faithful |
| `validateFlowUsageType` | SysML | A flow is typed by flow definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateForLoopActionUsageLoopVariable` | SysML | A for loop has a loop variable (the grammar requires one; a syntax error in both tools otherwise) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateForLoopActionUsageParameters` | SysML | A for loop has two parameters (implied by the grammar; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateForkNodeIncomingSuccessions` | SysML | A fork node has at most one incoming succession; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateFramedConcernMembershipConstraintKind` | SysML | A framed concern is a required constraint (implied by the `frame` keyword; a syntax error in both tools otherwise) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateIfActionUsageParameters` | SysML | An if action has at least two parameters (implied by the grammar; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateIncludeUseCaseUsageReference` | SysML | An include use case usage references a use case | internal/core/passes/w11a_usage_typing.go:W11AUsageTypingPass.Run | — | none | ✅ faithful |
| `validateInterfaceDefinitionEnd` | SysML | An interface definition's ends are ports | internal/core/passes/w10b_ends.go:W10BEndKindPass.Run | — | none | ✅ faithful |
| `validateInterfaceUsageEnd` | SysML | An interface usage's ends are ports | internal/core/passes/w10b_ends.go:W10BEndKindPass.Run | — | `xpect/p27-interface-end-not-port.sysml` | ✅ faithful |
| `validateInterfaceUsageType` | SysML | An interface is typed by interface definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateItemUsageType` | SysML | An item is typed by item definitions (`An item must be typed by item definitions.`); the pilot reports `validateOccurrenceUsageType` instead on every probe | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateJoinNodeOutgoingSuccessions` | SysML | A join node has at most one outgoing succession; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateMergeNodeIncomingSuccessions` | SysML | Incoming successions of a merge node have source multiplicity 0..1; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateMergeNodeOutgoingSuccessions` | SysML | A merge node has at most one outgoing succession; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateMetadataUsageType` | SysML | A metadata usage is typed by one metadata definition (`A metadata usage must be typed by one metadata definition.`); OpenSysML reports only the abstract case (`Must have a concrete type`, `validateMetadataFeatureMetadataNotAbstract`) | — | — | none | ❌ not implemented |
| `validateObjectiveMembershipIsComposite` | SysML | An objective is composite (the grammar admits no `ref objective`; a syntax error in both tools) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateObjectiveMembershipOwningType` | SysML | Only cases have objectives; the pilot grammar rejects `objective` elsewhere, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateOccurrenceUsageIndividualDefinition` | SysML | An occurrence usage is typed by at most one individual definition | internal/core/passes/w10b_individual_portion.go:W10BIndividualTypingPass.Run | — | `xpect/p25-two-individual-definitions.sysml` | ✅ faithful |
| `validateOccurrenceUsageIndividualUsage` | SysML | An individual usage is typed by one individual definition | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | `xpect/p33-individual-typed-by-plain-def.sysml` | ✅ faithful |
| `validateOccurrenceUsageIsPortion` | SysML | A portion usage is owned by an occurrence definition or usage | internal/core/passes/w10b_individual_portion.go:W10BPortionOwnerPass.Run | — | none | ✅ faithful |
| `validateOccurrenceUsageType` | SysML | An occurrence, item or part is typed by occurrence definitions | internal/core/passes/w8d_occurrence_typing.go:W8DOccurrenceTypingPass.Run | — | `xpect/p16-part-typed-by-attribute-def.sysml` | ✅ faithful |
| `validateOperatorExpressionQuantity` | SysML | The right operand of `[` on a quantity is a measurement reference (warning `Should be a measurement reference (unit).`) | — | — | none | ❌ not implemented |
| `validatePartUsagePartDefinition` | SysML | A part is typed by at least one part definition; no textual model made the pilot report it | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validatePartUsageType` | SysML | A part is typed by item definitions (`A part must be typed by item definitions.`); the pilot reports `validateOccurrenceUsageType` instead on every probe | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validatePerformActionUsageReference` | SysML | A perform action usage references an action | internal/core/passes/w11a_usage_typing.go:W11AUsageTypingPass.Run | — | none | ✅ faithful |
| `validatePortDefinitionConjugatedPortDefinition` | SysML | A port definition has exactly one conjugated port definition (implicit element; not expressible in text) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validatePortDefinitionOwnedUsagesNotComposite` | SysML | Owned usages of a port definition other than ports are referential | internal/core/passes/w10b_structural.go:W10BStructuralPass.Run | — | `xpect/p26-port-def-nonreferential-usage.sysml` | ✅ faithful |
| `validatePortUsageIsReference` | SysML | A port usage is referential (the grammar admits no composite `port`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validatePortUsageNestedUsagesNotComposite` | SysML | Nested usages of a port usage other than ports are referential | internal/core/passes/w10b_structural.go:W10BStructuralPass.Run | — | none | ✅ faithful |
| `validatePortUsageType` | SysML | A port is typed by port definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateReferenceUsageIsReference` | SysML | A reference usage is referential (implied by `ref`; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateRenderingUsageType` | SysML | A rendering is typed by one rendering definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateRequirementConstraintMembershipIsComposite` | SysML | A requirement constraint is composite (the grammar admits no `ref require`; a syntax error in both tools) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateRequirementConstraintMembershipOwningType` | SysML | Only requirements have assumed or required constraints | internal/core/passes/nonstandard_notation.go:NonstandardNotationPass.Run | warning `require outside a requirement body is an OpenSysML extension with no SysML v2 production` where the pilot grammar rejects it as a syntax error | none | ⚠️ approximate |
| `validateRequirementDefinitionOnlyOneSubject` | SysML | A requirement definition has at most one subject | internal/core/passes/at_most_one_member.go:checkAtMostOneMember | — | `xpect/p14-requirement-two-subjects.sysml` | ✅ faithful |
| `validateRequirementDefinitionSubjectParameterPosition` | SysML | A requirement definition's subject is its first parameter | internal/core/passes/at_most_one_member.go:checkSubjectParameterPosition | — | none | ✅ faithful |
| `validateRequirementUsageOnlyOneSubject` | SysML | A requirement usage has at most one subject | internal/core/passes/at_most_one_member.go:checkAtMostOneMember | — | none | ✅ faithful |
| `validateRequirementUsageSubjectParameterPosition` | SysML | A requirement usage's subject is its first parameter | internal/core/passes/at_most_one_member.go:checkSubjectParameterPosition | — | none | ✅ faithful |
| `validateRequirementUsageType` | SysML | A requirement is typed by one requirement definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateRequirementVerificationMembershipKind` | SysML | A requirement verification is a required constraint (implied by the `verify` keyword; a syntax error in both tools otherwise) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateRequirementVerificationMembershipOwningType` | SysML | A requirement verification is in the objective of a verification case | internal/core/passes/w8d_verification.go:W8DVerificationPass.Run | — | `xpect/p29-verify-outside-objective.sysml` | ✅ faithful |
| `validateSatisfyRequirementUsageReference` | SysML | A satisfy requirement usage references a requirement (`Must reference a requirement.`) | internal/core/passes/typecheck.go:compatMessage | `satisfy target must be a requirement usage, found partUsage` | none | ⚠️ approximate |
| `validateSendActionUsagePayloadArgument` | SysML | A send action used as a state entry/do/exit subaction or transition effect has a payload argument (`A send action must have a payload.`; the pilot grammar already demands the payload there, while `send to x` inside an action body is accepted by both tools; OpenSysML parses `entry send to d;` and reports nothing) | — | — | none | ❌ not implemented |
| `validateSendActionUsageReceiver` | SysML | Sending to a port should use `via` rather than `to` (warning `Sending to a port should generally use "via" instead of "to".`) | — | — | none | ❌ not implemented |
| `validateStakeholderMembershipOwningType` | SysML | Only requirements have stakeholders; the pilot grammar rejects `stakeholder` elsewhere, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateStateDefinitionParallelSubactions` | SysML | A parallel state definition has no successions or transitions | internal/core/passes/state_transition.go:transitionChecker.checkParallelStates | — | none | ✅ faithful |
| `validateStateDefinitionSubactionKind` | SysML | A state definition has at most one entry, do and exit action each | internal/core/passes/w10b_structural.go:W10BStructuralPass.Run | — | none | ✅ faithful |
| `validateStateSubactionMembershioOwningType` | SysML | Only a state has entry, do or exit actions (the pilot's constant is spelled `Membershio`); the pilot grammar rejects them elsewhere, OpenSysML's parser accepts them and reports nothing | — | — | none | ❌ not implemented |
| `validateStateUsageParallelSubactions` | SysML | A parallel state usage has no successions or transitions | internal/core/passes/state_transition.go:transitionChecker.checkParallelStates | — | `xpect/p19-parallel-state-with-transition.sysml` | ✅ faithful |
| `validateStateUsageSubactionKind` | SysML | A state usage has at most one entry, do and exit action each | internal/core/passes/w10b_structural.go:W10BStructuralPass.Run | — | `xpect/p13-state-two-entry-actions.sysml` | ✅ faithful |
| `validateStateUsageType` | SysML | A state is typed by state definitions | internal/core/passes/typecheck.go:compatibleTyping (messages in w10b_usage_typing.go:pilotTypingMessage) | — | none | ✅ faithful |
| `validateSubjectMembershipOwningType` | SysML | Only requirements and cases have subjects; the pilot grammar rejects `subject` elsewhere, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateTransitionFeatureMembershipEffectAction` | SysML | A transition effect is an action; the pilot reports `validatePerformActionUsageReference` instead on a non-action `do` | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateTransitionFeatureMembershipGuardExpression` | SysML | A transition guard is a Boolean expression | internal/core/passes/typecheck_expr.go:exprChecker.checkBoolean | `transition guard must be Boolean, found String`; the pilot did not report the probe (`if "yes"`), so OpenSysML is stricter here | none | ⚠️ approximate |
| `validateTransitionFeatureMembershipOwningType` | SysML | A transition feature membership is owned by a transition (structural; not expressible in text) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateTransitionFeatureMembershipTriggerAction` | SysML | A transition trigger is an accept action; the pilot reports `validateUsageType` instead when `accept` names a non-definition | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateTransitionUsageParameters` | SysML | A transition usage has its parameters (implied by the grammar; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateTransitionUsageSuccession` | SysML | A transition owns a succession to its target (implied by the grammar; a syntax error in both tools otherwise) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateTransitionUsageTriggerActions` | SysML | A transition with an accepter has a state as its source | internal/core/passes/state_transition.go:transitionChecker.checkAccepterSource | — | `xpect/p34-accepter-source-not-state.sysml` | ✅ faithful |
| `validateTriggerInvocationActionAfterArgument` | SysML | An `after` trigger argument is a DurationValue | — | — | none | ❌ not implemented |
| `validateTriggerInvocationActionAtArgument` | SysML | An `at` trigger argument is a TimeInstantValue | — | — | none | ❌ not implemented |
| `validateTriggerInvocationActionWhenArgument` | SysML | A `when` trigger argument is Boolean | — | — | none | ❌ not implemented |
| `validateUsageIsReferential` | SysML | A usage owned by a non-occurrence is referential (derived; no textual violation found) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateUsageType` | SysML | A usage is typed by definitions (`A usage must be typed by definitions.`); the pilot reports the kind-specific typing constraint instead on every probe | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateUsageVariationIsAbstract` | SysML | A variation usage is abstract (implied by `variation`; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateUsageVariationMembership` | SysML | An owned usage of a variation usage is a variant | internal/core/passes/w8d_variability.go:W8DVariabilityPass.Run | — | none | ✅ faithful |
| `validateUsageVariationSpecialization` | SysML | A variation usage does not specialize another variation | internal/core/passes/w8d_variability.go:W8DVariabilityPass.Run | — | none | ✅ faithful |
| `validateUseCaseUsageReference` | SysML | A use case usage that references another references a use case (only `include` can reference; see `validateIncludeUseCaseUsageReference`) | — | — | none | ❔ unknown — no case and no identifiable pass yet |
| `validateUseCaseUsageType` | SysML | A use case is typed by one use case definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateVariationMembershipOwningNamespace` | SysML | A variant is an owned member of a variation | internal/core/passes/w8d_variability.go:W8DVariabilityPass.Run | — | `xpect/p08-variant-outside-variation.sysml` | ✅ faithful |
| `validateVerificationCaseUsageType` | SysML | A verification case is typed by one verification case definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateViewDefinitionOnlyOnvViewRendering` | SysML | A view definition has at most one view rendering (the pilot's constant is spelled `Onv`) | internal/core/passes/w8d_view_rendering.go:W8DViewRenderingPass.Run | — | none | ✅ faithful |
| `validateViewRenderingMembershipOwningType` | SysML | Only views have view renderings; the pilot grammar rejects `render` elsewhere, OpenSysML's parser accepts it and reports nothing | — | — | none | ❌ not implemented |
| `validateViewUsageOnlyOneRendering` | SysML | A view usage has at most one view rendering | internal/core/passes/w8d_view_rendering.go:W8DViewRenderingPass.Run | — | `xpect/p30-two-view-renderings.sysml` | ✅ faithful |
| `validateViewUsageType` | SysML | A view is typed by one view definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateViewpointUsageType` | SysML | A viewpoint is typed by one viewpoint definition | internal/core/passes/one_type.go:checkOneType | — | none | ✅ faithful |
| `validateWhileLoopActionUsageParameters` | SysML | A while loop has at least two parameters (implied by the grammar; no textual violation) | — | — | none | ❔ unknown — no case and no identifiable pass yet |

## Out of scope for this census

Rows are recorded as `main` stands at the recording date. Constraints being implemented in
parallel (`validateFeatureValueOverriding`, the enumeration cases of
`validateDefinitionVariationSpecialization`, the `validateTriggerInvocationAction*Argument`
trio, `validateSendActionUsagePayloadArgument`/`validateSendActionUsageReceiver`, and the
control-node succession constraints) keep the status the evidence supports today; when their
passes land, the row's status moves and `-update` is not needed — the baseline's statuses are
edited by hand, the names are not. The feature-chain value bindings of
`validateBindingConnectorTypeConformance` landed on `main` before this recording, so its row
already describes them.
