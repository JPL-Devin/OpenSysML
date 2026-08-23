# Wave 11D — metadata annotations and model-level evaluability

The slice owns the Xpect rows about what a metadata annotation may say and about which expressions
the model alone can evaluate: `MetadataTests_MetadataFeature_invalid.kerml.xt`,
`MetadataUsage_Invalid.sysml.xt`, the `SemanticMetadata` files, and the `Must be model-level
evaluable`, `Must have a concrete type` and `Must have a Boolean result` families.

Measured on `main` at `07dc713c` and on the branch, with `go run ./cmd/pilot-xpect`,
`go run ./cmd/pilot-diff` and `go run ./cmd/pilot-reject`.

| Oracle | `main` | Branch |
|---|---|---|
| Xpect | 1326 expectations: 1172 agree (239 wording-only), 154 disagree | 1323 expectations: 1187 agree (239 wording-only), 136 disagree |
| Differential | 353 files, 312 fully agreeing; 25 agreed, 139 only ours, 73 only the pilot's | 353 files, 313 fully agreeing; 25 agreed, 138 only ours, 73 only the pilot's |
| Rejection | 120 cases: 114 both reject, 6 only the pilot rejects, 0 only we reject | 120 cases: 115 both reject, 5 only the pilot rejects, 0 only we reject |

