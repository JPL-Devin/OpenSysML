# Wave 10 — the three adjudications the slices depend on

Three items in the wave-10 worklist are not implementation choices: they decide what OpenSysML
*means*, so they are recorded here before the slices start rather than settled inside one. Each
entry states the divergence, the measurement behind it, the decision, and which slice owns the work.

Measured on `main` at `0de1181d` (wave 9 merged, including the rebaseline). Every number below is
reproducible with the command given; none is quoted from a session report.

---

## D1 — `allocate` as a usage keyword: require the connector part

**The divergence.** The pinned grammar spells the allocation usage keyword `allocation`
(`AllocationKeyword`, `SysML.xtext:1192`) and gives `allocate` a single role — `AllocateKeyword`,
which must be followed by a `ConnectorPart`:

```
fragment AllocationUsageDeclaration returns SysML::AllocationUsage :
      AllocationUsageKeyword UsageDeclaration? ( AllocateKeyword ConnectorPart )?
    | AllocateKeyword ConnectorPart
;
```

`ConnectorPart` is binary or n-ary, and the binary form requires `to` (`SysML.xtext:1076-1079`). So
`allocate a;` has no derivation and the reference reports `mismatched input ';' expecting 'to'`,
while `allocation al;` is well formed. We accept both: `allocate` is mapped to `ast.UsageAllocation`
and `ast.DefAllocation` in `internal/core/parser/defusage.go`, and the `ast.UsageAllocation` arm
skips the connector ends when it sees `;` or `{`, leaving an allocation usage *named* `a`.

**What is not a divergence.** The map entry is also the only entry point into that arm, so it is how
the legal form `allocate f to g;` parses. Removing the synonym outright would drop a spec form.

**Decision — close the gap without a language regression.** Require a `ConnectorPart` after
`allocate` in usage position, and remove the definition-side entry (`allocate def`, which
`AllocationDefKeyword` does not admit — it is `allocation def` only, `SysML.xtext:1196-1198`).
`allocate f to g;` and `allocation al;` keep parsing; `allocate al;` and `allocate def D` become
errors in the default mode, which is what `g31` measures.

Verify:

```bash
go run ./cmd/pilot-reject            # g31 leaves the gap table
go test -count=1 ./internal/core/parser
```

Owner: **10C**. The RDF export keeps the two spellings distinguishable through
`sysx:declaredKeyword`, so `internal/core/export`'s goldens record the surviving forms unchanged.

---

## D2 — `import` without a visibility indicator: error by default

**The divergence.** `ImportPrefix` makes the visibility indicator mandatory (`SysML.xtext:241-244`,
`KerML.xtext:169-172`) — deliberately, since the `MemberPrefix` beside it leaves visibility
optional. The reference therefore rejects `import Q::*;` with `mismatched input 'import'`. We parse
it and report `import-visibility` as a **warning**
(`internal/core/passes/import_visibility.go`), and a warning is not a rejection, so `g02` scores as
a permissiveness gap.

**Decision — raise it to an error in the default mode.** This is a severity change only: the bare
form already parses unambiguously and the diagnostic already exists, so nothing about how we *read*
a model changes. `expose` stays exempt — the pinned grammar gives it an implicit protected
visibility (`ExposeVisibilityKind`, `SysML.xtext:2366-2372`).

Blast radius, measured rather than estimated:

| Surface | Bare imports | Note |
|---|---:|---|
| `internal/core/libs/stdlib` (117 files) | **0** | 583 imports, all visibility-qualified; `TestStdlibConformance` unaffected |
| our own `.sysml`/`.kerml` | **4** | `testdata/passes/import_no_visibility.sysml` (2, the fixture for this diagnostic), `internal/core/runtime/testdata/conformance/accept_statement_via_port.sysml` (1), and `cmd/pilot-reject/testdata/negative/grammar/g02-import-without-visibility.sysml` (1, which must be rejected) |
| pinned Xpect corpus (428 `.xt`) | **2** | |
| `docs/` prose | 64 lines | no gate reads these; sweep them with the wave's central docs update |

```bash
grep -rnE '^[[:space:]]*import[[:space:]]' --include=*.sysml --include=*.kerml .
```

Owner: **10C**.

---

## D3 — circular re-entry: bound it, by the rule the corpus states

**The divergence as it was framed.** Wave 9D made the enumeration emit the re-entry of a circular
containment, which emptied the `other-paths` class (11 → 0) and left the deep fixtures overshooting:
`ShadowingTests_CircleProblem3.kerml.xt:23` declares 829 names and we offer 3362. The choice was
put as "match the pilot's truncation exactly, or bound the enumeration at a defensible depth".

