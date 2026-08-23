# Wave 11E — the KerML validation and visibility rows that stay open

Wave 11E owns 25 Xpect disagreements in the KerML validation and visibility suites. Nineteen closed
by implementing or re-attaching a rule; the six below do not close inside this slice. Each entry
states the fixture, what the pilot declares, what we do, the specification reading, and who owns the
remaining work. Every number is from a fresh run of `go run ./cmd/pilot-xpect -out build/xpect-fresh`
on the branch, not quoted from a report.

---

## E1 — `Type_Multiplicity_invalid`: a second multiplicity is a parse error for us

**Declared.** `error: "Only one multiplicity is allowed"` at
`multiplicity subsets Base::zeroToMany;` — a validation error on a well-formed model.

**Ours.** `expected '{' or ';'` on the same line: our parser has no production for a second
`multiplicity` member in a type body, so the rule can never be reached.

**Reading.** KerML's `Type::multiplicity` is single-valued (KerML 8.3.3.1.1), and the pilot's grammar
admits the surplus member so that its validator can name it. Reporting it as a validation error
requires the parser to accept the form first.

**Owner.** Parser (slice 11B). Not a divergence — a rule blocked behind a missing production.

---

## E2 — `AssociationTest_CrossFeatures_invalid`: the owned cross feature has no syntax here

**Declared.** `error: "Must be the cross feature"` at `x1 [0..1]`, on

```
end x1 [0..1] feature x : C1 crosses y.b { public import y::y1; }
```

**Ours.** No error on that line. The two `A1` rows and the `A2` row of the same fixture agree.

**Reading.** The pilot's rule is `checkFeatureCrossingSpecialization` in `KerMLValidator`: it
compares `FeatureUtil.getCrossFeatureOf(f)` — what the end's `crosses` names — with
`f.ownedCrossFeature()`, the cross feature the end *declares inline* (`x1`), and reports the owned
one when they differ. We have no AST for that inline declaration: `internal/core/ast` records
`RelCrosses` but no owned cross feature, so there is nothing to compare. The rule itself is
KerML-side, and this slice would implement it once the declaration exists.

**Owner.** Parser and AST (slice 11B) for the `end <crossFeature> [mult] feature <name>` form; the
rule then belongs to a KerML validation slice.

---

## E3 — `ConnectorTest_ConnectorEndSubsettingBadCase`: we resolve a feature the pilot does not

**Declared.** `error: "Couldn't resolve reference to Feature 'f'."`

**Ours.** No error anywhere in the file — the reference resolves.

**Reading.** The declared error is a *resolution* verdict about which features a connector end may
name, not a constraint on a resolved model, so no `internal/core/passes` rule can produce it without
duplicating name resolution.

**Owner.** Resolver (slice 11C).

---

## E4 — `SimpleImportTestsFromOtherFile_Import3` and `_FT`: our redefinition check is stricter

**Declared.** File-wide silence for

```
classifier Try specializes B {
    feature try : A::a1 redefines B::b;
}
```

where `B::b` is typed by `A` and `a1` is a class *nested in* `A`, not a specialization of it.

**Ours.** `try (typed by a1) redefines b (typed by A): types do not conform`.

**Reading.** A redefinition is a subsetting, and the values of a subsetting feature are values of the
subsetted feature (KerML 7.4.9, 8.3.4.2), so a redefinition whose type does not conform describes an
unsatisfiable model — `a1` nesting inside `A` gives its values no conformance to `A`. The pilot does
not validate subsetting type conformance at all; it has no constraint for it, leaving the consequence
to a reasoner. So the fixture's silence records the absence of a check rather than the legality of the
model.

**Decision — keep the check, record the divergence.** Dropping it would remove a sound rule to move
two rows, and re-labelling it wording-only would be false: the pilot emits nothing here. These two
rows stay open as a deliberate divergence, and `noErrors` carries them.

---

## E5 — `VisibilityTests_Protected_FeatureChaining`: one of five references

**Declared.** Five errors; four agree as wording-only. The fifth, `Couldn't resolve reference to
Feature 'c'.` at line 34, we do not report — our nearest error is at line 39.

**Reading.** Which segment of a feature chain a protected member is visible through is decided in
visible/local resolution (KerML 8.2.3.5.3-8.2.3.5.4), the same mechanism wave 10E corrected for the
qualified tail. The missing row is a resolution verdict on one chain segment, not a constraint.

**Owner.** Resolver (slice 11C).

---

## E6 — `ShadowingTests_SameNamesImportAsFeature_Rdef`: an ambiguity we raise

**Declared.** File-wide silence, plus `Couldn't resolve reference to Feature 'B'.`

**Ours.** `ambiguous reference: SamePackage::container (2 candidates)`, and our unresolved reference
lands on `container` rather than on `B`.

**Reading.** Both rows follow from how an imported name and an inner declaration of the same name
combine (KerML 8.2.3.5): shadowing should leave one candidate, and the residual unresolved reference
should then be the chain's last segment. This is import and shadowing resolution.

**Owner.** Resolver (slice 11C). The two sibling `SameNames*` fixtures behave the same way and are
already recorded in [pilot-xpect.md](pilot-xpect.md) as fixtures that declare both file-wide silence
and the errors within them.