Row keys were compared, not totals, and against a fresh run of each oracle on `main` rather than the
committed baselines, which the wave owner rebaselines: 18 Xpect rows became agreements and **no row
became a disagreement**, in any of the three. Three of the 18 are not a behaviour change and are
reported as such in [the expectation-count correction](#the-expectation-count-correction) below. The
differential moved by exactly one row (`Simple Tests/MetadataTest.kerml`:53) and the rejection corpus
by exactly one case (`xpect/p11-metadata-body-not-evaluable.sysml`, now rejected by both).

Of the 23 rows the slice owns, **14 closed** and 9 stand: the four span rows and the four
concrete-type rows below, and `SemanticMetadata_valid.sysml.xt`:53, which is a grammar gap.

---

## What moved

- **`null` and `new T(…)` in a filter condition.** Both are model-level evaluable (KerML 1.0 §7.4.9):
  `null` is the empty sequence, and a constructor is decided by the model when its arguments are. A
  condition built from a constructor is therefore reported for yielding an instance rather than a
  truth value — `Must have a Boolean result`, not `Must be model-level evaluable`, which is the
  verdict the two `filter new A(null, 1, "", false);` rows declare.
- **One diagnostic per fault, on the membership.** `ElementFilterMembership`'s two constraints
  (KerML 1.0 §8.2.4) are stated on the membership carrying the condition, so a condition with several
  faulty operands draws each fault once, at the membership, rather than once per operand.
- **Model-level evaluability as a walk over the expression** (`semantics/evaluable.go`,
  `Model.ModelLevelEvaluable`), replacing the approximation that reused filter compilation. Literals,
  `null`, metadata access, sequences, constructors, library-function invocations and reads of
  features that reach an evaluable value are evaluable; a user-declared function, an operation no
  library function implements (`~3`), a cast of the instance under evaluation (`(as A).y`) and an
  unresolved name are not. This is what closed the `filter f((as A).y);`, `filter ~(as A).z;` and
  `filter (as A).y->ControlFunctions::collect { … };` rows.
- **Metadata annotation bodies are checked wherever they are written**
  (`passes/w8c_metadata_annotation.go`). The body-feature rule (`Must redefine an owning-type
  feature`) moved out of wave 8B's redefinition pass, which owned it only for prefix annotations, and
  the body's bound values are now checked for model-level evaluability, which is what the `x = ~3` and
  `z = f((as A).y)` rows declare.
- **What an annotation may annotate** (`semantics/annotated_element.go`): the metaclass of the
  annotated element must conform to every type of the `annotatedElement` feature the metadata type
  restates (KerML 1.0 §8.3.4.9). This closed `Cannot annotate Classifier` and
  `Cannot annotate ItemUsage`.
- **A conditional `baseType` binding is decided per annotated element.** `:>> baseType = if
  annotatedElement istype KerML::Structure ? SS meta KerML::Type else CC meta KerML::Class` names one
  base type per annotation, evaluated against the annotated element's metaclass. The two file-wide
  silence rows that closed with the metadata work are `ParsingTests_Metadata.kerml.xt`:33 and
  `MetadataTests_SemanticMetadata_valid.kerml.xt`:31.

One ratchet line moved with it: `kerml-examples/Simple Tests/MetadataTest.kerml` is now clean
(`internal/core/model/testdata/pilot_corpora_expected.txt`). Its one diagnostic was
`unresolved-reference: cc` at line 53, a false positive we owned alone — the differential baseline
records it as `openSysMLOnly` and the pinned validator is silent — and `feature :>> cc;` resolves now
that the annotated element's `baseType` is the branch its metaclass selects.

## What did not move, and why

### The four `Must be model-level evaluable` rows in a metadata body — a span divergence

`MetadataTests_MetadataFeature_invalid.kerml.xt`:66 and :69 and `MetadataUsage_Invalid.sysml.xt`:82
and :85 declare the diagnostic at `= ~3` and `= f((as A).y)`. We report the same rule, at the same
element, on the same line — but spanning the bound expression (`~3`), not the binding.

The pilot attaches the constraint to the `FeatureValue` relationship, whose text region starts at the
`=`. Our AST records the bound expression only: a feature's value is an `ast.Node` on the declaration
(`ast.Usage.Value`), with no node or token offset for the `=`. Reporting the pilot's span requires the
parser to record the binding token, which is wave 11B's package, so the divergence is left standing
rather than approximated. It is a real divergence, not a wording difference: the rule and the element
agree, the span does not.

### The four `Must have a concrete type` rows on `@A;` — tier gating

`MetadataTests_SemanticMetadata_invalid.kerml.xt`:49 and :63 and `SemanticMetadata_invalid.sysml.xt`
:65 and :79 declare `Must have a concrete type` on an `@A;` annotation whose metaclass is abstract.
We implement that rule (`passes/w8c_metadata_type.go`) and it is correct in isolation, but it never
runs on these files: both declare, a few lines later, a `feature :>> x;` whose redefinition target
does not resolve — an error both implementations report and whose row agrees — and a name-resolution
error skips every higher tier (`passes/registry.go`, and AGENTS.md's tiered-passes invariant).

The reference has no tiers, so it reports both. Closing these rows means either running the type tier
after name-resolution errors, or moving this one rule down a tier; the second was measured and
rejected — a concrete-type error at the name-resolution tier then blocks the constraint tier and
suppresses filter diagnostics elsewhere. The first is an architecture-wide decision affecting every
slice, so it is an **adjudication question for the wave owner**, not a change made here.

### `SemanticMetadata_valid.sysml.xt`:53 — a parser row

The file's file-wide silence fails on `actor #B a;` (line 90): we report `expected '{' or ';' after
declaration`. A prefix annotation on an `actor` usage is a grammar gap, owned by wave 11B.

---

## The expectation-count correction

The `.xt` reader took the text of an `at "…"` clause up to the first quote inside it. Xpect does not
escape those quotes, so four assertions were read as two expectations each — a truncated one and a
junk one:

```
// XPECT errors --> "Must have a Boolean result" at "filter new A(null, 1, "", false);"
// XPECT errors ---> "Must have a Natural value" at ""x""
// XPECT errors ---> "Must be a Boolean expression." at "if \"test\""
```

The clause runs to the last quote on the line, and reading it that way removes three junk rows —
`MetadataTests_MetadataFeature_invalid.kerml.xt`:59, `MetadataUsage_Invalid.sysml.xt`:71 and
`MultiplicityRange_invalid.kerml.xt`:51 — and gives the real ones their declared text. The
declared-expectation population is therefore **1323, not 1326**, and three of the 18 fewer
disagreements are that correction rather than a behaviour change. The per-kind census in
[pilot-xpect.md](pilot-xpect.md) is asserted by `cmd/pilot-xpect/w5c_census_test.go`, which is updated
with them; the agreement tables there are the wave-10 baseline and are the wave owner's to rebaseline.