**Both halves of that framing are wrong, and the rows say so.** There are **8** disagreeing rows
across 4 files, not 5, and they are two different defects:

| Row | Declared | Ours | Shape |
|---|---:|---:|---|
| `CircleProblem3:23` | 829 | 3362 | 2871 extra, **338 missing** |
| `CircleProblem3:183` | 696 | 3362 | 2942 extra, 276 missing |
| `CircleProblem4:23` | 12 | 12 | 2 missing, 2 extra |
| `CircleProblem4:32` | 111 | **67** | we offer *fewer* |
| `CircleProblem4_FT:23` | 76 | 64 | we offer fewer |
| `CircleProblem4_FT:43` | 98 | 86 | we offer fewer |
| `CircleProblem4_Rdef:23` | 76 | 64 | we offer fewer |
| `CircleProblem4_Rdef:43` | 98 | 86 | we offer fewer |

Six of the eight are **under**-enumeration, and `A.B.B` — the re-entry wave 9D was meant to add — is
the first missing name in every one of them, while we offer `A.b.B` instead. The fixture shows the
pilot declaring `A.B.B`, `Test1.A.B.B`, `A.B.b.B` *and* `Test1.A.B.b.B` at
`ShadowingTests_CircleProblem4.kerml.xt:23`: it is not truncating at the second occurrence of a name
at all. So "match the pilot's truncation" is not a well-defined rule — on one fixture the pilot
stops sooner than us and on three it goes further — and it would not buy the rows the framing
credited it with.

**Nor is a depth bound the pilot's rule.** Across all 227 declared `scope` blocks in the pinned
corpus, no name ever appears more than **twice** in a declared path (204 blocks reach 1 repetition,
23 reach 2, none reach 3), while declared paths run to 9 segments deep:

```bash
python3 - <<'EOF'
import re, glob
from collections import Counter
hist = Counter()
for p in glob.glob('build/pilot-xpect-corpus/**/*.xt', recursive=True):
    for at, body in re.findall(r'XPECT scope at (\S+) ---(.*?)---', open(p, errors='ignore').read(), re.S):
        names = [n.strip() for n in body.replace('\n', ' ').split(',') if re.match(r"^[\w.']+$", n.strip())]
        if names:
            hist[max(max(Counter(n.split('.')).values()) for n in names)] += 1
print(sorted(hist.items()))
EOF
```

The bound is therefore per name, not per depth: a path may re-enter a name once, so the cycle is
observable exactly once. That is a rule we can state and defend, and it is *closer* to the reference
than truncation is — so bounding is not a concession we document, it is the correct reading.

**Decision — bound the enumeration by the one-re-entry rule.** State the rule, the 829-vs-3362
number and the residue in the same sentence, per the process rule about metrics that improve a class
label.

**Read the bound honestly: it is necessary, not sufficient.** Some of our extras already satisfy it
and the pilot still does not declare them (`A.B.a.a` — `a` twice), so the reference also declines to
re-expand through some features whose type it has already visited. The bound explains the blow-up;
it does not reproduce the declared set on its own.

### 10A item (a) is three items, not one

1. **Bound the enumeration** by the one-re-entry rule. Closes the 2 `CircleProblem3` rows.
2. **Per-anchor filtering.** The pilot's answer differs between anchors in the same file — 829 at
   `A` (line 23) and 696 at `B` (line 183) — and ours is **3362 at both**. `A` and `B` wildcard-import
   each other, so near-equality is expected, but the 133-name distinction is not something a bound
   produces. The harness computes each anchor independently
   (`cmd/pilot-xpect/scope.go`, `ws.VisibleNames(scope, opts)`), so this is our engine's answer.
3. **The dropped specialization re-entry** in the `CircleProblem4` family, 6 rows. With
   `classifier A specializes A::B` containing `classifier B`, we lose the re-entry level and emit a
   path through the sibling member `b`. `nameWalk` records each path "once, the first time it is
   reached" (`internal/core/model/scope_names.go`), so the shorter `A.b.B` claims the slot and
   `A.B.B` is never recorded — first-reached-wins is the suspect, and it is a defect, not a
   divergence.

Bounding alone moves 2 of the 8 rows. A report that shows the class label improving without items 2
and 3 has not answered this decision.

Owner: **10A**, which merges before 10E per the sequencing rule.
