---
name: testing-sysml-repl
description: How to build, drive, and record end-to-end tests of the OpenSysML sysml REPL (bin/sysml) and the sysml-grpc service with its pysysml Python client — meta-command behavior, symbol lookup, action/state debugging, gRPC slot serialization, and GUI-terminal recording setup.
---

# Testing the `sysml` REPL end-to-end

The REPL is the user-facing surface of `cmd/sysml`. Test it by actually running the binary, not
just via `go test ./internal/repl`.

## Build

```bash
export PATH=/usr/local/go/bin:$PATH   # the repo blueprint puts Go here
make build-sysml                      # -> ./bin/sysml
./bin/sysml --version                 # prints the commit it was built from — useful evidence on camera
```

Always rebuild after pulling a new commit; a stale `bin/sysml` silently tests the old revision.
Print `--version` at the start of a recording so the reviewer can see which commit ran.

### A contrast binary from the previous commit

For any fix whose point is "this used to fail", build the parent revision alongside the new one
and run the same input through both on camera. A worktree keeps the branch checkout untouched:

```bash
git worktree add /tmp/old<sha> <sha>            # e.g. the commit before the fix
(cd /tmp/old<sha> && go build -o /tmp/old-sysml ./cmd/sysml)
git worktree remove /tmp/old<sha>               # when finished
```

Then `/tmp/old-sysml` vs `./bin/sysml` on identical input is the strongest evidence available —
it rules out "the test would have passed anyway". Especially valuable for diagnostic wording and
line/column numbers, where a screenshot of the new behavior alone proves nothing.

## Profiling flags: `-memstats`, `-cpuprofile`, `-memprofile` (PR #156)

`main()` is a one-liner over `runCLI()` returning an exit status, so every profile is flushed by a
`defer` before the process exits. That makes **exit codes the main regression risk** of any change
in `cmd/sysml/main.go`: walk every mode and assert the status, since a `return` mistranslated from
`os.Exit` is invisible in the output text. Values observed at bb0cfdc:

| command | exit |
|---|---|
| `-version`, `-eval '1+2'`, `-validate <clean>`, `<m> -convert sysml -o f`, `<m> -eval '1+1'` | 0 |
| `-validate <model with errors>` (reported as `did not analyse cleanly; no check was made`) | 2 |
| `<m> -convert sysml -validate` (refuses: "check it in its own run"; writes no file) | 2 |
| `-debug -quiet`, `SYSML_MAX_STEPS=abc …`, `-cpuprofile`/`-memprofile` on an unwritable path | 2 |

- `-memstats` writes exactly one line to **stderr** (`sysml: 637ms wall, 242.6 MiB allocated in
  … allocations over … collections, … MiB taken from the OS`). Prove the split with
  `>out 2>err` and `cat` both — a memstats line on stdout would break `-convert -o /dev/stdout`
  and JSON reporting.
- It fires for the interactive REPL too, after the session ends, because it runs in the deferred
  stop. This is the cheapest check that the profile flush survives the REPL path.
  **Leave the REPL with Ctrl-D, not by typing `goodbye`** — at b3f16e4 `goodbye` at the `sysml>`
  prompt is parsed as model text and answers `1:1: error: expected a namespace member` (and, over a
  pipe, makes the whole run exit 2), so a transcript that types it looks like a failure.
- Heap profiles are written at end of run, when the model is already unreachable, so read them as
  `go tool pprof -top -sample_index=alloc_space <file>` (plain `-top` shows inuse_space and looks
  almost empty). Expect `parser.(*Parser).fill` / `parseUsage` and `symbols.*` at the top.
- A 0-byte profile, or `unrecognized profile format` from pprof, is the signature of the flush not
  happening — treat it as a failure of the runCLI change rather than a pprof problem.
- Unwritable-path errors must abort before any model work: assert no `✓` line is printed.
- Generate a load big enough for the numbers to be meaningful (a ~28k-line model with several
  hundred `part def`s allocates ~240 MiB, ~600 ms); `/tmp/m*.sysml` generators from earlier
  sessions may still be around.

## Diagnostics-unchanged checks for core refactors

For perf refactors in `internal/core` (scope indexes, resolve memoization, redefinition-owner
lookup) the only convincing evidence is a **byte-for-byte diff against a binary built from the
parent commit** (see the contrast-binary recipe above):

```bash
for f in inherit_ok noinherit imports_ok imports_bad; do
  diff <(./bin/sysml -validate /tmp/pt/$f.sysml 2>&1; echo $?) \
       <(/tmp/old-sysml -validate /tmp/pt/$f.sysml 2>&1; echo $?) >/dev/null \
    && echo "$f: IDENTICAL" || echo "$f: DIFFERS"
done
```

Fixtures that actually exercise the redefinition-owner path:

- **clean:** `part def Base { attribute speed : SpeedType; }`, `part def Car :> Base;`,
  `part def RaceCar :> Car { attribute speed : SpeedType :>> Base::speed; }` — inheritance through
  a chain must stay clean.
- **still an error:** the same `:>> Base::speed` inside a `part def Other` that does *not*
  specialize `Base` → `error: speed redefines speed, but speed is not an inherited member of Other`
  (code `redefinition-no-inherited`).
- Note an unqualified `attribute :>> weight : Real;` naming a member nothing declares reports
  `unresolved reference: weight` from name resolution instead, never `redefinition-no-inherited` —
  use the **qualified** `:>> Base::speed` form to reach the constraint pass.
- Imports: a wildcard `import Lib::*` plus a nested `import Lib::Inner::Wheel` for the positive
  case, and a usage of an undeclared type for the negative (`unresolved reference: Gearbox`).

## Two ways to drive it

1. **Non-interactive (fast, for exploration and expected-value discovery).** The REPL reads a
   script from stdin fine:
   ```bash
   printf '%%load internal/repl/testdata/vehicle_package.sysml\n%%instantiate Vehicle\n%%features Demo::Vehicle\n%%quit\n' | timeout 30 ./bin/sysml
   ```
   Note `%%` in `printf` format strings. Always wrap in `timeout` so a hang shows up as a
   non-zero exit rather than stalling the session.
2. **Interactive in a GUI terminal (for the recording).** Because the app under test *is* a CLI,
   the recording should show a real terminal session. See the recording setup below.

## Saving and converting models (`%save`, `sysml -convert`)

The CLI is `sysml <model> -convert <format>`: the model is a positional argument and `-convert`
names the format to write, so the output path never decides it — `-o /dev/null`, `-o /dev/stdout`,
`-o /dev/fd/63` and a FIFO named without a suffix all work as they are. An *input* whose
extension names no format needs `-from`, otherwise you get
`cannot tell the format of "input.txt": expected .sysml, .kerml or .ttl, so pass -from`
and no write is attempted. The REPL takes the format from the file extension and has no such
flags, so `%save` says
`name the file with a .sysml, .kerml or .ttl extension` instead — check the two surfaces word
their advice differently and neither mentions the other's remedy.

Things worth setting up as fixtures before a save/write test, since each exercises a different
branch of `internal/core/export/write.go`:

- a FIFO (`mkfifo`) with a **background reader** — the write blocks until something reads; assert
  `ls -l` still shows `prw-` afterwards, i.e. the pipe was written through, not renamed over.
- `/dev/full` — the honest failure path; expect `cannot write /dev/full: no space left on device`
  and a non-zero exit rather than a success line.
- a symlink, a link **chain**, a link into another directory, and a **dangling** link — all must
  stay links, with the file they point at (created, if dangling) holding the new bytes.
- an existing 0600 file (permissions must survive) next to a brand-new one (0644).
- a directory chmod-ed 0500 containing a writable file, where the previous content is **longer**
  than the new model — proves the direct-write fallback truncates instead of leaving stale bytes.
  Remember to `chmod 700` it again or later cleanup fails.
- assert the negative too: no leftover `.name.sysml.<digits>` temp files, and failure messages
  must name the path the user typed rather than the temp file.

**`x.sysml -convert sysml` is a source-preserving formatter, not an AST printer.** It keeps the
original inline/multi-line layout and reproduces surface notation verbatim (a member-attached
`then part b;` comes back as `then part b;`, not as the desugared `then a b;`), so its output is
**not** evidence about what the parser built. To assert on AST/desugaring, use `-convert ttl` (generated
from the tree — e.g. `grep -c SuccessionAsUsage`) or a parser golden fixture. The `.ttl -> .sysml`
direction *is* a real AST print, so a round-trip through Turtle is the way to see canonical notation.
A useful corollary: to prove "no relationship was recorded", count the triples in the `.ttl`, never
grep the reformatted `.sysml`.

Models that convert to Turtle are a narrow set, but the set grows: **condition members
(constraint/requirement `assert`/`assume`/`require`, `subject`, `return`) map since PR #182**, while
state and action nodes still do not. Anything with a state substate, an initial node, `perform`,
`send`, an assignment or prefix metadata still fails with
`cannot convert the <thing> at <file>:<line>:<col>: save to .sysml or .kerml instead …` and exit 2.
How many of `examples/` convert is measured in `docs/project/roadmap.md` § D6; most of the
training copies do, so a Turtle test can use real models — the Constraints/Requirements/
Analysis/Verification training packages are the richest. A useful sweep, which also proves "the message is clear and it never panics":

```bash
find examples -name '*.sysml' -print0 | while IFS= read -r -d '' f; do
  out=$(./bin/sysml -convert ttl "$f" -o /tmp/e.ttl 2>&1) \
    && echo "OK $f" || echo "FAIL $f :: $(echo "$out" | head -1)"
done | sort | uniq -c        # note -print0: many training paths contain spaces
```

A hand-written fixture is still the smallest positive case; this one round-trips and yields exactly
1180 bytes of Turtle:

```
package Demo {
    // a comment
    part def Vehicle {
        attribute mass;
    }
    part v : Vehicle;
}
```

Use it for byte-identity assertions across argument orders (`md5sum … | sort -u | wc -l` must print
`1`) — comparing hashes is far stronger evidence than eyeballing that each invocation "worked".
Beware: `-convert sysml` **does** normalize tab indentation to 4 spaces (comments and inline/
multi-line layout survive), so a tab-indented fixture like `vehicle_package.sysml` will never be
byte-equal to its own reformatted output. Assert idempotency (pass 2 == pass 1) rather than
fixture-equality (pass 1 == input).

For formatter changes, the cheap adversarial check is idempotency over real models:
convert every `examples/*.sysml` with `-convert sysml` twice and `diff` the two outputs — all eight
must be byte-identical, and stderr must stay empty for well-formed input. Pipe the whole loop
through `sort | uniq -c` so the evidence is one aggregate count instead of a screenful that scrolls
the failures off the top:

```bash
for f in examples/*.sysml internal/repl/testdata/*.sysml; do
  ./bin/sysml "$f" -convert sysml > /tmp/p1 2>/dev/null
  ./bin/sysml /tmp/p1 -convert sysml -from sysml > /tmp/p2 2>/dev/null
  cmp -s /tmp/p1 /tmp/p2 && echo idempotent || echo "NOT IDEMPOTENT: $f"
done | sort | uniq -c
```

### `bin/sysml-grpc` takes `-port`, not `-addr`

`-addr` prints "flag provided but not defined" with a usage dump, so drive a freshly built service
with `./bin/sysml-grpc -port 50123` and `pysysml.connect(port=50123, auto_start=False)` to keep
`~/.pysysml/bin` out of it.

## Proving a Turtle round trip loses nothing

`.sysml -> .ttl -> .sysml` is never byte-equal to the input (the back direction is a canonical AST
print: it spells out `in attribute x`, `return attribute`, adds the `;` after a bare condition and
re-indents). So assert **fixed points and AST identity**, never input-equality:

- `model.sysml -> a.ttl -> a.sysml -> b.ttl` with `diff a.ttl b.ttl` empty — this simultaneously
  proves the emitted Turtle is well-formed and reloadable and that nothing was lost.
- `b.ttl -> b.sysml` and `diff a.sysml b.sysml` empty (notation fixed point).
- Strongest: compare the **parsed AST** of the original and of the round trip. There is no CLI dump
  flag, so build a throwaway `main.go` that calls `parser.New(source.New(path, bytes)).ParseFile()`
  and prints `ast.Dump(f)`. It must live *inside* the repo module (`internal/…` blocks an outside
  module), so `mkdir tmp_dumpcmd && go build -o /tmp/dump ./tmp_dumpcmd && rm -rf tmp_dumpcmd`, then
  `diff <(/tmp/dump orig.sysml) <(/tmp/dump rt.sysml)`. Empty output is the no-silent-data-loss
  proof; `git status --porcelain` afterwards confirms the repo is untouched.

Two round-trip failure classes are **pre-existing** (reproduce them with a binary from the parent
commit before reporting them as regressions):

- **Quoted names are printed unquoted.** `package 'Package Example'` comes back as
  `package Package Example {` and `<'1'>` as `<1>`, so the re-parse fails with
  `1:17: expected '{' or ';'`. Most `examples/sysml-v2-training` files use quoted names, so sanitize
  them (`sed -E "s/package '([^']*)'/package P/"`) before a round-trip sweep, or the whole sweep is
  red for one unrelated printer bug.
- `ref` on an `end` attribute is dropped and `redefines`/`typing` relationship order is swapped in a
  couple of models.

### Behavioral bodies through RDF (PR #270)

Since PR #270 an action or state body converts instead of being refused, so a round-trip sweep now
reaches paths that used to stop at the first behavioral node. What to know:

- **`-o` decides nothing about the format, but the *extension* of an input does.** A sweep that
  writes intermediate files as `/tmp/x.rt` and feeds them back gets
  `cannot tell the format` and every model looks refused. Always name intermediates `.sysml` /
  `.ttl` (or pass `-from`).
- A good adversarial single fixture: one `action def` with
  `first/perform/assign/send … via/accept when/fork/join/merge/decision/while/loop-until/for/
  if-else/terminate/done/succession` plus one `state def` with a nested substate, a `region`,
  `entry`/`do`/`exit`, `defer`, choice/junction/shallow+deep history and a
  `transition first a accept s : S via p if g do action … then b`. At 77abc565 it round-trips with
  the *only* difference being that a state named `deep` comes back as `'deep'` (the writer quotes it
  because `deep` is a keyword) — cosmetic, and stable on the next hop.
- Corpus baseline at 77abc565: of the 110 `examples/**/*.sysml`, **95 convert to ttl and back, 15
  are refused**, and the only model whose notation drifts between hop 1 and hop 2 is
  `41. Language Extension/Model Library Example.sysml` (the documented `end [*] ref` parser gap).
  Treat any other drift as a real defect.
- Refusal probes that still work and print no file: two members of one namespace sharing a name, a
  bare `snapshot`, and a succession whose end name needs quotes (`then 'my a' b;`). Note
  `succession then b;` and `transition first a accept s : S;` (no `then`) are *parse* errors, so
  they cannot be used as export-refusal fixtures.
- Two silent-change classes were found in that sweep and fixed in the same PR: prefix metadata
  (`#Safety part def Car;`) is now carried as `sysx:prefixMetadata "#Safety"`, and a feature that
  wrote no kind keyword (`in x : Real`) is flagged `sysx:isKindImplicit` instead of gaining a kind
  on the way back. An `@` annotation ahead of a definition is refused, because the parser records
  it on the declaration before the one it prefixes — worth re-probing if the parser changes.
- `export.ExperimentalNotice` (internal/core/export/experimental.go) is printed verbatim by the CLI
  (stderr), `%save` and `ConvertResponse`, and the same wording is duplicated in
  `cmd/sysml/main.go`, `python/pysysml/`, `api/proto/` and `docs/guide/`. Check every copy whenever
  the mapping's coverage changes.

### Checking the experimental notice's copies (PR #271)

When a PR claims one wording is stated from one place, check the *runtime* copies mechanically and
the *documented* claims by running them:

- Extract the Go literal and compare collapsed whitespace, rather than eyeballing:
  parse `internal/core/export/experimental.go` for the quoted pieces of `ExperimentalNotice`, join
  them, then compare `" ".join(x.split())` against the paragraph `./bin/sysml -help` prints between
  "Turtle is normalized." and "Every run that converts RDF". `cmd/sysml/main.go`'s `wrapped(…, 78)`
  wraps on `strings.Fields`, so also assert every line's **rune** count ≤ 78 — the notice contains
  `§` (2 bytes), so a byte-based wrapper would pass a naive byte check.
- `python/pysysml/conversion.py:EXPERIMENTAL_NOTICE` should equal the same literal; compare it in
  Python against the Go file directly.
- The client fallback lives in `Connection.convert` (`connection.py`, `response.experimental_notice
  or EXPERIMENTAL_NOTICE`). To exercise it, wrap the stub: `Connection._stub` is a read-only
  property, so patch **`conn._service`** with an object that delegates via `__getattr__` and, in
  `Convert`, calls the real stub then `response.ClearField("experimental_notice")` /
  `ClearField("experimental")`. That is the only way to simulate an older service without touching
  the Go side.
- **Guide transcripts go stale silently.** `docs/guide/07-saving-and-rdf.md` and
  `docs/guide/09-python.md` embed real byte counts and refusal messages. Re-run each fenced command:
  the `%save` pair (guide 2's `MyModel` file → `181 bytes of sysml` / `1872 bytes of ttl`) and
  `examples/rdf-interop-demo.sysml` (`7937` ttl / `877` sysml bytes) still hold at 493693a3, but the
  page's refusal example (`examples/state-machine-demo.sysml -convert ttl` → "cannot convert the
  substate member") and its prose "a model whose point is a behavior does not [convert]" are wrong
  since #270 — the model converts, and all ten `examples/parser_features_demo_*.kerml` convert too.
  Grep for the *old* wording (`model structure only`, `bodies state behavior`) across `docs/ cmd/
  python/ api/proto/ internal/` to catch leftover copies, and check re-worded prose paragraphs did
  not leave one line far wider than its siblings (`awk '{print NR": "length($0)}'`).

### Round-trip fidelity of a declaration head (PR #272)

Two narrow export paths decide whether a declaration comes back spelled the way it was written:
`encodeSubaction`/`bareWord` (`internal/core/export/behavior.go`) for a combined state subaction, and
`wroteKindKeyword`/`withoutComments` (`rdf_out.go`) for a kind keyword ahead of a name. Testing them:

- **A fixture only exercises `wroteKindKeyword` when the commented word equals the keyword of the
  kind the declaration would get implicitly.** `in /* attribute */ x : Real;` inside an `action def`
  proves nothing: the inferred kind there is `part`, so the check looks for `part`, the ttl records
  `sysx:isKindImplicit true`, and old and new binaries agree. Put the same members inside a
  `part def` (where `in x : Real` infers *attribute*) and the pre-fix binary writes back
  `in attribute x : Real;` while the fixed one writes `in x : Real;`. Always diff against a contrast
  binary from the parent commit (§"A contrast binary from the previous commit") — an equal result
  means the fixture missed the path, not that the fix is a no-op.
- Useful fixture set for these paths, all inside one `part def`: `in /* attribute */ x`,
  `out // attribute\n y`, `in //* a note\n attribute */ w`, the control `in attribute z`,
  `in /* /* */ v`, `in //* same line note */ u`, plus quoted names that contain the markers
  (`attribute 'a /* b */ c'`, `in 'x{y'`, `attribute 'do'`) — the quoted ones must come back
  character-identical.
- For subactions: `entry do { … }`, `entry do{ … }`, `entry do{perform A;}` must all come back
  `entry do {` (the tight two were the bug), and `entry /* do */ { … }` must **not** gain a `do`.
  A comment adjacent to a real `do` (`entry /*c*/ do { … }`, `entry do/*c*/{ … }`) keeps it too since
  aa9fa5ab, which reads the subaction head through `withoutComments` as well; before that commit the
  comment token took `do`'s place and the `do` was dropped, so use a pre-aa9fa5ab binary as the
  contrast for these two.
- An unterminated comment in a head (`in /* attribute x : Real;`) is a *parse* error
  (`unterminated comment: missing */`), not an export refusal — so it cannot serve as a refusal
  fixture, only as a "writes no output" check.
- Corpus baseline at bf2f21e3: `converted=102 refused=18 total=120` over
  `examples/**/*.{sysml,kerml}`; 97 of the converting models are byte-stable across two RDF round
  trips, 0 drift. **5 models' RDF-written notation does not re-parse** (`parser_features_demo_
  declarations.kerml`, `27. Occurrences/Interaction {Example-2,Realization-1,Realization-2}`,
  `32. Requirements/Requirement Satisfaction`) — e.g. `event X;` comes back as
  `event references X;`, which the parser then rejects. Pre-existing (identical on the parent
  commit), so treat it as the baseline, not a regression.
- A cheap way to separate printer formatting from RDF-hop changes: convert the model both ways
  (`-convert ttl` → back, versus `-convert sysml` alone) and diff the two outputs token-per-line
  (`tr -s ' \t\n' '\n'`). At bf2f21e3 76 of 102 examples still differ that way (`:>` written
  `specializes`/`subsets`/`redefines`, `default 3` written `= 3`, `on;` written `'on';`, `doc`
  comments dropped), **identically on the parent commit** — so use the old-vs-new diff of that report
  as the regression gate rather than the raw count.

Also note the parser rejects `require constraint <name> { … }` (a *named* nested constraint after
`require`/`assume`): `expected '{' after 'require constraint'`. Only the anonymous form and the
`require R { … }` reference form parse, so an export test cannot cover the named variant.

## Argument order and the `--` marker

`cmd/sysml/args.go` permutes arguments before `flag.Parse`, so flags may be written **after** the
model (`sysml model.sysml -convert ttl`, `sysml model.sysml -trace`). This code is on the path for
*every* invocation, so any change to it needs the unrelated modes (`-e`, `-debug`, `-trace`,
`-quiet`, plain REPL load, `-version`, `-h`) re-checked, not just the conversion flow.

The orders worth covering, since each hits a different branch: model first, flags first, model
*between* two flags (`-o out.ttl model.sysml -convert ttl`), a joined value (`-convert=ttl`), an
`-o` value that itself starts with a dash (`-o -weird.ttl`), and a flag whose value could be
mistaken for a model (`-from sysml in.txt -convert ttl` — if `sysml` is read as the file you get a
bogus `open sysml: no such file`).

A **file name beginning with a dash needs the `--` marker**: `sysml -weird.sysml` is reported as
`flag provided but not defined: -weird.sysml` (correct — a genuine typo like `--badflag` must still
be caught), while `sysml -e "1+1" -- -weird.sysml` loads it. Remember `--` is POSIX end-of-options,
so **everything after it is positional**: `sysml -- -weird.sysml -e "1+1"` loads the model and then
tries to open `-e` as a second file (`load -e: open -e: no such file or directory`). Write the flags
*before* the marker. A regression here is easy to miss — assert on a dash-leading file name
explicitly, because a permutation bug that drops `--` still looks fine for ordinary file names.

## Fixtures worth knowing

- `internal/repl/testdata/vehicle_package.sysml` — everything nested in `package Demo`
  (`Engine`/`power`, `Vehicle`/`mass`+`engine`, `calc add`, a passing `withinMassLimit` and a
  failing `overMassLimit` constraint, `requirement SafeMass`). Ideal for package-scoped vs
  qualified lookup, and for pass *and* fail constraint paths in one file.
- `internal/repl/testdata/action_debug.sysml` — `Debug::tally` with named nodes
  `start, accumulate, end`, so `%break accumulate` has something to stop at; completes with
  `total = 5`.
- `internal/repl/testdata/state_debug.sysml` — `Debug::Cycle`, timed transitions at +10 and +5.
  Good for `%advance` accumulation: `%advance 1` then `%advance 9` reaches the event due at 10
  (`working`), and ten successive `%advance 1` calls process exactly two events (one at t=0, one at
  t=10) with zeros in between — a per-call deadline that did not accumulate would never reach t=10.
- `internal/core/runtime/testdata/conformance/state_orthogonal_regions.sysml` — `Test::TrafficLight`
  with two regions; `%current` should print one state per region joined by `|`
  (e.g. `start | start`, then `Walk | Green`), never `<unknown>`.
- Write your own for ambiguity: the same simple name (`part def Vehicle`) in two packages forces
  `error: symbol "Vehicle" is ambiguous: Alpha::Vehicle, Beta::Vehicle (use a qualified name)`.
- Write your own for parse errors (e.g. `attribute mass = ;` plus a missing `}`): the parser never
  panics, so the REPL should print diagnostics and keep accepting commands.
- `internal/core/runtime/testdata/conformance/state_body_state_local_member.sysml` — `test::monitor`,
  whose substate `working` declares `localGain` that its own entry action reads together with the
  package's `pkgBonus`; `%state monitor` + `%advance 1` reaches `done` with `result = 5.00`. The
  scope-regression canary for states declared directly in a machine body.

## Variations, variants and redefinition-inherited members

Fixtures live in `internal/core/runtime/testdata/conformance/`: `variation_attribute_selection.sysml`
(`test::idealDiamond`), `variation_part_selection.sysml` (`test::electricVehicle`),
`variation_interface_selection.sysml` (`test::nestedAssembly`), `variation_unselected.sysml`
(`test::unconfiguredDiamond`) and `ballandchain_variant_configuration.sysml`. Each `.expected.json`
is the cheapest source of the values `%features` should print.

- **Variant rendering.** A bound variation slot prints `name = variantName (Instance ID: n)` with the
  variant's nested values indented under it (`engine = electric (Instance ID: 2)` / `power = 150.00`).
  A plain `name = Instance(ID: n)` for a variation feature means nothing was bound.
- **Assert a computed attribute, not just the variant name.** `curbMass = 1200.00` (= `900 + mass`)
  distinguishes the `electric` variant (mass 300) from `petrol` (mass 200 → 1100); the variant label
  alone would look the same if the wrong nested values were materialized.
- **Always include a constraint that must be *violated*.** `variation_attribute_selection` asserts
  both `isIdeal` (satisfied) and `notShallow` (violated). An implementation where
  `x == x::variantName` returned true for any variant would still show `isIdeal: satisfied`, so the
  violated one is the only real discriminator. `%features` renders these inline as
  `name: <constraint: satisfied|violated>` — note `%satisfy` answers
  `no satisfaction assertion in the session` for `assert constraint` members, so use `%features`.
- **Error paths** (all are per-slot `<error: …>` lines, and the dependent computed slot repeats the
  cause): unselected → `variation has no variant selected: <usage>.<feature>`; a name that is not a
  variant → `not a variant of the variation: X is not a variant of Y (variants: a, b)`; two
  selections (`= (cut::cutIdeal, cut::cutShallow)`) → `more than one variant selected: variation cut
  selects 2 variants (cutIdeal, cutShallow)`. Assert the *variant list* is present — a message that
  stops at the offending name is the weaker pre-fix shape. Follow each with `%eval 1 + 1` → `= 2` to
  show the session survived, and wrap piped runs in `timeout` so a hang shows as non-zero exit.
- **Redefinition merge canary.** A three-line model is enough:
  `part base : Outer { part :>> inner { attribute :>> b = 7.0; } }` plus
  `part derived :> base { part :>> inner { attribute :>> a = 1.0; } }` where `Outer` computes
  `t = inner.b`. Fixed builds print `inner = {a = 1.00, b = 7.00}` and `t = 7.00`; a build that drops
  inherited nested members prints only `a = 1.00` and
  `t: <error: slot derived.t: member b not found in instance>` — a perfect A/B against the parent
  commit. Name the part something other than `derived` if you want to avoid the (harmless)
  `"derived" is a reserved keyword` warning.
- **A valueless feature of a value type (`attribute d : Real;`) renders as `= <unset>`** since the A3
  work (`runtime.Context.HoldsNoValue` + `runtime.UnsetText`); a collection of them reads
  `[<unset>, <unset>]` and no `(no features)` block is printed under it. On revisions *before* that
  change the same slots read `= Instance(ID: n)` / `(no features)` (also `ringCost`, `ringPort`), so
  a parent-commit binary is the ideal A/B. Things that must stay object-shaped: a class-typed part
  (`Instance(ID: n)` with its nested features, and a genuinely featureless `part def Empty` still
  legitimately shows `(no features)`) and a value type that *does* declare features
  (`attribute def Point { attribute x : Real = 1.0; }` still expands). Enums, strings, booleans,
  quantities, derived attributes and `null` are untouched.
  Caveat worth re-checking on any follow-up: an expression over an unset slot
  (`attribute n : Real = d + 1.0;`) still reports
  `<error: slot q.n: type mismatch: operator '+' is not defined for an instance and a Real>` — the
  diagnostic still says "an instance" rather than unset, i.e. the unset spelling has not reached the
  type-mismatch wording.
- **Surfaces to check together for any value-rendering change**, since they share `formatValue`:
  `%features` / `%eval` in the REPL, `sysml <model> -instantiate <fqn> -e <expr>` (same text), the same
  run with `-json` (the JSON encoder escapes it, so grep `\u003cunset\u003e`, not `<unset>`), and the
  gRPC/pysysml path. On the Python side `pysysml.UNSET` is a falsy singleton spelled `<unset>` and
  distinct from `None`: assert `inst.d is pysysml.UNSET`, `inst.d is not None`, `bool(inst.d) is
  False`, `inst.get_slot('d').value.WhichOneof('kind') == 'unset'` with `materialized=True`, and
  `'&lt;unset&gt;' in inst._repr_html_()`. `Value.unset` is send-only: the client cannot build it
  through `_python_to_value`, so to prove the server refuses it, hand-build the request and call the
  stub directly —
  `conn._stub.EvaluateCalc(sysml_pb2.EvaluateCalcRequest(model_hash=m.hash, symbol_id='P::Add',
  arguments=[sysml_pb2.Value(real_value=1.0), sysml_pb2.Value(unset=True)]))` answers
  `error = 'calc argument could not be read: unset is not a value a caller can supply'` (no result),
  and a follow-up `conn.calc('P::Add', m.hash, [1.0, 2.0])` → `3.0` proves the service survived.
  The refusal arrives **in band** — `resp.error` with `failure_reason=FAILURE_REASON_EVALUATION`, not
  an `INVALID_ARGUMENT` status exception — so assert on `resp.error`, and note the stub method is
  `EvaluateCalc` (there is no `Calc`).
- **Getting the CLI value surface wrong wastes a run:** `-instantiate <fqn>` *alone* prints only the
  creation lines and no slot values — the value surface is `-instantiate <fqn> -e <fqn>::d`, while
  `-e` *without* `-instantiate` answers `sysml: "…" has no value to evaluate` and exits 1 (the
  no-instance path, not unset). In `-json`, grep `u003cunset` — a pattern like `u003cunset.003e`
  misses, because `\u` is two characters. And write the null case `attribute nul : Real[0..1] = null;`:
  `Real = null` is itself a multiplicity violation and you end up debugging that instead.
- **Known limits, so don't plan around them:** `%features` takes only the instantiated usage's own name
  — `%features test::electricVehicle::engine` answers `no instance of …`, and
  `%eval test::electricVehicle.engine.power` answers `usage test::electricVehicle has no value`.
  Nested traversal is only observable through the indented nested rendering of the top-level `%features`.
- **Careful with `clear` while recording:** typed at the `sysml>` prompt it is parsed as a
  declaration (`1:1: error: expected a namespace member`) *and* drops previously created instances,
  so the next `%features` says `no instance of …`. Use `%clear` to reset the session, and clear the
  screen before entering the REPL.

### `variant` used outside a variation, and per-variation-point variant objects

This section describes behavior added by PR #128, so it applies only once that PR is on `main`; the
"old" shapes below are what current `main` prints, so they double as A/B canaries. Three easy
discriminators for changes in the variant/variation layer:

- **A `variant` member whose owner is not a `variation` must stay an ordinary feature.** Model:
  `part def P { attribute k : Real = 2.0; variant attribute x : Real = 1.0; }` +
  `part p : P { attribute total : Real = k + x; }`. Correct behavior: loading prints a *warning*
  (code `variant-outside-variation`) —
  ``variant x is declared in P, which is not a variation, so it offers no choice; declare its owner `variation` or drop `variant` `` —
  and `%features M::p` still shows `k = 2.00`, `x = 1.00`, `total = 3.00`, with `%eval M::p.x` → `1.00`.
  A build that skips every `DeclaresVariant` member instead prints no warning, omits `x`, reports
  `total: <error: slot p.total: type mismatch>` and `error: evaluation failed: member x not found in
  instance`. The same must hold for a package-level `variant` (warns, still readable through a
  computed attribute such as `h = loose + 1.0` → `6.00`) and for a `variant` with no value (warns,
  renders as an instance of its type). Genuine variants *inside* a `variation` must NOT warn —
  loading the `variation_*` conformance fixtures should stay diagnostic-free.
- **Two variation points selecting the same variant must not share one object.** Model: an
  `attribute def Shade { attribute lum : Real = 3.0; }`, a
  `variation attribute def ShadeChoice :> Shade { variant attribute bright :> Shade; variant attribute dark :> Shade; }`
  and a part with `attribute front : ShadeChoice = front::bright;` plus
  `attribute rear : ShadeChoice = rear::bright;`. Correct behavior prints two *different* instance
  IDs (`front = bright (Instance ID: 2)` / `rear = bright (Instance ID: 3)`); a build whose variant
  object cache is keyed only by `{owner, variant}` prints `Instance ID: 2` for both. The instance ID
  is the only visible signal here, so read it carefully.
- **A variation point may inherit its variation-ness, and its variants are still choices.** Model:
  `variation part def EngineChoice :> Engine;` with
  `part def Car { part engine : EngineChoice { variant part electric : Engine { attribute :>> power = 150.0; } } }`
  and `part ev : Car { part :>> engine = engine::electric; }`. Correct behavior: no warning and
  `engine = electric (Instance ID: 2)` with `power = 150.00`. A build testing the owner by keyword
  only either reports `not a variant of the variation: electric is not a variant of engine (variants:
  electric, …)` (self-contradictory) or `usage engine::electric has no value` plus a spurious
  `variant-outside-variation` warning. The same applies when the owner redefines a variation usage
  (`part :>> engine { variant part … }`) without restating `variation`. Drop the variants' own
  `: Engine` to test the second half: a variant specializes its variation point, so untyped
  `variant part petrol;` must still print `power = 100.00` (Engine's default) rather than
  `(no features)` — that shape is the sharper canary, since it needs the supertype edge, not just
  the selection.

## Testing lexical scope of behavior bodies (declaring-scope changes)

When a change claims "expressions in an action/state body now see their declaring scope", the same
model must be run on both binaries — but pick fixtures that can actually distinguish the layers,
because the obvious ones cannot:

- **A state fixture whose states share the machine's imports proves nothing about per-state scope.**
  If every name a state's behavior uses is also visible from the machine (or the package), a
  machine-scoped and a state-scoped lookup give identical output. To isolate the state's own scope
  the state must declare a member that *only* its own scope can answer
  (`state working { attribute localGain = 4.0; entry { assign result := localGain + pkgBonus; } }`),
  and for nesting a substate inside a `region` must declare one read by its own entry action.
  A broken build reports `error: event processing failed: enter state: entry action: eval assignment
  RHS: unresolved reference: localGain`.
- **Always include a shadowing case, because the failure mode is a wrong value, not an error.** Give
  a state (or an action, or a loop/if block) a member whose name also exists at package level with a
  *different* value, and assert the inner one wins. A build that hands out the enclosing scope
  completes happily with the package's number (e.g. `result = 100.00` instead of `4.00`), which no
  error-message check would ever catch. The three-layer action shape (package `h`, action `h`,
  block-local `h` → `seenAction = 2.00`, `seenBlock = 3.00`) is the equivalent for action bodies.
- Substates are **not** entered by nesting `initial`/`then` inside a plain `state`; that shape runs
  the outer entry and completes without ever entering the inner state (`innerRan = 0.00` on every
  revision, pre-existing). Use the `region` form from `state_fork_join_pseudostate.sysml`, with a
  transition naming the substate (`transition init to inner;`), or the machine never descends.
- Quantity values are the cheapest scope probe for imports, since a missing `SI::*` shows up as
  `not a quantity expression: not a measurement unit: unresolved unit s` rather than a wrong number.
  Pair each such model with a bare-`Real` rewrite and assert the same magnitudes, which catches a
  silently dropped unit factor.
- Comparing whole-session output across binaries with `diff` is a good regression sweep, but the
  `State data:` / `Results:` blocks are printed in **map iteration order**, so lines legitimately
  reorder run to run (`leftDone`/`rightDone`, a lone `status`). Diff the *values*, or treat a
  pure line-reordering diff as identical.

## Things to exercise (and their expected shapes)

- There is **no `%what` command** — check `%help` before believing a task description. The lookup
  surface for "does this name resolve?" is `%instantiate` / `%features` / `%eval` (a `part def` is
  easiest via `%instantiate`, an attribute via `%eval`), all funnelling through
  `internal/repl/lookup.go`. A request phrased as "`%what`/lookup" means those.
- Symbol-taking commands: `%instantiate %features %eval %calc %constraint %requirement %action %state`.
  All go through one helper (`internal/repl/lookup.go`), so test each with a **simple** name and a
  **qualified** one.
- `%slots` is a deprecated alias of `%features`, dispatched through the same code: the identical
  listing, led by `note: %slots is deprecated — use %features`. It stays out of `%help` but tab
  completion still offers it, so a script written against the old spelling keeps working.
  Things worth asserting when the alias table (`metaCommand.instead`, `deprecationNote`) changes:
  the note appears **exactly once** per invocation and only for the deprecated spelling; the no-arg
  usage line names the spelling the *user typed* (`usage: %slots <name>`, not `%features`); and the
  note must not shift the exit status — a listing carrying `<error: …>` still exits `2` over a pipe
  under either spelling, so check `printf '%%instantiate X\n%%slots X\n' | ./bin/sysml m.sysml; echo $?`
  next to the `%features` form. The cheapest strong evidence is a **byte-for-byte diff of the two
  listings captured in one session** (run `%features N` then `%slots N`, drop the note line, compare):
  a per-spelling code path that drifted shows up there and nowhere else.
- A part whose type contains its own kind does **not** print a "materialization is bounded" note; the
  bounded walk renders the nested feature as `child : Node (not expanded: contains its own kind)`
  after expanding one level. Don't grep for wording the binary never emits — capture the real line
  over a pipe first. Follow it with `%eval 1 + 1` → `= 2` to show the session survived the walk.
- `%step` is **action-only**. In a state session it answers
  `error: no active action session (use %action <name> first)`; drive a state machine with
  `%advance <time>` instead. That message during a state sweep is expected, not a broken session.
- When comparing two variants of the same model (e.g. a `private import` version against a
  `public import` or bare-`Real` rewrite), load each in a **separate REPL process**. Loading both
  into one session makes the shared simple name ambiguous —
  `error: symbol "propagate" is ambiguous: DescentQ::propagate, DescentR::propagate (use a qualified
  name)` — and qualifying it is not a reliable escape, since one bad submission in between can
  invalidate the other snippet. `./bin/sysml <file>` per variant keeps the runs independent.
- Meta-commands are only understood at the `sysml>` prompt: a shell habit like `clear; %action foo`
  is parsed as SysML, pollutes the session buffer, and can make an already-loaded package's symbols
  stop resolving (`unresolved reference: DescentR::propagate`). Type shell and REPL commands in
  separate turns, and `%clear` (or restart) if a stray line lands in the buffer.
- `%satisfy` takes no argument (every satisfaction assertion the model states) or the name of the
  element stating them, since `assert satisfy … by …` is anonymous.
- Instances are keyed by resolved FQN, so `%instantiate Vehicle` then `%features Demo::Vehicle` must
  hit the same `ID`, and the reverse spelling too. Differing IDs = broken keying.
- Qualified attribute access works with a full FQN (`%eval Demo::Engine::power` → `= 300.00`) but a
  **partial** qualification (`%eval Engine::power`) is `unresolved reference: …` — the qualified path
  goes through the index, which wants the whole FQN. Expect this, don't file it as a bug without
  checking intent.
- `%break <node>`: unknown node names are rejected *with the valid node list*; after a stop,
  `%tokens` should print the **node name** (`Token 1 @ accumulate`), not a Go type like `*ast.Usage`.
- A breakpoint on the node a token **already occupies** must fire: right after `%action Debug::tally`
  the token sits at `start`, so `%break start` + `%continue` has to pause at `start` rather than
  running to completion. Then check the mirror case — resuming with `%step` or `%continue` must not
  re-trigger on that same node (the executor records fired token/node visits). Test both directions;
  "fires" and "doesn't re-fire" are separate bugs.
- `%advance` also drives a state's **do behavior**: when the only queued event is past the deadline
  but the current state has do actions left, they run and the output gains a
  `Do behavior actions run: N` line (`internal/repl/testdata/state_do_far_event.sysml`, event at
  t=100 — `%advance 1` runs 2 do actions and `%current` shows `count = 2`). Assert the clock does
  **not** jump to the far event and the event stays queued, and that repeating small advances does
  not re-run the behavior (`count` stays 2, `0 event(s) processed`).
- `%advance` arguments: `abc` → invalid-time error, `-5` → must-not-be-negative, no arg → `usage:`,
  `1e400` → invalid (overflows a float64), a trailing `s` is allowed (`%advance 10s`), `0` still
  drains events already due, and a huge value → `No pending work - simulation time is now <clock>`
  (must not hang).
- `NaN`, `inf`, `-Inf` and `NaNs` (the `s` suffix is trimmed first, so `NaNs` parses as `NaN`) must be
  rejected with `expected a finite number of time units`. The important follow-up assertion is that
  the **clock survives** a rejected argument: run `%current` afterwards and check `Time:` is still a
  real number, then that normal advances still accumulate. A regression here poisons the clock to
  `Time: NaN` for the rest of the session, which no single error-message check would catch.
- **Two clocks, deliberately.** `stateSession.now` is the debugger clock (seeded from the executor,
  incremented by every requested duration, and it keeps moving even when nothing is queued);
  `exec.CurrentTime()` only moves when an event is processed. So `%advance` prints
  `✓ Advanced to <debugger clock>` plus `Last event at: <executor clock>`, and `%current` prints both
  `Time:` (debugger) and `Last event at:` (executor). `Advanced to 20.00` above `Last event at: 15.00`
  is correct, not a bug. When asserting on accumulation, use `%current`'s `Time:`.
- Error paths that must never panic: unloaded session (`no declarations loaded`), debugger commands
  with no active session, wrong-kind symbols (`%action` on a part def → `is not an action`),
  nonexistent names, and malformed qualified names (`Demo::`, `::Vehicle`, `Demo::::Vehicle`).

## Accepts in actions (accept/send semantics)

An action token that reaches an `accept` with no matching message **parks** instead of failing:
`%step` reports `State: Waiting` and keeps the token at the accept node. Two paths worth testing:

- Satisfiable: `internal/core/runtime/testdata/conformance/action_send_accept.sysml`
  (`%action communicator`) — `%break counter` pauses on the accept node, and resuming completes with
  `number = 50` / `n = 50` (the typed accept skips the String and takes the Integer). Loading these
  fixtures used to print a tier-2 `unresolved reference: n` diagnostic for the `assign` that reads
  the accepted parameter while the runtime bound it anyway; **PR #196 fixed that**, so on current
  revisions loading them must be clean. Seeing that diagnostic again is a regression of the
  accept-payload scope contribution, not harness noise.
- Unsatisfiable: write a one-accept action with nothing sending to it. `%step` → `State: Waiting`,
  and `%continue` must return a typed
  `accept deadlock in action <name>: nothing can post the awaited message (...)` — **never** hang.
  A breakpoint on the parked node still fires, and `%stop` then `%continue` gives
  `no active action session`.

Always run these under `timeout` when driving over a pipe; a hang is the failure mode to catch.

### An accept node's payload as a body-scoped name (PR #196)

`internal/core/resolve/accept_payload.go` contributes `action r accept msg : T;`'s payload to the
**body** the accept node is declared in, so sibling nodes read it by simple name. Test it on both
surfaces — `bin/sysml -validate f.sysml` for the diagnostic and
`printf '%%load f\n%%action <n>\n%%continue\n%%quit\n' | timeout 30 ./bin/sysml` for the value —
because check-clean alone never proves the runtime bound anything.

- The A/B against a parent-commit binary is what makes this convincing: the same model gives
  `error: unresolved reference: msg — did you mean test::communicator::receiver::msg?` (exit 2) on
  the old binary and `✓ … no errors` on the new one. Payload names must be **non-keyword** —
  `accept first : Integer` is parsed as an initial node and derails the whole graph with
  `action has multiple initial nodes`; `first`, `after`, `then` are all reserved.
- Positive shapes worth one run each, with the numbers a correct build gives (send value in
  parens): later sibling reads `msg + 1` (42 → `seen = 43`); nested `if msg > 5` + `while` in a
  sibling node (7 → `total = 21`); two accepts binding `alpha`/`beta` (11, 30 → `sum = 41`);
  reader declared textually **before** the accept but sequenced after it (4 → `seen = 5`).
- **Shadowing is the case only a value can catch.** A package-level `attribute msg : Integer = 1;`
  keeps the model check-clean on *both* binaries, so assert `seen = 9` (the sent payload) and not
  `1`. Note a *body*-level `attribute msg = 111` in the same action is a weak probe: the accept
  writes the same value slot, so `seen = 9` either way — prefer the package-level form.
- Negatives that must stay `unresolved reference: msg` at exit 2, and each exercises a different
  guard: a misspelling in the same body (now gains `— did you mean msg?`); a **sibling** action's
  node in the same package; a package-level `attribute` initializer; a wildcard
  `import src::communicator::*` into another package; and a `part def` body declaring an action
  node with the accept (`sharesBodyFeatureSpace` rejects a part — this one is easy to forget).
- A node that executes **before** the accept binds must fail, not read stale data:
  `error: execution failed: eval assignment RHS: unresolved reference: msg` in ~0.1 s, with
  `%tokens` afterwards still showing `Token 1 @ <node>` (session survives).
- A **qualified** `receiver::msg` reference resolves at check time and then fails the run with
  `usage receiver::msg has no value` — identical on both binaries, i.e. pre-existing rather than a
  regression of this change. Always A/B it before reporting it as a defect.
- Cheap corpus gate for a resolve change: loop `internal/core/runtime/testdata/conformance/*.sysml`
  and `examples/*.sysml` comparing `grep -c 'error:'` counts new vs old binary and print only files
  where new > old (~1 min).

## Addressed sends and per-object message identity (`send S() to t`, PR #267)

To observe *which object* consumed a message you must drive one performer at a time. `%state` and
`%action` take an object argument (`%state <machine> <object>`, `internal/repl/meta.go:320`) but the
object must already exist, so **`%instantiate <Pkg>::<part>` first** — otherwise every command
answers `error: no instance of "…" (use %instantiate first)` followed by `no active state machine
session`, which reads like a broken session.

The fixture shape that distinguishes confined from leaking delivery is two structurally identical,
unconnected parts of one type, each with a `via`-qualified accept **and** a catch-all:

```
transition first waiting accept Ping via inPort then received;   // the addressee
transition first waiting accept Ping             then strayed;   // the leak detector
```

Run the same model once per performer: the addressee must reach `received`, every other object must
stay in `waiting`. `strayed` is the pre-fix signature (a same-named port of another object consuming
the message) and it appears for *both* objects on a pre-#267 binary — always A/B against a
parent-commit binary, since "it ran and ended somewhere" proves nothing on its own.

Limits worth knowing before writing fixtures:

- **Nested performers cannot be driven from the prompt.** `%state <M> Pkg::one::mid::inner` fails
  with `no instance of "Pkg::Mid::inner"` even after `%instantiate Pkg::one` materialized it (visible
  in `%slots`). So a nested/composite target (`one.mid.inner.inPort`) is only observable *negatively*:
  put the catch-all accept on the **sending outer** object and assert it stays `waiting` with no
  error — the deep path resolved (else it would be a typed error) and did not leak outward.
- **`accept … via <dotted.path>` does not parse** in a state transition (`expected a body member` /
  `expected the target of the transition after 'then'`) nor in an action node (`expected ';' after
  accept action`). Only `send … via p.q` takes a nested path. Don't design an accept-side nested-port
  fixture; use `conformance/port_nested_port_path.sysml` (connect `p.q` to a simple `sink`) instead.
- Addressing needs **no connector**: `send Ping() to two.inPort` from `one` delivers to `two`, and the
  sender must not consume it. That cross-object case is the cheapest positive proof of identity.
- `send X to self` resolves to nothing and is a silent no-op on both old and new binaries (no
  receiver is named `self`) — expect `waiting`, not an error. A target naming a **part** rather than a
  port (`one.mid.inner`) is also delivered-but-unaccepted, not an error.
- The two unroutable wordings are distinct and both must stay reachable
  (`internal/core/runtime/routing.go`): an addressed target gives
  `send reaches no receiving port: "alpha.count" names no port of an object the sender can address`,
  while a `via` port gives `port "lonely" is joined to no port that can receive it` /
  `port "dst" is joined only to outbound ends (src)`. A pre-#267 binary **completes successfully**
  (`✓ Action completed`) for the addressed-target cases, so assert the error text, not just non-zero
  exit. From a state entry action it surfaces as
  `error: event processing failed: enter state: entry action: send reaches no receiving port: …` with
  `Current state: <none>`; the REPL session must survive it (`%tokens` still shows `Token 1 @ sender`,
  `%eval 1 + 2` still answers `3`).
- Regression set that must be byte-identical old vs new for any routing change:
  `port_nested_port_path` (`got = 7`), `port_interface_typed_connection` (conjugated `~Chan`,
  `got = 13`), `send_no_reachable_receiver`, `send_into_outbound_only_end`,
  `state_send_self_signal` (`send Ping to Driver` → `done`), `action_send_accept` (accept-node target
  → `n = 50`, `number = 50`).
- Loading these conformance fixtures in the REPL prints pre-existing
  `unresolved reference: Integer — did you mean ScalarValues::Integer?` noise; it reproduces on the
  parent binary and does not stop execution.

## Standard SysML v2 behavioral notation (flows, triggers, sends, transition effects)

Testing this family end-to-end has a few traps that cost a whole run if hit late:

- **A state machine cannot be handed a signal from the REPL** — there is no `%send`. To exercise
  `accept <p> : <Item> [via <port>]` interactively, the model must post the message itself:
  give the machine `port out : P; port in : P; connect out to in;` and a state whose
  `entry send Item(9) via out;` feeds the transition (the shape of
  `internal/core/runtime/testdata/conformance/state_transition_accept_via_port.sysml`). The shipped
  `state_transition_accept_payload.sysml` has **no** sender — its event comes from the
  `.expected.json` `events` array, so in the REPL it just sits in `idle` forever. That is the
  harness, not a bug.
- **Always run a signal-driven state model on both the REPL and the gRPC/conformance path and
  diff them.** They disagreed until `ProcessNextEvent`/`%advance` learned to dispatch a pending
  context-bus signal: a port send produced by an entry action was delivered by `RunToCompletion`
  (gRPC `execute_state`, conformance) but not while stepping, so the machine parked with the
  attribute at its initial value. `TestAdvanceDeliversPendingPortSignal` locks the parity; a
  REPL-only result understates the feature and a gRPC-only result hides a debugger gap.
- **A transition effect is worth testing per effect form**, because they lower differently:
  `do assign x := <expr>` and a performed-action reference (`do perform Bump then s;`) reach the
  executor through different lowering paths, and the performed form used to abort with
  `transition effect: unsupported action type: *ast.Membership`. A parse-only check on such a form
  proves nothing — always drive the transition and assert the attribute changed.
- After any change to statement-termination in `parser/behavior.go`, re-run a **negative** case in
  the same breath: a genuinely missing `;` in an action body (`assign n := n + 1` followed by
  another statement) must still report `expected ';' after assignment`, and one in a state body
  must still report `expected '{' or ';' after declaration`.
- `accept at <t>` / `accept after <d>` in an **action** body are supposed to fail fast with
  `no clock to wait on: … a time event is only waited on by a state machine's transitions` — check
  they do not park forever. In a **state** transition, plain `accept after 5` works while
  `accept after 5 [s]` fails with `schedule events: time duration must be constant, got quantity`;
  prefer the unitless form unless quantities are the thing under test.
- Both spellings of a machine's start state work: an explicit `initial <s>;` and the standard
  `entry; then off;` succession out of the body's own entry subaction. If a bodied
  `exhibit state s { … }` reports `initialize state machine: no initial state found in state
  machine <name>`, neither reached lowering — check the state body actually parsed.
- Corpus regression sweep for parser work: `./bin/sysml <file>` over
  `/home/ubuntu/corpus/apps/*.sysml` and `/home/ubuntu/corpus/dragon/Dragon.sysml`, counting
  `error:` lines, is a cheap high-signal gate — read each remaining message and classify it as
  structural/unresolved-library vs behavioral rather than trusting the count alone.
- Don't name a pysysml probe script `grpc.py`: `sys.path[0]` shadows the real `grpc` package and
  pysysml dies with `partially initialized module 'pysysml'`, which looks like a client bug.

## Fork/join and an action's shared value space (PR #170)

An action performance holds **one** value space (`ActionExecutor.data`, read back by `Results()` and
`Data()`); a fork duplicates control only, a join merges nothing, and a retiring token carries
nothing out. Consequences worth asserting whenever anything in `internal/core/runtime`'s action path
changes:

- Every branch's writes must appear in `Results:` together. The historical bug (pre-#170) was that
  each token carried its own copy of the data, so only the **last-retired** token's snapshot
  survived — a broken build still completes with `✓ Action completed`, just missing values, so
  "it ran" proves nothing. Always A/B against a binary built from the parent commit and assert the
  exact numbers.
- Fixtures that distinguish working from broken, and the values a correct build gives (an incorrect
  build zeroes the branch that retires first):
  `fork` + two branches assigning `x`/`y` → `x=1 y=2`; three branches → `1/2/3`; nested forks where
  the inner join's successor reads what the inner branches wrote; a branch whose body is a
  `while` loop (`n=10 i=5` — the loop runs entirely within one token step); a branch that reads a
  feature the other branch wrote (`seen=42`, since a read after the other branch's write sees it);
  an `accept` in one branch (`action rcv accept n : Integer;` with the other branch sending);
  a branch ending at a node with **no succession** (no join) — both branches' values must survive;
  a fork whose branch invokes a nested action with `in`/`out` params that itself forks (only
  parameters cross the boundary — the callee's own locals must NOT appear in the caller's results).
- Both branches assigning the **same** attribute is deterministic last-write-in-step-order, not a
  merge: assert a single stable value over repeated runs (`w=10` for branches writing 10 then 20)
  and that it never panics, hangs or drops the feature.
- `%tokens` prints token **locations** followed by one shared `Values:` block, never per-token
  values. At a fork expect `Token 2 @ left` / `Token 3 @ right` with a single `Values:` — per-token
  blocks or duplicated names are the pre-#170 display. After completion `%tokens` says
  `No active tokens` and the values only show in the `Results:` of the final `%continue`.
- An unsatisfiable `accept` inside a fork branch must still fail fast (exit 2, ~0.1 s) with
  `accept deadlock in action <name>: … ; N token(s) blocked for another reason` — never hang.
- Model-writing traps met while building these fixtures: `after` is a reserved keyword (bad node
  name); an accept's bound parameter is only resolvable from other nodes when the owner is written
  `action Foo { … }` rather than `action def Foo { … }` — otherwise the reader body reports
  `unresolved reference: n`; the CLI has **no** flag for action inputs, so exercise `in`/`out`
  parameters by having a caller action invoke `action call = Callee(a = 3, b = 4);`.

## Control flow inside an action node body

`while`, `loop … until` (braced and unbraced), `for … in` and `if`/`else` execute inside an
`action <node> { … }` body. The whole body runs within **one** token step, so `%step` past the node
jumps straight from `Token 1 @ <node>` to the successor with the loop's final values already in the
token data — do not expect one step per iteration. Drive these with the standard shape:

```bash
printf '%%load f.sysml\n%%action tally\n%%continue\n%%quit\n' | timeout 30 ./bin/sysml
```

and assert on the exact numbers in the `Results:` block; "it completed" proves nothing, since the
historical bug (pre-PR #79) was that the statements were *silently dropped at lowering* and the
action still completed, just with the wrong total. The cheapest way to prove a control-flow change
is real is an A/B against a binary built from `main` in a `git worktree` — same model, different
number.

Things that must fail rather than hang: the REPL builds its runtime context with a step budget that
defaults to **10000000** (`runtime.DefaultMaxSteps`, `internal/core/runtime/budget.go`; sessions
carry the five budgets via `Session.SetBudgets(runtime.Budgets)`), and every loop iteration spends
several steps, so a runaway loop (or an empty loop body, whose condition can never change) returns
`error: execution failed: eval … : evaluation step limit exceeded (10000000 steps; raise SYSML_MAX_STEPS to allow more)`
in under a second (0.7 s measured). Always follow the failure with another meta-command (`%tokens`, `%instances`)
to prove the session survived — `%tokens` still shows the token parked at the node with its partial value. A
`for` over a non-collection gives
`action node <n>: 'for' collection must be a sequence or a set, got constant` before PR #202, and
`action node <n>: type mismatch: 'for' iterates a collection, and expression is not one` after it —
and only for a value that states a *computation* (a body expression). See the next section.

### `for` over a scalar is a typed error (PR #231)

Since PR #231 `forElements` (`internal/core/runtime/statements.go`) only iterates a **sequence** or a
**set**; `null` iterates zero times, and *everything else* — Integer, Real, Boolean, String, an
expression — is `type mismatch: 'for' iterates a collection, and <describeValue> is not one`. Exact
texts observed on `bin/sysml`, worth asserting verbatim:

| input | message |
|---|---|
| scalar `Integer` / nested `for` whose inner input is scalar | `error: execution failed: action node <n>: type mismatch: 'for' iterates a collection, and an Integer is not one` |
| `Boolean` | `… and a Boolean is not one` |
| `String` | `… and string is not one` (no article — `describeValue` renders it lowercase) |
| the same `for` in a **calc** body | `error: calc invocation failed: calc <fqn>: calculation body: type mismatch: 'for' iterates a collection, and an Integer is not one` |
| attribute declared with **no value** (`attribute missing : Integer;`) | `error: execution failed: eval 'for' collection: unresolved reference: missing` — the loop input never reaches `forElements`, so do not expect the new wording here |

Testing notes for this class of change:

- The pre-fix contrast is the convincing frame: build the parent commit into `/tmp/old-sysml` and the
  same model **completes** with the counter at `1` (and the scalar calc answers `4` instead of
  erroring) — a silent single iteration, which is exactly what a broken build looks like.
- Zero-iteration inputs must still *complete*: give the fixture an `assign marker := 1;` **after** the
  loops so the `Results:` block proves execution continued (`emptyVisited = 0`, `nullVisited = 0`,
  `marker = 1`), not just that nothing errored. `attribute absent : Integer[0..1] = null;` is the way
  to get a `ValNull` loop input.
- Check the strictness did **not** leak into `elementsOf` (collection operators, multiplicity):
  `%eval SequenceFunctions::size(7)` → `= 1`, `%eval 7->size()` → `= 1`,
  `%eval SequenceFunctions::isEmpty(7)` → `= false`.
- `%calc` takes **positional** arguments only: `%calc P::C 4`. `%calc P::C n=4` answers
  `error: named arguments are not supported here; pass arguments positionally`.
- An entry action nested inside a part is reachable by its qualified path (`%action test::p::a`), which
  is the REPL twin of the conformance harness's qualified `"evaluate": "test::p::a"`.
- After every failing case, run `%eval 1+1` → `= 2` to prove the session survived.

### Block token flows: nested actions and `perform` in a loop body / `if` branch (PR #202)

Before PR #202 a loop body or `if` branch containing an **action node** (a nested `action`
declaration, or a `perform`) aborted the run at lowering:
`action node iterate: action usage "scale" in a body is not executable` /
`'perform' in a body is not executable`. After it, the block becomes a token flow of its own
(`internal/core/lower/block_graph.go`) and runs. That old message is the **ideal A/B contrast** —
build the parent commit into `/tmp/old-sysml` (recipe above) and run the same model through both;
the old binary aborting while the new one prints numbers is far stronger than a screenshot of the
new numbers alone.

Things that cost time when writing models for this area:

- **A `perform Foo;` binds `Foo`'s `in` parameters by NAME from the enclosing scope**, and merges
  its `out` values back under their own names. So `action def Bump { in i; out doubled; }` performed
  inside `for i in 1..3` works because the loop variable is *also* called `i`; rename either side and
  the run dies with `invoke action Bump: … unresolved reference: i` — which reads like a feature bug
  but is just a name mismatch in the fixture. Always name the performed action's inputs after the
  values that exist where it is performed.
- **`perform <an action def>` also emits a pre-existing static diagnostic**
  `error: references target must be a usage, found actionDef` on *every* revision (the shipped
  conformance fixtures do it too) while still executing correctly. Do not report it as a regression;
  check it against the contrast binary first.
- **A nested action's own declarations do not leak outward.** Reading one after the node
  (`assign x := local;`) fails at *name resolution* time, i.e. already at `%load`, with
  `unresolved reference: local` — a cheaper and stronger no-leak assertion than inspecting Results.
  Conversely a value declared *before* the nested action IS readable after it (one block scope), so
  assert both directions.
- **`%trace on` is the best evidence here**: it labels each block-flow node
  (`stmt node scale`, `stmt node deeper`, `stmt node Bump` / `stmt perform`) under
  `iteration N`, so "the perform ran once per iteration" and "the untaken branch ran nothing"
  are directly visible (no `stmt node <else-branch-action>` line at all). Assert zero `ast.`
  substrings in the trace — a Go type name there means a node kind is missing from the label switch.
- The whole block flow still runs inside **one** token step, so `%step` jumps
  `Token 1 @ start` → `@ iterate` (values still at their initial values) → `@ end` (final values).
- A non-terminating `while` that performs an action fails with the **evaluation** step limit
  (`eval assignment RHS: evaluation step limit exceeded (10000000 steps; …)`) in ~4 s, not the
  action step limit — budget ~10 s, and don't assert on which of the five budgets trips.
- To reach the `for`-over-a-non-collection error you need a **body expression**, e.g.
  `attribute notACollection = { in j; j > 1 };` then `for i in notACollection`. Naming a `calc def`
  (`for i in Mk`) gives `unresolved reference: Mk` instead and does not exercise the path.
- **The shipped `*_block_flow_*` / `action_for_over_produced_collections` fixtures emit static
  `unresolved reference` errors when loaded in the REPL** (`select` → "did you mean
  `ControlFunctions::select`?", `Integer` → "did you mean `ScalarValues::Integer`?") because they
  import only what the conformance harness needs. Execution still yields the right values, so this
  is a fixture wart, not a runtime defect — but add the missing imports to your *own* models so the
  recording is clean.

### Raising the budgets (PRs #83, #87)

Five variables, one per runaway bound, each counting a different unit — raising one says nothing
about the others:

| Variable | Default | Counts |
|---|---|---|
| `SYSML_MAX_STEPS` | 10000000 | expression evaluations |
| `SYSML_MAX_ACTION_STEPS` | 1000000 | action token-flow steps |
| `SYSML_MAX_EVENTS` | 1000000 | state machine events, and the events one `%advance` drains |
| `SYSML_MAX_DO_STEPS` | 5000000 | do actions, ditto for `%advance` |
| `SYSML_MAX_ELEMENTS` | 1000000 | collection elements one evaluation holds (~104 MB of `Value`s), the memory bound: `(1..2000000)` reports `collection element limit exceeded`, not the step limit, while a loop building a small collection many times is unaffected |

`%budget` prints the five bounds in force with the variable raising each, so a test can read what a
session runs on instead of inferring it from the environment.

Each bounds **one run** — one `%eval`, `%instantiate`, `%calc`, action or state machine, a stepped-
through run included — not a whole session, so a long session of small operations never runs out; a
run started inside another shares the outer one's budget. Testing the bound therefore needs a single
runaway run, not a sequence of evaluations.

Each overrides the default for both `bin/sysml` and `bin/sysml-grpc`: unset/empty → the default, a
positive integer (whitespace is trimmed) → that value, anything else → the binary refuses to start
(`sysml` exits **2** with `sysml: SYSML_MAX_STEPS="…" is not an integer …` /
`… must be greater than zero …`; `sysml-grpc` exits **1** and logs the same error under
`msg="Invalid service configuration"`). Every unusable value is reported at once, not just the first.
Useful test model — a `while i < N { assign …; assign …; }` body costs roughly 10 evaluation steps
per iteration, so a 100 000-iteration loop completes at the default in 0.13 s and a budget of 100000
stops a 10 000-iteration one:

```bash
SYSML_MAX_STEPS=100000 sh -c "printf '%%action loopn\n%%continue\n%%quit\n' | ./bin/sysml /tmp/loopn.sysml"   # step-limit error
printf '%%action loopn\n%%continue\n%%quit\n' | ./bin/sysml /tmp/loopn.sysml                                  # completes at the default
```

Note the `sh -c` wrapper: `VAR=x cmd | …` only exports to the first process of the pipeline, so put
the whole pipeline inside `sh -c` or the REPL will not see the variable. The gRPC side inherits the
variable through the pysysml auto-start path (the client spawns `~/.pysysml/bin/sysml-grpc` as a
child), so `SYSML_MAX_STEPS=300000 python script.py` is enough — but `pkill -f sysml-grpc` first,
otherwise an already-running service from an earlier value keeps serving.

Tooling trap: running a pysysml script that auto-starts the service from a *non-tty* one-shot shell
tends to return no output at all (the spawned service holds the pipe). Run such scripts with a tty
shell (`tty: true`) or inside the GUI terminal, and use the venv interpreter
(`~/pysysml-venv/bin/python`) — the default `python` in a plain shell has no `pysysml`.
**The venv is not always at `~/pysysml-venv`**: some sessions ship `/home/ubuntu/sysml-venv`
instead, and the system `python` may have a protobuf too new for the generated stubs. Resolve it
once with `ls -d /home/ubuntu/*venv*` and check `<venv>/bin/python -c 'import pysysml'` before
blaming the client. A one-shot non-tty `<venv>/bin/python script.py` did work in practice, so try
it before falling back to a tty shell.

A name declared inside a loop or branch body lives in a block frame and must **not** appear in the
action's `Results:` — check for the *absence* of the line, not just the right total.

## Calc usage semantics: activation snapshots, body-local usages, integer ranges (PR #118)

These four runtime areas are cheap to test from the prompt and each has an unambiguous A/B against
a pre-fix binary, so build the contrast binary from the merge-base first (see above).

- **A calc usage's outputs must all come from one input binding per activation.** The probe is a
  pair of calc defs that differ only in *statement order*: one reads `p.a`, assigns the feature the
  input names, then reads `p.b`; the other reads both outputs before the assignment. Both must give
  the **same** number — the read order is not observable. The pre-fix signature is the interleaved
  one answering from mixed state (e.g. `1020.00` against `1010.00`). `%calc` on each is enough;
  ready-made as `internal/core/runtime/testdata/conformance/calc_usage_outputs_one_binding.sysml`.
  Two assertions must accompany it, since the fix relaxes the memoization key and could over-share:
  a genuine output cycle (`out a = b + 1.0; out b = a + 1.0;`) still has to report
  `cyclic output dependency: output a of calc … depends on itself`, and two usages of one calc def
  with **different** arguments must keep their own values (`p1` at `k=1.0` next to `p2` at `k=5.0`
  → `1005.00`, never `1001` or `5005`).
- **Body-local calc usages inside a `while`/`if`/`for` body** used to fail at
  `calc usage "<name>" in a body is not executable`. The realistic fixture is
  `conformance/calc_rk4_lunar_descent.sysml` (four body-local stage usages `k1..k4` in a `for` body
  plus a nested steering usage): `%calc test::Propagate 3` → `= 15001.72`, and the gRPC side gives
  the full-precision `15001.719185373526` that the `.expected.json` pins. **The REPL prints two
  decimals**, so assert exactness through pysysml `eval`, not the prompt. Keep a two-line `while`
  variant around too — it shows the removed error message on camera, which the RK4 file cannot,
  because on a pre-fix binary RK4 dies earlier on `..`.
- **`..` is the ordered integer sequence the stdlib declares**, not a value kind of its own
  (pre-fix: `unsupported operator: '..': a range is not a value kind the runtime carries`). Five
  prompt evals cover it: `(1..5)->collect { in i; i * i }` → `[1, 4, 9, 16, 25]`, `3..1` → `[]`
  (descending is empty, not an error), `1..2.5` → `type mismatch: '..' requires Integer bounds`,
  `(1..4)->SequenceFunctions::subsequence(2, 3)` → `[2, 3]` (a stdlib function defined *via* a
  range), and `conformance/calc_integer_range.sysml`'s `%calc test::RangeSum 4` → `= 44`, which
  folds `collect`/`select`/`for i in 1..n`/`sum`/`size`/`#(2)` into one number. Loading that
  fixture prints pre-existing tier-2 `unresolved reference: collect` / `select` diagnostics on
  **every** revision — not a regression. Note ranges need a loaded document: on a bare REPL the
  evals answer `no declarations loaded`, which is not the range path at all.
- **A calc usage nested in a part is readable through a feature chain.** `%eval Pkg::lander.mass.mDry`
  and `%eval Pkg::lander.dryMass` (an attribute whose default reads the usage) both answer the
  number; pre-fix both were `usage Pkg::lander has no value`. Reading the usage **without** naming
  an output must name the outputs instead:
  `no value: calc usage mass computes output features (mProp, mDry, mWet); read one of them`.
  The model lives in `internal/core/runtime/part_feature_chain_test.go` as `partChainModel` — copy
  it rather than inventing constants, and take the expected value from that test
  (`mDry = 100.0 + 250.0 * 0.4` → `200.00`); task descriptions quoting other magnitudes usually
  refer to an earlier draft of the model.
- **`pysysml` `Model.find` must accept the FQN it reports**: `m.find("Rhs")` and
  `m.find("test::Rhs")` return the same symbol (`.id == 'test::Rhs'`) and `m.find("test::Missing")`
  is `None`. Remember the package name in the fixture decides the FQN spelling.

Because all five reach the *same* statement engine, feature-chain evaluation and calc memoization,
follow them with the cheap canaries: `%action tally` + `%continue` → `total = 5`, a
`%state Debug::Cycle` + `%advance 1` + `%advance 9` sweep → `working` at `Time: 10.00`, and a gRPC
`get_slot` on `Demo::Vehicle` (`mass` → `materialized=True kind=real_value`, `engine` →
`kind=instance_id`).

## Session-accumulation trap (bites both testers and features)

Whether re-typing a namespace **adds to** it or **replaces** it depends on where the earlier one
came from (`mergeSubmission`, internal/repl/merge.go):

- **Typed earlier at the prompt → merged.** `package Demo { part def Trailer; }` folds into the
  `package Demo` already typed: `note: added to the existing package Demo (its other members are
  kept)`, and `Demo::Vehicle` still resolves. Re-typing a *member* replaces that member and says
  so (`note: added to the existing package Demo, replacing part def Wheel`), which is also what
  drops the instances built from it.
- **Loaded from a file (`%load`) → replaced.** A loaded snippet keeps its identity, so
  `package ActionExecutorDemo { part def Trailer; }` after loading that example reports
  `note: replaced package ActionExecutorDemo (action def SimpleAction, action sequential, action
forkJoin, action conditional no longer declared)` — every member it declared — and the
  file's members are gone. This is the shape that bites a test written against a `%load`ed
  fixture — use a **different package name** to add declarations there.
- An **empty body** (`package Demo { }`) is the deliberate way to empty a namespace, and a
  submission with a different header (or declaring more than one thing) replaces rather than merges.

`Submit` **carries instances over** what a submission did not change (`internal/repl/carryover.go`,
`runtime.Adopt`): after an unrelated `part def B;`, `%instances` still lists the instance with the
**same ID**, `%features` still prints its values, and the next `%instantiate` gets a *fresh* ID rather
than `ID: 1`. What the submission invalidated still goes — redeclaring the instance's own definition,
or a declaration its features are typed by — and then the notice
`note: N instance(s) … dropped because the declarations changed` is expected, with `%instances`
saying so too (`(no instances created; …)`, or `(… also dropped …)` when only some went). An active
`%action`/`%state` debugging session likewise survives an unrelated submission and ends, with a
notice naming the declaration, when what it depends on changes.

Recipes that actually distinguish working from broken here (used to verify PR #168):

- **Survival:** `part def A { attribute x : ScalarValues::Integer = 1; }` + `%instantiate A` +
  `part def B;` → no drop notice, `%instances` → `A (ID: 1)`, `%features A` → `x = 1`. The parent
  commit's binary (see the contrast-binary recipe) prints the drop notice and
  `error: no instance of "A"` on the same input, so run both on camera.
- **Partial loss:** instantiate two definitions, then redeclare only one → `note: 1 instance was
  dropped …` plus `%instances` listing the survivor and
  `(1 instance was also dropped when the declarations changed at submission N — re-run %instantiate)`.
  A `part def D { part t : T; }` instance is dropped by redeclaring `T`, not only `D`.
- **Fresh IDs:** the next `%instantiate` after a survival gets the next unused ID (2, 3, …); an ID
  restarting at 1 while an older instance still lists is the failure signature.
- **Debugger:** `%tokens` is the action-side inspector — `%current` answers
  `no active state machine session` for an action session, which is not a bug. After the session
  ends, `%step`/`%current` report `error: no active … session: … ended when <name> was redeclared at
  submission N`.
- **Performer-based end notice:** to lose the *object* without superseding the behavior, keep them in
  separate packages (`package P { part def Ship { action tally { … } } }`,
  `package Q { part s : P::Ship; }`, `%instantiate Q::s`, `%action P::Ship::tally Q::s`) and then
  resubmit `package Q` with an extra member → `note: action debugging session for "P::Ship::tally"
  ended (the object Q::s performing it was dropped)`. Redeclaring `P` instead names the declaration.
- **Value expressions are re-derived, not invalidated** (as of the `runtime.Adopt` change in
  PR #168): a carried slot whose feature has a value expression (`attribute m = double(3.0);`) is
  reset to unmaterialized, so the new context recomputes it from the declarations the expression
  reads *now*. Redeclaring `calc def double` with `x * 3.0` therefore keeps `ID: 1` and prints **no**
  drop notice, while `%features A` moves `6.00 → 9.00`; the same holds through chains
  (`outer` calling `inner`: `7.00 → 10.00`; `attribute h = g * 2.0` read by `m`: `11.00 → 15.00`).
  The assertion that catches the earlier stale-value bug is **`%eval` must agree with `%features`** after
  every such change — a `%features` that keeps the old number while `%eval` of the same expression
  returns the new one is the failure signature. Composite parts keep their carried values *and*
  nested instance IDs (`w = Instance(ID: 2)`). Drops are still expected for the instance's own
  redeclared definition and for a change to a declaration one of its features is typed by.
- **What is read again vs kept on carry-over** (`adopt.go` `derivedSlot`/`connectorSlot`/
  `collectedSlot`, as of 4947ca3 + 65b04ec). Four distinct classes, each with its own recipe:
  - *Connector slots are read again under the same identity.* `connection c1 connect a.x to b.y;`
    where `a.x = double(3.0)`: after redeclaring `double` with `x * 3.0`, `%features` must show
    `x = 9.00` **and** `c1 = Instance(ID: 4)` (unchanged) **and** `source = 9.00`. A `source` that
    stays `6.00` next to `x = 9.00` is the bug. This applies to named *and* anonymous connectors.
  - *A variation's selected variant is carried, not derived* — its default names a variant rather
    than a value. `part :>> engine = engine::electric;` must keep `electric (Instance ID: 2)` across
    an unrelated `part def Widget;`; an id that moves (2 → 3) means the variant slot was wrongly
    treated as derived. Changing the selection to `engine::petrol` must still drop with the notice.
  - *A collection of scalars is collected again* (its members are copies of what the subsetting
    features hold): `attribute pool : Real[*]; attribute one :> pool = double(3.0);` must go
    `pool = [6.00] / one = 6.00` → `pool = [9.00] / one = 9.00`. `pool` stuck at `[6.00]` while
    `one = 9.00` is the failure — the object then reads two values for one thing.
  - *A collection of objects is kept*, since those objects carry over under their identities:
    `part xs : B[3]` must print the identical `[Instance(ID: 2), Instance(ID: 3), Instance(ID: 4)]`
    after an unrelated submission, and the next `%instantiate` must not reuse 2/3/4.
- **A stale carried value only shows if the slot was materialized before the change.** These
  carry-over slot bugs need a `%features` (or other read) *between* `%instantiate` and the redeclaration:
  without it the slot was never materialized, so there is nothing stale to keep and even a broken
  build prints the right number. Contrast runs against a binary built from the parent commit
  (`git worktree add /tmp/wt-old <parent> && go build -o /tmp/old-sysml ./cmd/sysml`) are the cheapest
  way to prove a case actually discriminates — but include that intermediate read in both runs.
  Conversely, the *anonymous* connector-id case must also be checked with **no** read in between
  (two unrelated submissions back to back, then one `%features`), which is a separate code path.
- **Never put a literal TAB in piped REPL input** (`printf '…\t…' | ./bin/sysml`): readline enters
  completion mode and the process dies with `panic: bytes: negative Repeat count`. Use spaces in
  one-line rehearsal snippets.

Also: analysis still runs over the whole accumulated buffer, but since PR #65 the **report** is
scoped to the submission just made, so one bad snippet no longer keeps re-printing its error on
later submissions. Two consequences when testing:

- Reported line/column numbers are **relative to what you just typed** (`Result.Offset` /
  `baseLine()` in `internal/repl/render.go`), so a one-line submission reports `1:36:` no matter how
  much is already in the buffer. Only `%verbosity debug` numbers against the whole buffer.
- While an earlier error is unresolved, the next clean submission prints
  `note: deeper checks may not have run here: the error on buffer line N is unresolved (see it with
  -debug)` before its summary. That note is expected, not a failure — and it is printed **once**, not
  on every later submission, so its absence on the ones after is also expected.

Still true: accidentally typing a shell command like `clear` at the `>` prompt is parsed as SysML
and pollutes the buffer. `%clear` resets the session.

## The session-long symbol index and wildcard re-exports (PR #95)

`Session.symbolIndex()` (`internal/repl/session.go`) keeps **one** `symbols.Index` for the whole
session: the stdlib is loaded into it once (`model.LoadStdlibInto`) and only the session document is
re-indexed, when `doc.Version` changed. So stale/duplicated symbols are the failure mode to hunt,
and every assertion should be re-checked *late* in a long session, not only on the first submission.

The re-export cycle is the highest-value test, and it distinguishes working from broken in three
steps (a REPL built before this work fails at the very first one):

```
package Lib { part def Widget { attribute size = 3.0; } }
package P { public import Lib::*; }
%instantiate P::Widget      -> ✓ Created instance of Lib::Widget    (resolved FQN is the DECLARING one)
package P { }               -> replaces the earlier snippet (same declared name)
%instantiate P::Widget      -> error: unresolved reference: P::Widget  (re-export unwound)
%instantiate Lib::Widget    -> ✓ Created instance of Lib::Widget    (purge must not over-remove)
package P { public import Lib::*; }
%instantiate P::Widget      -> ✓ ... again, and never "is ambiguous"
```

Notes that save time:

- A re-export registers the symbol under the importing namespace but **never copies its subtree**:
  `P::Widget` resolves while `%eval P::Widget::size` is `unresolved reference: …`. Use the declaring
  FQN (`%eval Lib::Widget::size` → `= 3.00`). This is intended (confirmed by the maintainer).
- Resubmitting the same importing package several times must keep resolving and must never produce
  `is ambiguous` — duplicate re-export registrations would surface as ambiguity, so assert the
  negative explicitly.
- `%clear` drops the document from the index (`idx.RemoveDocument`) but **keeps the loaded library**,
  so `%clear` followed by more work exercises the removal path directly: expect
  `error: no declarations loaded (literals work, but feature references need declarations)` (or
  `error: runtime init: no document loaded` for `%instantiate`) for the old names, then full
  wildcard expansion again for freshly submitted packages.
- Because the REPL has a **single** document, a re-index removes and re-adds all of its re-exports
  wholesale. Index bugs that need one document's member to change while a *different* importing
  document survives are therefore not reachable from the prompt — verify those in
  `internal/core/symbols` tests instead of hunting them at the REPL. In particular a top-level
  (outside any `package`) `import Lib::*;` followed by a submission that drops the surfaced
  declaration still correctly reports `not found` at the prompt.
- Stdlib staleness check: quantity/unit evaluation resolves its unit through the session index, so it
  is the cheapest canary. `package Q1 { import SI::*; attribute speed = 1.5 [m/s]; }` then
  `%eval Q1::speed` → `= 1.5 [m/s]`, and re-running that exact command after each of ~15 further
  submissions must print the identical string every time. Without `import SI::*;` the unit is
  `error: evaluation failed: not a quantity expression: not a measurement unit: unresolved unit m`,
  which is a fixture mistake, not a regression.
- Pre-existing noise not caused by index work: ISQ-typed attributes
  (`attribute m : MassValue = 12.0;`) emit `cannot bind Rational value to a feature typed by
  ISQBase::MassValue` and a top-level `import Lib::*;` typed before `Lib` exists emits
  `unresolved reference: Lib`. Both reproduce on `main`; the lookups still succeed. A/B against a
  `main` binary in a `git worktree` before reporting any diagnostic as new.

## Output modes and execution tracing (PR #65)

Two independent axes — diagnostics verbosity and execution tracing. Don't conflate them.

```bash
./bin/sysml -quiet          # errors only (suppresses warnings)
./bin/sysml                 # default: errors + warnings + summary
./bin/sysml -debug          # whole-buffer diagnostics, absolute lines, pass names
./bin/sysml -trace          # execution tracing on from the start
./bin/sysml -debug -quiet   # rejected: stderr message, exit 2
```

Prompt equivalents: `%verbosity [quiet|normal|debug]` and `%trace [on|off]`; with no argument each
**reports** the current setting (`verbosity: normal`, `trace: off`). A bad argument prints guidance
(`error: unknown verbosity "bogus" (want quiet, normal or debug)`) and leaves the session usable.

To prove quiet actually suppresses something you need a **warning**, which is easy to trigger with a
mismatched comparison: `attribute flag = 1 == "a";` yields
`warning: comparing Natural with String is always false`. The strongest test is an A/B in one
session — submit it at `normal`, then `%verbosity quiet`, then submit an equivalent snippet under a
different package name and show the warning is gone while the summary line remains.

`%verbosity debug` output starts with
`[debug] submission at buffer line N; M diagnostic(s) over the whole buffer` and tags each
diagnostic with the pass that produced it (`[syntax/syntax]`, `[type/type.expr]`).

Tracing prefixes every recorded line with `[trace] `. Evaluation entries are **post-order and
indented**: sub-expressions appear before, and one level deeper than, the expression that consumed
them (`internal/core/runtime/trace.go`). `%features` on a model with derived attributes is the easiest
way to see a full tree — `derived_package.sysml` gives `eval operator * -> 3000.0` and a nested
`eval feature power -> 300.0` / `eval operator * -> 270.0` / `eval operator + -> 1770.0`.

The recorder is **drained per command** (`drainTrace` in `internal/repl/trace.go`), so each command
reports only its own steps. Always assert the negative too: run an unrelated command such as
`%instances` straight afterwards and confirm it prints **no** `[trace]` lines — a recorder that is
not cleared would replay the previous command's tree.

### Regression watch: traces during a debugging session

`ActionExecutor.trace()` reads the recorder off `e.ctx` rather than caching it on the executor
(`internal/core/runtime/action_executor.go`). That is what makes `%trace on` reach an execution
already under way, and what makes **expression** traces appear alongside step traces. The
historically broken sequence, worth re-running after any change in this area:

1. `%load internal/repl/testdata/action_debug.sysml`, `%action tally`
2. submit an unrelated declaration, e.g. `package Unrelated { part def Widget { attribute size = 1.0; } }`
3. `%trace on`, then `%step` repeatedly

Expect **both** `[trace] step N: token 1@accumulate` **and** `[trace] eval feature total -> 0` /
`eval literal 5 -> 5` / `eval operator + -> 5`. Bare step lines with no `eval` lines is the bug
signature. Then `%trace off` must silence output in that same session.

Control nodes are named by what they do, not by Go type: an unnamed fork/join/final reports
`token 1@fork` / `@join` / `@final` (`nodeIdentifier` in `internal/core/runtime/trace.go`). A `*ast.`
type name in trace output means a node kind is missing from that switch.

## Spot-checking the docs against the binary

`docs/guide/` and `README.md` contain REPL transcripts that are easy to let rot. Verify them
by **typing them by hand** at the prompt in a GUI terminal, not over a pipe: some failure modes (a
blank line inside a braced declaration ending the submission early) only exist interactively.
Discover the expected values over a pipe first, then do one clean recorded pass.

As of PR #107 the guide and README action- and state-debugging transcripts are real captured output
and match the binary verbatim, including the hint lines
(`Use %step to advance, %tokens to inspect, %continue to run to completion`,
`Use %tokens to inspect, %step or %continue to resume`,
`Use %events to see queue, %current for state, %advance <time> to step`) and the `✓ <kind> <name>`
echoes (`✓ calc distance`, not `✓ distance`). Treat a mismatch there as a real doc regression now.

Traps worth re-checking after any doc or REPL edit:

- **A blank line inside braces ends the submission.** A transcript that shows one is un-typable; the
  remainder arrives as a separate submission and produces diagnostics. Assert the whole declaration
  comes back as a single `✓ state X` with zero diagnostics.
- **`%save` over an existing file appends `(replaced the existing file)`.** A doc block whose earlier
  step created or loaded that same file must show the suffix. Byte counts are exact and
  content-dependent (`saved 181 bytes of sysml` / `saved 1872 bytes of ttl` for the
  `MyModel` file of guide chapter 7) — recompute them whenever the sample model changes.
- **Snippets using `Real` need `import ScalarValues::*;` in the session**, or the submission is
  rejected with `error: unresolved reference: Real` and no `✓` echo. A snippet relying on an import
  made in an earlier, separate doc section fails for a reader who starts a fresh REPL there.
- The binary's continuation prompt is `  ...>` (two leading spaces); some doc blocks write `...>`.
- Konsole ligatures render `<=` as `≤` on screen — cosmetic, not a REPL difference.
- Typing `clear` at the `sysml>` prompt pollutes the buffer (see the session-accumulation trap);
  restart the REPL rather than trying to recover mid-transcript.

## The gRPC service and the `pysysml` Python client

The REPL is not the only user-facing surface: `cmd/sysml-grpc` plus `python/pysysml` is the path a
Python user takes, and the two can disagree. When a change touches `internal/grpc/convert.go` or
the runtime's slot evaluation, **test both and diff them** — that comparison is the highest-value
assertion available.

```bash
export PATH=/usr/local/go/bin:$PATH
make build && make build-grpc              # -> bin/sysml, bin/sysml-grpc
mkdir -p ~/.pysysml/bin && cp bin/sysml-grpc ~/.pysysml/bin/   # where the client looks
pip install -e python/
```

Do **not** start the service by hand. `Connection._ensure_service`
(`python/pysysml/connection.py`) probes `localhost:50051` and spawns the binary itself; letting it
auto-start is both the realistic user path and the only way it writes its pidfile. Attaching to a
service you started yourself makes `test_lifecycle.py::test_service_shuts_down_when_last_process_exits`
fail — that is the documented `docs/project/roadmap.md` §P1 gap, not a new bug.

### Driving the service on an ephemeral port (process-lifecycle work, PR #249)

When the thing under test is the **process** (flags, exit status, graceful shutdown, leaks) rather
than the model semantics, a hand-started service on `-port 0` is the right harness and
`pysysml.connect(host, port, auto_start=False)` attaches to it without touching the pidfile
machinery. Pass `-health-port` too: it defaults to 8081 and collides with an already-running
service. **Never pipe a command whose exit code you are asserting** — `… | tail` reports `tail`'s
status, which has produced a false pass on this very exit-code matrix; run it bare and echo `$?`.
And prove "never listened" with `ss -ltn`, since an exit code alone does not. Two more traps cost
real time:

- `-port 0` makes the *resolved* port observable only in the log line
  `msg="gRPC server listening" addr=[::]:<port>`. The `-health-port 0` line logs the literal
  `addr=:0`, so a naive `grep -o 'addr=[^ ]*' | head -1` grabs the health line and you dial port 0
  ("Connection refused"). Always filter on `gRPC server listening` first, and take the port with
  `${ADDR##*:}` — `cut -d: -f3` yields `]` for `[::]:41325`.
- Expected values at 0cf94e80 for `internal/grpc/testdata/conformance/instantiate_derived_slot.sysml`:
  `mass` → `materialized=True kind=real_value 1500.0`, `doubled` → `real_value 3000.0`; a missing
  model path raises `pysysml.errors.ModelFileNotFoundError` ("file not found: open …") and the
  server logs `code = NotFound` for `/sysml.SysMLService/ParseFile` while staying alive. An already
  occupied `-port` exits **1** with `msg="Failed to listen" port=<port>`; `kill -TERM` exits **0**
  after `Shutting down gracefully...` / `gRPC server stopped`. `pgrep -x sysml-grpc` (never `-f`)
  before and after is the leak check. Rest of the matrix: an unknown flag exits **2**;
  `-cache-size 0` exits **1** with `cache maxSize must be positive`, raised in `NewService` *before*
  `net.Listen`; a bogus `model_hash` is `NOT_FOUND` / `ModelNotFoundError`. Shutdown with a client
  channel still open exits 0 and makes the client's next call raise `UNAVAILABLE`
  (`pysysml.ConnectionError`) — bound that call with a timeout so a hang fails instead of hanging
  the run.

<a id="venv-trap"></a>
**Python interpreter trap on this box** (bites every pysysml section below): whatever `python3`
resolves to in a tool shell may be another project's venv, and a venv built from it gets a
mismatched `sys.path` — `pyvenv.cfg` naming one minor version while `bin/python` runs another, so
the editable install lands in a `site-packages` the interpreter never searches and `import pysysml`
(or `import grpc`) fails right after a *successful* `pip install -e python/`. Always build the venv
from an explicit real interpreter (`/home/ubuntu/.pyenv/versions/3.12.8/bin/python3.12 -m venv ~/pv`,
or `/usr/bin/python3.10`) and verify `<venv>/bin/python -c 'import pysysml'` before blaming the
client. `$HOME/pv` is created by the blueprint, so prefer reusing it.

### Service lifecycle, the stale-service check and `require_capabilities` (PR #181)

Since PR #181 `Connection` interrogates whatever is *already* listening (`GetServerInfo`) and
compares it against the release asked for (`connect(version=...)` or `$PYSYSML_GRPC_VERSION`) plus
`require_capabilities=[...]`; a mismatch raises `pysysml.StaleServiceError`. To test that surface
you **do** need a hand-started service — that is precisely the "foreign process" case:

```bash
./bin/sysml-grpc -port 50099 &                   # a port other than 50051 keeps the auto-start
                                                 # tests independent
PYSYSML_GRPC_VERSION=v0.0.7 python -c '...connect(port=50099)...'   # -> StaleServiceError
```

- Ownership is decided in `_started_by_this_client`: `~/.pysysml/sysml-grpc.pid` must name the
  process **and** its cmdline must end `['-port', str(port)]`. A hand-started service has no
  pidfile, so it must be reported and left running — assert `psutil.pid_exists(pid)` *and* that a
  subsequent connect still serves (`model.instantiate(...)`), not just that an exception was raised.
- A locally built binary reports the **commit** as its version (`version e695687`), not a `vX.Y.Z`
  tag, so any `PYSYSML_GRPC_VERSION=v0.0.x` is a mismatch — handy, and it also means asking for a
  tag while a dev build runs will *always* raise.
- Capability names the service reports today: `convert`, `query`, `type_facts`, `verification`.
  A bogus `require_capabilities=['time_travel']` surfaces as `MissingCapabilityError` whoever
  started the service (only a release mismatch is a `StaleServiceError`), and it resolves in
  <0.2 s — time the run so a hang is visible as a number.
- With `auto_start=False` the release check is lazy: a service that was not listening when the
  client was built is checked at the first call of *any* kind, so assert it through `conn.load(...)`
  and not only through `conn.server_info()`.
- `pysysml` has **no module-level `load_from_content`**; use `conn.load_from_content(...)`.
- Lifecycle state is per-port: `~/.pysysml/sysml-grpc-<port>.pid` (a JSON ownership record whose
  `refs` is the in-process holder count) and `~/.pysysml/sysml-grpc-<port>.lock`. There is no
  `sysml-grpc.refcount` file. Reset a clean auto-start state with
  `pkill -x sysml-grpc; rm -f ~/.pysysml/sysml-grpc-50051.pid ~/.pysysml/sysml-grpc-50051.lock`
  (`-x`, never `-f`, which matches your own shell — see the pkill trap below).
  Refcount behaviour worth asserting both within one process (two `connect()`s) and across two
  processes: 1 → 2 → 1, service still serving the remaining holder, and the pidfile removed
  only when the last one closes.
- The known-failing pair with a service on :50051
  (`test_integration.py::…::test_load_nonexistent_file_real_server`,
  `test_lifecycle.py::…::test_service_shuts_down_when_last_process_exits`) reproduces on `main`;
  run `pytest tests/ -q` from `python/` both with and without a listener so you can tell a new
  failure from those two.

Client API shapes that are easy to get wrong:

- `Model.find(name)` returns **one `Symbol` or `None`**, not a list — iterating it raises
  `TypeError: 'Symbol' object is not iterable`. Use `.id` (FQN), `.name`, `.kind`.
- `pysysml.instantiate(fqn, file_path=...)` and `pysysml.evaluate(expr, file_path=..., context_symbol_id=...)`
  each take *exactly one* of `file_path` / `model_hash`.
- `Instance.get_slot(name)` returns the raw protobuf `SlotValue`. Read it as
  `sv.materialized` and `sv.value.WhichOneof('kind')` → `real_value` / `int_value` / `instance_id` /
  `null`. Printing the `Instance` alone hides exactly the detail under test.

What the slot kinds mean (`ValueToProto`, `convert.go`):

- A **derived** attribute (`attribute doubled = mass * 2.0;`) must arrive as
  `materialized=True kind=real_value`. `kind=null value='unsupported'` is the pre-fix signature.
- `null: 'unsupported'` is the generic fallback arm for a slot `GetSlot` returned **unmaterialized
  without an error** — a feature with no default and no composite type. Both a bare
  `attribute d : Real;` and a **constraint usage** land here, so the REPL's
  `massOK: <constraint: satisfied>` has no gRPC equivalent. Check whether that divergence is
  intended before filing it.
- `SlotValue.error` is the real error arm (the value is left unset). Force it with cyclic derived
  attributes (`attribute a = b + 1.0; attribute b = a + 1.0;`) — expect
  `slot Loop.a: slot Loop.b: cyclic slot dependency: Loop.a`, promptly, raised as `SlotError` by
  the client, and prove the service is still alive afterwards with a follow-up
  `pysysml.evaluate('1 + 1', ...)`.
- A nested `part engine : Engine;` still marshals as bare `instance_id=N`, but
  `InstantiateResponse.instances` carries every instance reachable from the root, so Python
  expands the child too (`inst.engine.power`). An id is only resolvable against the response that
  carried it — runtime instances do not survive the request.

`execute_action` is the gRPC twin of the REPL's `%action` + `%continue`, and it is the cheapest way
to A/B the two surfaces on the same model. The call shapes are **not** the ones the docstrings
suggest: there is no `parse_file` on `Connection` — use `c.load_from_content(src)` (or `c.load(path)`),
which returns a `Model` whose hash is `model.hash` (not `.model_hash`). Then
`c.execute_action("Pkg::action", model.hash)` returns a plain `{name: value}` dict, and a runtime
failure raises `pysysml.errors.RuntimeError` with the executor's message
(`action execution failed: execute action: …`). If you must attach to a service you started
yourself, `pysysml.connect("localhost", 50551, auto_start=False)` avoids the
`Binary not found at ~/.pysysml/bin/sysml-grpc` error — but prefer the auto-start path per above.

Fixture trap when proving `Model.execute_action` / `Model.execute_state` "exist and work": an action
that only declares parameters (`action add : Add { in x = 2.0; out z = x + y; }`) validates clean but
raises `ExecutionError: initialize action: no initial node found in action add` — there is nothing to
execute. Do not read that as a broken RPC; borrow a body-bearing fixture instead, e.g.
`internal/core/runtime/testdata/conformance/action_body_local_calc_usage.sysml`
(`execute_action("test::run")` → `{'v': 2.0, 'i': 3, 'doubled': 4.0, 'acc': 12.0}`) and
`state_anonymous_action_body.sysml`
(`execute_state("Test::Bodies")` → `states_visited ['start','working','nstart','nested','done']`,
`final_context {'log': 321879}`). Those two doubles are the cheapest end-to-end proof that both
RPCs really run rather than merely being present on the object.

Suite baseline: `cd python && python -m pytest tests/ -q` with no service running should be
`148 passed, 24 skipped` (~40s; it was `75 passed, 18 skipped` before the Tier 1/Tier 2 client
work), and `158 passed, 14 skipped` with a service running. As of the 0.0.8 prep branch
(`b0f5f23`) that baseline is `368 passed, 26 skipped` in ~42 s from the repo root
(`python -m pytest python/tests/ -q`), with one expected `UserWarning` from
`test_a_cache_survives_a_replacement_that_cannot_be_downloaded` — re-measure rather than trusting an
older count. The skips are the integration
tests gating on a live service. `pytest` is **not**
installed in `~/pysysml-venv` by default — `~/pysysml-venv/bin/pip install pytest` first, or
`python -m pytest` fails with `No module named pytest` (the `pytest` on `PATH` belongs to an
unrelated venv and cannot import `pysysml`).

A test that skips with no service and fails with one is never actually green — treat it as a
reportable defect, not a known gap. Two lifecycle traps caused exactly that: a
`Connection(auto_start=False)` used to release a refcount it never took (fixed), and any test
that shells out to a pysysml subprocess lets that subprocess's exit decrement the shared
refcount, so such tests must isolate `HOME` for the child.

Liveness check: after `test_lifecycle` runs, `pgrep -af sysml-grpc` still lists a `<defunct>`
zombie, so it lies. Use `ss -ltn | grep 50051` to decide whether a service is really listening.
`pkill -9 -f sysml-grpc` matches your own shell's command line — use `pkill -9 -x sysml-grpc`.
A full-suite run stops even a service another process owns, leaving a stale
`~/.pysysml/sysml-grpc-<port>.pid`; clear it before the next liveness test.
To hold a service alive for a whole test run, keep a client process open, e.g.
`(setsid python -c "import pysysml,time; pysysml.connect(); time.sleep(300)" &)` — a plain
backgrounded `python -c` from a non-tty shell may exit before it prints, so verify the port.

Download paths (`python/pysysml/binary.py`) are testable without a real release: move
`~/.pysysml/bin/sysml-grpc` aside, unset `PYSYSML_GRPC_VERSION`, and call `ensure_binary()`,
`resolve_latest_version()`, `download_binary('latest')`. All three must raise `ConnectionError`
naming the path or URL. `PYSYSML_GITHUB_REPO` overrides the repo. Beware: these hit the
unauthenticated GitHub API, so repeated runs flip from a truthful `HTTP Error 404: Not Found` to a
misleading `HTTP Error 403: rate limit exceeded` — rehearse sparingly and report the 404 wording,
not whichever one the recording happened to catch.

#### The stale-*cache* decision is testable offline (PR #178)

`stale_cache_reason(version, github_repo=None)` decides whether `~/.pysysml/bin/sysml-grpc` may
answer for a requested release, and it needs no network — drive it directly and write the sidecar
`~/.pysysml/bin/sysml-grpc.json` (`{"version":…, "sha256":…, "repo":…}`) by hand. The four shapes
worth asserting, with the wording each produces:

| sidecar | asked for | reason |
|---|---|---|
| absent | `v0.0.8` | `… was not downloaded by this client, so which release it is cannot be told` |
| `v0.0.7` + **true** sha256 of the file | `v0.0.8` | `… is v0.0.7, but v0.0.8 was asked for` |
| `v0.0.7` + wrong sha256 (hand-swapped binary) | `v0.0.8` | falls back to the "not downloaded by this client" wording |
| `v0.0.8` but `"repo":"someone/OpenSysML-fork"` | `v0.0.8` | `… was downloaded from someone/OpenSysML-fork, but v0.0.8 of Open-MBEE/OpenSysML was asked for` |

- The digest is re-verified (`cached_release`), so a *true* sha256 in the sidecar is what makes the
  "is v0.0.7" branch reachable — a placeholder digest silently tests the wrong branch.
- `stale_cache_reason(None)` is `None` **by design**: with `$PYSYSML_GRPC_VERSION` unset any cached
  binary is taken on faith. So before any Python check, `cp bin/sysml-grpc ~/.pysysml/bin/` —
  otherwise a binary from an older release answers as if it were your build, and
  `~/.pysysml/bin/sysml-grpc -version` is the one-line way to confirm which build you are testing.
- `ensure_binary(version=…)` on a stale cache emits `UserWarning: Replacing the cached sysml-grpc: …`
  and then, when the download fails (403/404 in a sandbox), a second
  `UserWarning: Keeping the cached sysml-grpc … could not be downloaded` and returns the old path.
  That is the only observable half of the replacement without network; say so rather than claiming
  the replacement was proven. Always back up the cache + sidecar and restore them afterwards.

#### Service start-up timing and its failure paths (PR #250)

`Connection._ensure_service` probes the service it spawns immediately and then backs off
(`START_PROBE_INITIAL_DELAY` 10 ms, doubling to `START_PROBE_MAX_DELAY` 250 ms) until
`START_TIMEOUT` (2.5 s). Timing claims here need a **contrast run against the parent revision**,
which needs no rebuild since pysysml is pure Python: `git worktree add /tmp/mainwt main`, copy the
generated `python/pysysml/proto/*.py` in if they are missing, then run the same script twice, once
plain and once with `PYTHONPATH=/tmp/mainwt/python`, on the *same* `$HOME/pv` venv. Numbers seen at
c590253e on a free port with nothing listening: **21 ms on the branch vs 515 ms on main**; the
connection must then really work (`conn.load_from_content(...)` + `Model.eval('1 + 1') == 2`), since
a fast connect to a dead service would look the same.

Recipes for the failure paths, all with a port of their own so the :50051 tests stay independent:

- **A binary that exits at once** — point `$HOME` at a throwaway dir holding
  `.pysysml/bin/sysml-grpc` = `#!/bin/sh\nexit 3`. `get_binary_path()` is hard-coded to
  `~/.pysysml/bin`, so `$HOME` is the only injection point (there is no `PYSYSML_GRPC_BINARY`);
  `PYSYSML_STATE_DIR` moves the pid/lock files only. Expect
  `ConnectionError: Service exited with code 3 without serving localhost:<port>` in ~0.02 s.
- **A port that accepts TCP but never speaks gRPC** — `nc -l` is wrong: it exits after the first
  connection closes, the spawned service then binds the port and the test silently *passes*. Use a
  listener that stays up and never answers (bind + `listen(64)` + accept into a list), and pair it
  with a fake binary that does not exit (`exec sleep 120`) if you want the `START_TIMEOUT` branch
  rather than the "could not bind" one.
  **`START_TIMEOUT` bounds the sleeping, not the wall clock**: each probe may spend its RPC timeout
  (pre-spawn probe 5 s, later ones `START_PROBE_RPC_TIMEOUT` 2 s), so the observed failure is
  `ConnectionError: Service failed to start within 2.5s` after **~9 s** (3 probes) on the branch
  against ~17.5 s (6 probes) on main. Do not assert ~2.5 s wall time; assert the message, a
  single-digit probe count and the improvement over main. Count probes by wrapping
  `Connection._probe_service` with a timestamping function — that is also the cheapest proof of "no
  busy-spin".
- **A replacement that could not bind** (the `_wait_for_a_service_that_could_not_bind` /
  `START_CONFIRM_DELAY` path) — start a real service outside pysysml on the port, then push the
  client past the adopt step the way a release mismatch does — either
  `Connection._adopt_running_service = lambda self: False`, or without monkeypatching, ask for a
  release the running service cannot be: `pysysml.connect(port=P, version="v9.9.9")` (there is no
  `PYSYSML_REQUIRE_VERSION` env knob). Expect `StaleServiceError` ("the service
  started here exited (1) while another one kept serving the address"), **no** ownership record
  (`~/.pysysml/sysml-grpc-<port>.pid` absent, `_OWNED_SERVICES` empty), the foreign pid still alive
  and a follow-up `Connection(auto_start=False)` still evaluating. Asserting only "an exception was
  raised" would miss the real regression risk, which is silently adopting a service this client did
  not start and killing it on close.
- **Ownership under a race** — two `python /tmp/x.py <port>` processes started together: exactly one
  prints `_holds_refcount=True` with `{'refs': 1}`, the other reports
  `origin=service already listening …`; exactly one `sysml-grpc` runs, and it stops when the owner
  exits. Pair it with a test that closes an *adopted* connection and asserts the hand-started
  service is still serving.

### Verification RPCs, typed errors and strict loading (`pysysml` Tier 3, PR #149)

The verification questions the REPL answers with `%constraint`, `%requirement`, `%satisfy` and
`%calc` are also RPCs (`internal/grpc/verify.go`), wrapped as `Model.verify_constraint /
verify_requirement / verify_satisfaction / satisfied / calc`. Testing them from Python:

- Use a **clean venv** — the box's default `python3` may carry an incompatible `protobuf`, which
  fails at `import pysysml`. A venv with `pip install -e python/` (e.g. `~/pv`) is the reliable
  interpreter; rebuild with `make build-grpc` and **re-copy** `bin/sysml-grpc` to
  `~/.pysysml/bin/` after every rebuild or the client silently auto-starts the old binary.
- Argument order bites: `Connection.eval(expression, model_hash)`,
  `Connection.instantiate(symbol_id, model_hash)`, `Connection.verify_constraint(symbol_id,
  model_hash, subject_symbol_id=…)`, `Connection.calc(symbol_id, model_hash, arguments=[…])` —
  hash **second**. On `Model` the hash is implicit and the kwarg is `subject=`. There is no
  `Connection.evaluate`. Getting the order wrong yields a confusing
  `ModelNotFoundError: model not found: Demo::sedan`.
- `Instance.slots` is a **property** (a dict), not a method; unmaterialized slots (constraint and
  requirement usages) appear as `SlotError` values inside it, which is expected.
- The three-way semantics worth asserting separately: a condition evaluating **false** is a
  verdict (`holds False`, `.condition` set, `.error == ''`, `raise_for_error()` silent); a
  **failure to evaluate** is `.error` non-empty with `.evaluated False` and
  `raise_for_error()` raising `ExecutionError`; an **unanswerable request** (unknown symbol)
  raises `ExecutionError` from the call. Force the middle case with a requirement whose
  attribute is never bound (`requirement loose : SpeedRequirement;` with `maxSpeed` unbound) —
  the error reads `no value for feature maxSpeed`.
- A subject of an unrelated type is *not* rejected: `verify_constraint(c, subject=<an attribute
  or a calc>)` instantiates it and answers from declared defaults. If you need a raising subject,
  use a name that does not exist (`unresolved reference: …`).
- `verify_satisfaction(fqn)` narrowed to an element that states no assertion returns an **empty
  list**, so `satisfied()` is vacuously `True`; the `"<x> states no satisfaction assertion"`
  error branch in `verify.go` only fires for a scope-less symbol and is hard to reach.
- Each verdict from `verify_satisfaction()` carries the **whole response's** instance graph, so
  `verdict.instances` includes other assertions' subjects — filter on `verdict.instance_id`.
- Model-cache eviction is testable for real (the service caches 100 models, `-cache-size`):
  load a model, then `load_from_content` 120 throwaway packages, then call any RPC with the old
  hash → `ModelNotFoundError`. Takes ~1 minute; run it in the background.
- Capability negotiation against an "old" service can be simulated without an old binary:
  `conn._server_info = ServerInfo(version='old', capabilities=frozenset({'convert'}),
  answered=True, origin='simulated')` → the verify calls raise `MissingCapabilityError` before
  any RPC. Label it as simulated in the report.
- gRPC status translation lives in `pysysml/errors.py`: assert both the pysysml class and the
  builtin it also is (`ModelFileNotFoundError`/`FileNotFoundError`,
  `InvalidRequestError`/`ValueError`, `ConnectionError`/`ConnectionError`,
  `ExecutionError`/`RuntimeError`) and that `__cause__` is the original `grpc.RpcError`. A dead
  service is reproducible with `pysysml.connect(port=50123, auto_start=False)`.

### The prewarmed library-index pool, `SYSML_GRPC_INDEX_POOL` (PR #252)

`internal/grpc/libindex.go` keeps N standard library indexes built ahead of the requests that need
them (default 4; `0` restores the old per-cache-miss build). How to observe it end to end, and what
generalizes to any service-side perf change:

- **Measure with DISTINCT model texts.** The service caches models by content, so a repeated model
  is a cache hit and shows nothing. Append a unique trailing comment (`// distinct model %d`) per
  iteration and time `conn.load_from_content(src)` client-side; a library-backed model (imports
  `ScalarValues`/`ISQ`, a derived attribute) is required, otherwise no library index is needed.
- Numbers observed at 607b0eb8 on a ~85-line model, 12 distinct models: **pool default (4) median
  4.4 ms**, `SYSML_GRPC_INDEX_POOL=0` **median 112.5 ms**. Expect 1–2 spikes of ~140–155 ms in the
  pooled run: a tight client loop drains the 4 warm indexes faster than the background refill
  (~100 ms per index), and the drained request builds inline by design. Report the median plus the
  spikes rather than the mean, which the spikes dominate.
- **The env var reaches the service through the pysysml auto-start path** (the client spawns the
  child, so it inherits the env), so `SYSML_GRPC_INDEX_POOL=0 python sweep.py` is enough — but
  `pkill -x sysml-grpc; rm -f ~/.pysysml/sysml-grpc-50051.pid ~/.pysysml/sysml-grpc-50051.lock`
  first, or you keep measuring the previously spawned service's configuration.
- **Equivalence is the assertion that catches a wrong index.** Have one script print a sorted JSON
  blob (diagnostics, `find()` id/kind, `eval`, instantiate slot kind+value, `execute_action`,
  `execute_state`) and `diff` the pool=4 and pool=0 runs: only the line naming the configuration may
  differ. A pool that shared an index between models would show up here, not in the timings.
- Bad values are rejected in `NewService`, so `sysml-grpc` **exits 1 before listening**:
  `-1` → `library index pool size must not be negative, got -1 (SYSML_GRPC_INDEX_POOL)`,
  `many`/`1.5`/`"4 4"` → `library index pool size must be an integer, got "many" (…)`. Assert the exit
  code *and* `ss -ltn | grep :<port>` empty — a service that started anyway is the real failure. An
  empty or all-whitespace value is treated as unset and the service starts normally.
- Client-side shapes that break these sweeps: proto diagnostics carry severity as a **string**
  (`d.severity == "error"`; there is no `sysml_pb2.SEVERITY_ERROR`), `Instance.slots` is a **map** so
  iterate `inst.instance.slots.items()` (and prefer the public `inst.slots` over `inst._slots`), and
  `file_path="/virtual/…"` must never be passed together with `content=` — the service tries to open
  the path and answers `NOT_FOUND`. Keep a runner behind `if __name__ == "__main__":` so importing it
  to reuse its fixture generator does not execute the whole suite.
- Fixture syntax that silently ruins a pool sweep, since 3 diagnostics per model look like the pool
  corrupting resolution: write `transition first s1 accept after 1 then s2;` (not
  `transition s1 then s2 accept after 1;`), and give an `out` an expression
  (`action def Act { in x : Real; out y : Real = x * 2.0; }`).
- Prewarming must not block startup: the port accepts in ~30 ms with pool=4, and SIGTERM after
  prewarming exits **0** in ~13 ms (`Close()` waits for in-flight builds, so a hang here is the
  regression to watch). Time both with `date +%s.%N` around the launch/`kill -TERM`.
- Two cheap adversarial shapes worth keeping: `SYSML_GRPC_INDEX_POOL=1` with 8 threads loading
  distinct models at once (pool permanently drained → mostly inline builds; all 8 must still answer
  `Perf::Engine`, `1+1 == 2` and the full `execute_action` dict), and `-cache-size 5` with 8 distinct
  models loaded (the 3 oldest hashes raise `ModelNotFoundError`, the 5 newest still evaluate) — an
  index handed to a model that is later evicted must not disturb the models still cached.
- Interpreter trap: see [the venv trap](#venv-trap) above before blaming `import grpc`/`import
  pysysml` on the change under test.

#### The shared on-disk library index cache (`internal/core/libs/cache.go`)

The REPL, the LSP and the gRPC service all read `~/.cache/sysml-ls/libs` (or
`$XDG_CACHE_HOME/sysml-ls/libs`), 95 content-addressed `*.idx` files — so a change to that file needs
a cross-surface check, not just a gRPC one. Move the directory away (cold) and run
`./bin/sysml <model> -instantiate <fqn> -e <fqn>::attr` plus an LSP `initialize`/`shutdown`/`exit`
over `--stdio`, then repeat warm: expect roughly **860 ms cold vs 160 ms warm** for the CLI run, the
95 files rebuilt, no leaked `sysml-lsp`, and `ls -a` showing **zero** `.idx.tmp*` droppings (writes go
through a unique temp file plus rename). Overwriting one `.idx` with garbage — and, separately,
truncating one to 0 bytes — must degrade to a silent rebuild back to its original size (same output,
exit 0), never an error mentioning the cache.

### The `Query` RPC / `model.query(...)` (SysML v2 API & Services, PR #155)

`internal/grpc/query.go` + `python/pysysml/query.py` implement the standard's Query resource
(`scope`/`select`/`where`, `PrimitiveConstraint` with `=`/`>`/`<` and `inverse`,
`CompositeConstraint` with `and`/`or`). Testing notes that generalize:

- **Refresh `~/.pysysml/bin/sysml-grpc` or you test a stale service.** A missing capability shows
  up as a `MissingCapabilityError` or an "unimplemented" RPC rather than a build failure, so start
  every run with `make build-grpc && cp bin/sysml-grpc ~/.pysysml/bin/sysml-grpc` and print
  `md5sum` of both plus `git log --oneline -1` on camera. `GetServerInfo().capabilities` should
  list `type_facts, convert, verification, query`.
- **The Python layer validates before the wire, so client-path tests cannot reach service-side
  faults.** Undefined payload keys, an empty composite and a missing operator are rejected locally
  as `QueryError`. To exercise the service's own validation (unknown property, unknown scope, `>`
  on a non-ordered property, non-numeric operand, `Constraint` with neither variant, unset query)
  call the raw stub: `model.connection._stub.Query(sysml_pb2.QueryRequest(...))`. Assert **both**
  paths — the typed client error and the service's `INVALID_ARGUMENT` message naming the problem.
- Typed client errors are the contract: `INVALID_ARGUMENT` → `InvalidRequestError` (also a
  `ValueError`), a bogus/evicted `model_hash` → `ModelNotFoundError` (**NOT_FOUND** is intended
  here, consistent with the other RPCs — don't file it as a wrong status). If a raw
  `grpc._InactiveRpcError` reaches the caller, the call is missing `translate_rpc_errors()`.
- **A query matching nothing must be an empty list, never an error.** Always assert that
  explicitly, otherwise a validation bug that silently answers `[]` looks like a pass.
- **Anonymous members are the classic scope-walk bug.** A `doc`/comment note, an anonymous usage or
  an anonymous `connect` has a degenerate FQN (`Pkg::`), so an unguarded walk answers non-unique
  `@id`s. Keep a fixture with all four shapes and assert, per model: no empty/degenerate `@id`, no
  duplicates, anonymous elements absent, **and** that named descendants of a named parent are still
  answered (the fix drops descendants of anonymous elements too, so it can over-prune). Element
  counts are the canary: `examples/rdf-interop-demo.sysml` is 22 elements with the fix (25 before),
  4 of them PartUsages, so an `inverse` partition is 18 + 4 = 22.
- **Stdlib elements restored from the library cache are lossy** and this is documented, not a bug:
  ~47 elements under `Base`/`Occurrences`/`Links`/`Clocks` come back with **no** `@type` (their
  symbol kind is `SymbolUnknown`), so they never match `@type =` but are kept by its `inverse`; ~12
  `*Definition`s (e.g. `ScalarValues::Boolean`) come back with **no** `isAbstract`. Any claim in
  docs/reference/api.md that the `@type` mapping is total is a doc bug unless it is scoped to "every kind a
  parsed declaration can have".
- Property-absence is the documented shape throughout: `owner` absent for a top-level package,
  `type` absent for an untyped attribute, `declaredName` absent for an *effective* name. A one-line
  fixture (`part :>> engine;` inside `part v`) gives the effective-name case, with a sibling
  `part motor2;` as the control that still carries `declaredName`.
- Good fixtures: `examples/rdf-interop-demo.sysml` (nesting + PartUsages),
  `examples/sysml-v2-training/04. Subsetting/Subsetting Example.sysml` (`[*]` vs `[4]` — proves `*`
  behaves as infinity for `>` and is excluded by `< 5`, plus `abstract part def`),
  `examples/state-machine-demo.sysml` (imports `ScalarValues::*`; an empty scope must still answer
  only its own 5 elements, not the stdlib).
- Drive it as one scripted assertion runner (`PASS/FAIL <id> <claim> | <evidence>` lines and a
  final `n/n assertions passed`) with an `input()` pause between sections when `QT_PAUSE=1`, then
  step the pauses on camera. Never type `clear` at such a pause — it is consumed as the Enter for
  the *next* section and you lose that section from the screen; use Konsole's `shift+PageUp`
  scrollback to recover it.

## Enumeration literals: runtime value and gRPC wire form (PRs #197, #208)

A literal's identity is its **declaration**, so nothing in this area should ever be tested by
comparing rendered text. The two flavors behave differently and both need a case: a plain literal
(`enum def Color { red; green; blue; }`) is its own value and crosses the wire as an
`EnumLiteral`, while a literal of an enumeration that specializes a scalar
(`enum def GradePoints :> ScalarValues::Real { A = 4.0; }`) evaluates to the **declared scalar**,
so `eval("D::GradePoints::A")` must be the float `4.0`, not an `EnumLiteral`. A literal can also own
attributes (`Level { low { :>> n = 1; } high { :>> n = 9; } }`), read as `eval("D::Level::high.n")`
→ `9`.

- **Stale-binary trap (costs an hour if missed).** `pysysml` auto-start runs
  `~/.pysysml/bin/sysml-grpc` and otherwise *downloads a release*, which lacks new capabilities.
  Always `make build-grpc && cp bin/sysml-grpc ~/.pysysml/bin/` first and prove it on camera with
  `./bin/sysml-grpc --version` (prints `commit: <sha>`) plus `ls -l` on both paths. Capability check:
  `conn.server_info()` must list `enum_values`
  (`['convert','enum_values','query','type_facts','verification']`).
- **Wire field numbers are shared real estate.** `api/proto/sysml.proto` has
  `Quantity quantity = 8;` and `EnumLiteral enum_literal = 9;` — the enum arm moved from 8 to 9 when
  `main`'s quantity support landed. After any merge that touches the `Value` oneof, re-run both
  arms *on the same part* (one `part def` with `attribute c : Color = Color::red;` **and**
  `attribute mass = 1500.0 [SI::kg];`): a field-number mismatch shows up as `None`/`unsupported`,
  not as an exception. `python/tests/test_wire_compat.py` pins the numbers at unit level.
- **Incoming values go through one converter.** `ProtoToValueIn(pv, idx, sem)` in
  `internal/grpc/convert.go` dispatches the quantity and literal arms and recurses into sequences;
  it is called from `internal/grpc/service.go` (action inputs) and `internal/grpc/verify.go` (calc
  arguments). The error wording is layer-specific and worth asserting verbatim:
  calc → `calc argument could not be read: …`, action → `input "c" could not be read: …`.
- **Identity is `literal_id` alone** (`python/pysysml/enumeration.py` marks `enumeration_id` and
  `name` `compare=False`). Comparing two *wire-populated* literals passes even when this is broken,
  so always include the bare-vs-populated cases: with `bare = EnumLiteral("D::Color::red")` and the
  slot value, assert `bare == car.c`, `hash(bare) == hash(car.c)`,
  `len({bare, car.c}) == 1`, `{car.c: "R"}[bare]` and `{bare: "R2"}[car.c]` both resolve, while
  `bare != EnumLiteral("D::Color::green")` and `len({bare, green}) == 2`. The broken shape reads
  `False False 2`. Also send a bare literal *to* the server (`calc IsRed([EnumLiteral(
  "D::Color::red")])` → `True`) — a description-free literal must still resolve against the index.
- **Type, not text.** Assert `isinstance(car.c, pysysml.EnumLiteral)` **and**
  `car.c != "Color::red"`; a string-shaped decode passes every `str()`-based check.
- **Input round-trips need a real executable action.** An `action def` whose body is only `in`/`out`
  parameters is not executable (`initialize action: no initial node found`). Use
  `action Classify { attribute c : Color = Color::green; attribute isRed : Boolean = false;
  first start; action inner { assign isRed := c == Color::red; } done end; then start inner;
  then inner end; }` and pass `inputs={"c": red}` — the in-model default being *green* is what
  proves the input actually bound.
- **Adversarial wire ids** (run on both the calc and action path): `D::Color::mauve` →
  `… D::Color::mauve is not an enumeration literal of this model`; empty `literal_id` →
  `enumeration literal: literal_id names no declaration`; a valid but non-literal FQN `D::Car` →
  `… D::Car is not an enumeration literal of this model`. Through `eval` the typed runtime error
  appears instead: `not a literal of the enumeration: mauve is not a literal of Color (literals:
  red, green, blue)`. Finish with `eval("1 + 1")` → `2` to prove the service survived. Carried over
  from #197 and still true: a *bare* REPL `%eval D::Color::purple` never reaches `ErrNotALiteral`,
  it answers `unresolved reference … did you mean <stdlib name>?`.
- **Quantities: received but not sendable from the public client.** `connection._python_to_value`
  has no `Quantity` arm (bool/int/float/str/None/Instance/EnumLiteral/list only), so a quantity
  *input* can only be exercised by building a `sysml_pb2.Value(quantity=…)` and calling the stub on
  the same channel. That asymmetry is `main`'s, not the enum PR's — report it as "found, not fixed"
  rather than a defect. **Trap:** do not fabricate the `unit_term`; naming `SI::kg` as a base factor
  yields the false-looking `incommensurable units: cannot express SI::kg (1000·SI::gram) in SI::kg
  (SI::kilogram)`. Echo back the reduction the service itself reported
  (`car.mass.unit` → `scale_num=1000.0`, `factors=(UnitFactor('SI::gram', 1.0),)`).
- **Codegen and REPL cross-checks.** `python -m pysysml.generate` must emit
  `def c(self) -> _t.EnumLiteral: return _t.slot(self, "c", _t.as_enum_literal)` and, for the
  quantity slot, `_t.Quantity` / `_t.as_quantity`; then read both off a live instance
  (`Car.from_instance(conn.instantiate("D::Car")).c`) so a wrong decoder raises `TypeMismatchError`
  instead of passing silently. In the REPL, `%features` prints `name = value`, i.e.
  `c = Color::red`, `palette = [Color::red, Color::green, Color::blue]`, `mass = 1500.00 [SI::kg]` —
  requests often phrase it as `c: Color::red`, which is the same thing.
- **Set membership** is only observable through `->includes`/`union`; no REPL syntax builds a `Set`,
  and a sequence literal keeps duplicates by design.

## Typed codegen (`python -m pysysml.generate`, Tier 2)

`python -m pysysml.generate <model.sysml> [-o out.py] [--host --port]` loads the model through
the **live service** (so it auto-starts `sysml-grpc`) and prints/writes one class per SysML
definition deriving from `pysysml.typed.TypedObject`. Useful facts when testing it:

- The reference fixture is `internal/repl/testdata/vehicle_package.sysml` and the committed
  golden is `python/tests/golden/vehicle_types.py`; `cmp` them for a byte-for-byte assertion and
  generate twice + `cmp` for determinism. Emission is FQN-ordered with base classes first.
- Only instance-slot usages become properties (`attribute/part/item/occurrence/port/enum`);
  `calc`, `constraint` and `requirement` members are deliberately absent — a generated class
  that grows a `withinMassLimit` property is a bug, not progress.
- Annotations are the whole point: `attribute power = 300.0;` must render `-> float` and
  `part engine : Engine;` must render `-> Engine`. If everything renders `object`, the typefacts
  path (`internal/grpc/typefacts.go` → `SymbolInfo.type_info`) is broken.
- Static-check evidence needs `MYPYPATH=<repo>/python mypy --follow-imports=silent script.py`
  and the venv mypy (`~/pysysml-venv/bin/mypy`). Without `MYPYPATH`, mypy silently treats
  `TypedObject` as `Any` and *misses* attribute-typo errors, so a "clean" mypy run proves nothing
  until you have seen it also flag a deliberate misuse (`v.mas`, `v.mass + "x"`).
- Adversarial cases that distinguish working from broken, all reachable through a generated
  property: cyclic derived attributes (`a = b + 1.0; b = a + 1.0;`) must raise pysysml
  `SlotError` rather than returning `None`; and running *stale* generated code against a model
  whose attribute type changed (e.g. `mass = "heavy"`) must raise
  `TypeMismatchError: slot 'mass': expected float, got 'heavy'`.
- **Pass the model path absolutely.** The path travels to the service, which opens it relative to
  *its own* CWD, so `-o` works but `../internal/...` fails with a gRPC `NOT_FOUND: file not found`
  traceback that looks like a client bug and is not one.
- **Capability gate (`type_facts`).** Generation calls `GetServerInfo` first and exits 1 without
  writing anything unless the service reports `type_facts`. To simulate a stale service, keep a
  pre-`GetServerInfo` binary around — `~/.pysysml/bin/sysml-grpc` from an older release is usually
  one; check it with a direct `stub.GetServerInfo(...)` (expect `StatusCode.UNIMPLEMENTED`), then
  run it on a spare port (`-port 50077 -health-port 8099`) and generate with `--port 50077`. Assert
  the negatives: target file still absent / identical sha256, and stdout mode piped to `wc -c`
  yields `0` (no silent all-`object` module). The message should name the capability, the service
  origin and `make build-grpc`. Note the repo blueprint copies the freshly built `bin/sysml-grpc`
  into `~/.pysysml/bin/`, so in a clean session the cached binary is *current* and no longer serves
  as the stale fixture — either keep a copy of an older release binary aside first, or stand up a
  stub gRPC server that answers `GetServerInfo` with `UNIMPLEMENTED`.
- **`--check`** requires `-o` (exit 2 otherwise), writes nothing, and exits 1 when the target is
  missing or would change, naming the exact regenerate command. The strong assertion is
  `sha256sum` before *and* after — exit code alone does not prove nothing was written.
- Generated modules carry `SYSML_GENERATOR_VERSION` and `SYSML_MODEL_HASH` (sha256 of the
  newline-normalized model source), so any model edit changes the stamp and therefore the bytes.
- **`TypedObject.from_instance` guard.** It rejects only an instance whose reported type belongs to
  a *different* generated class; a generated subclass and an unrecognized type both pass. To cover
  all three branches you need a fixture with a specialization and a usage, e.g.
  `part def SportsCar :> Vehicle { ... }` plus `part myCar : SportsCar;` — an instantiated *usage*
  reports its own FQN (`Demo::myCar`), which no class carries, so it must be accepted. `unchecked()`
  bypasses the check.
- **Restarting the service on 50051:** kill it by PID (`ls -l /proc/<pid>/exe` to confirm which
  binary), not `pkill -f 'bin/sysml-grpc'` — that pattern also matches the tool shell running the
  command and kills your own session. Rebuilding leaves the old process serving a `(deleted)`
  binary, which silently tests the previous revision.
- `python/tests/test_lifecycle.py::TestLifecycleRobustness::test_service_shuts_down_when_last_process_exits`
  fails (`FileNotFoundError: ~/.pysysml/sysml-grpc.pid`) whenever an externally started service is
  already listening on 50051 — a known service-ownership gap, reproducible on `main`. Confirm on a
  `main` worktree before reporting it as a regression.
- Do not try to force a type mismatch inside the model — the checker rejects
  `attribute count : Integer = 4.0 / 2.0;` with `cannot bind Rational value to a feature typed
  by Integer`, and `Integer`/`String` need `import ScalarValues::*;` or generation exits 1 with
  `error: unresolved reference: Integer`.

## Built-in library functions (sqrt/sin/exp/ln/log/atan2 …)

The runtime supplies bodies for the function-library declarations in
`internal/core/runtime/library_functions.go`; the non-normative extensions
(`exp`, `ln`, `log`, `atan2`) live in
`internal/core/libs/stdlib/OpenSysML Libraries/OpenSysMLMathFunctions.kerml`.
Testing notes that generalize to any future built-in:

- The fastest end-to-end surface is the batch flag, which loads a model *and* evaluates
  expressions against it: `./bin/sysml -e "exp(1.0)" -e "log(8.0, 2.0)" model.sysml`. Several
  `-e` flags are allowed and each prints `✓ <expr>` then `  = <value>`; a failure prints
  `error: evaluation failed: ...` and the process still exits 0, so assert on the text, never on
  the exit code.
- **`-e` is evaluated in the root scope, not inside the model's package.** So a model that declares
  its own `calc def exp` is *not* exercised by `-e "exp(2.0)"` (that hits the built-in). To prove
  shadowing, either use the FQN (`-e "OwnExp::exp(2.0)"`) or read it out of an attribute default
  with `%instantiate`/`%features`. Getting this wrong looks exactly like a shadowing bug.
- Results print to **two decimals**, which hides precision differences. To assert exactness, compare
  in the model instead: `-e "log(1000.0, 10.0) == 3.0"` → `= true` (the naive `ln(x)/ln(base)`
  gives 2.9999999999999996 and would print `= 3.00` while being `false`).
- Domain/overflow handling is deliberate: `realResult` converts NaN/Inf into
  `arithmetic domain error` / `arithmetic overflow`, so a bad argument must yield an `error:` line,
  never a number. Worth checking per function (`ln(0.0)`, `log(4.0, 1.0)`, `atan2(0.0, 0.0)`,
  `exp(1000.0)`), plus wrong arity (`calc argument count mismatch`) and a non-numeric argument
  (`type mismatch: ... requires a numeric value`).
- A **bare** call with no `import` still evaluates (the unqualified-name table is always in force)
  but the checker prints `error: unresolved reference: <name>` for the same call inside a
  declaration. That divergence is a known rough edge of the built-in dispatch, not a new bug —
  report it as expected, and use `import OpenSysMLMathFunctions::*;` in fixtures to avoid it.
- Fixture gotchas when writing action fixtures by hand: `done` is a reserved keyword (use another
  name), a body has one start so only one `first` end (chain the rest as `first a then b; then b c;`
  — two `first` ends yield `action has multiple initial nodes`), and `and`/`or` are
  `unsupported operator` in constraint bodies — keep constraint expressions to a single comparison.

## Testing parser changes end-to-end (keyword/symbol parity, dispatch rewrites)

A parser change is only convincing if the *same input file* behaves differently on the two binaries,
so build an A/B baseline first and keep it around:

```bash
git worktree add /tmp/mainwt main
go build -C /tmp/mainwt -o /tmp/mainwt/sysml-main ./cmd/sysml && go build -C /tmp/mainwt -o /tmp/mainwt/sysml-lsp-main ./cmd/sysml-lsp
```

Three cheap, high-signal sweeps:

1. **Corpus no-diff sweep** — load every model on both binaries and diff the output. Anything other
   than `0` differences is either the intended change or a regression, and it takes ~4 min.
   **Both `%load` and the argv positional split the path on whitespace**, and ~80% of the corpus
   lives under directories like `examples/sysml-v2-training/05. Redefinition/`, so copy each file to
   a space-free path instead of passing it — and count what you compared, or a sweep that loaded
   almost nothing still prints a reassuring zero:
   ```bash
   n=0; d=0
   while IFS= read -r -d '' f; do
     cp "$f" /tmp/sweep.sysml
     n=$((n+1))
     diff <(./bin/sysml -quiet /tmp/sweep.sysml </dev/null 2>&1) \
          <(/tmp/mainwt/sysml-main -quiet /tmp/sweep.sysml </dev/null 2>&1) >/dev/null \
       || { d=$((d+1)); echo "DIFF: $f"; }
   done < <(find examples testdata internal/repl/testdata -name '*.sysml' -print0)
   echo "compared $n, differing $d"
   ```
   A `for f in $(find …)` loop word-splits those paths and silently compares nothing for them: on
   PR #98 that hid three real output changes (better diagnostic spans on `part redefines engine = …`)
   behind a clean-looking 0.
2. **Twin table** — for a notation with two spellings, run every degenerate form of *both* spellings
   through a tiny script and print keyword/symbol/main side by side. Testing only the well-formed
   forms hides the interesting bugs: on PR #98 the well-formed forms were perfectly at parity while
   `redefines;`, `redefines = 5;`, `subsets ;`, `crosses ;` were accepted **silently** and their
   symbol twins `:>>;`, `:>> = 5;`, `:> ;`, `=> ;` all reported `expected a name`. A silently
   accepted member is invisible in a `✓ package T` line — always pair the load with
   `%instantiate`/`%features` to see what the member actually did (there, nothing).
3. **LSP diagnostics without an editor** — drive `bin/sysml-lsp` over stdio with ~15 lines of Python
   (`initialize`, `initialized`, `textDocument/didOpen`, sleep 3, read stdout) and count
   `"severity":1` in the `publishDiagnostics` notification. A file with **no** diagnostics produces
   **no** notification at all, so treat a missing notification as zero errors rather than a hang.
   `sysml-lsp: failed reading header line: EOF` on stderr at shutdown is normal.
4. **LSP hover/definition as a symbol-table probe** — the REPL has **no** `%hover`/`%def`, so a
   naming/symbol-registration change is best shown through `bin/sysml-lsp`: reuse the stdio driver
   and add `textDocument/hover` and `textDocument/definition` requests with `id`s, then parse the
   `Content-Length`-framed replies (searching for `"id":100` in the raw text prints the *tail* of the
   message, not its body — frame-parse instead). Hover returns a one-liner like
   `attributeUsage x`, which makes a mis-derived name obvious: on `main` a short-named redefinition
   hovered as `attributeUsage redefines` and its definition was `null`. Make the sleep before reading
   stdout configurable (e.g. `LSP_WAIT=8`); 4 s is occasionally too short and yields
   `NO RESPONSE`/zero diagnostics, which looks like a pass — re-run with a longer wait before
   believing an empty result.
5. **Naming probes for a `<shortName>` change** — for `attribute <sn> redefines x = 5;` the
   assertions that separate working from broken are: `%features` lists the member as **`x = 5`**
   (broken revisions show `sn = 5` with `x = 1`, or a bogus `redefines = <unknown>` slot);
   `%eval T::A::x` **and** `%eval T::A::sn` both evaluate; and an action body doing
   `assign total := total + x` completes instead of `unresolved reference: x`. Load-level `✓ package T`
   proves nothing here.

Go may not be at the blueprint's `/usr/local/go/bin` on every box; check `~/sdk/go/bin` too
(`PATH=$HOME/sdk/go/bin:$HOME/go/bin:$PATH`) before concluding the toolchain is missing.

### Naming changes that reach lowering and the runtime

When a parser change alters which declarations carry a name (e.g. `ast.EffectiveName` deriving the
name from a `redefines`/`references` target), the parse is the *least* interesting surface —
consumers reading `Usage.Ident.Name` break silently downstream. The probes that actually distinguish
fixed from broken, each with a visible A/B against `main`:

- **Attribute default** — `attribute redefines x = 5;` in an action body with a statement reading
  `x`; a lost name surfaces as `error: execution failed: eval assignment RHS: unresolved reference: x`,
  not as a parse error.
- **Step ordering** — `action redefines bump { … }` ordered by `then start bump; then bump end;`.
  A lost name fails at lowering: `succession edge references undefined target node`.
- **Trace naming** — `%trace on` then `%continue`; the step must print `token 1@bump`. A generic
  `token 1@usage_action` (the `nodeIdentifier` fallback in `runtime/trace.go`) is the bug signature,
  and it also reproduces with any unnamed step such as `perform worker;`.
- **Calc parameter** — `in redefines factor = 3;` overriding an inherited `in factor = 2`. The
  giveaway is a *wrong number*, not an error: the invocation silently uses the inherited default
  (`Scaled(7)` → 14 instead of 21), so assert the value, never just "it evaluated".
- **State** — `state redefines waiting { accept go then active; }`. A lost name makes the sourceless
  accept vanish: `%state` shows `Events: 0` and `%advance 1` never leaves the initial state.

Ready-made fixtures for all of these live in `internal/core/runtime/testdata/conformance/`
(`action_redefined_attribute_default[_symbol]`, `action_redefined_step_ordering`,
`calc_redefined_parameter[_symbol]`, `state_redefined_state_accept[_symbol]`) — load them straight
into the REPL with `%action test::run` / `%eval test::Scaled(7)` / `%state Test::Machine` rather than
writing new models. Where only a keyword fixture exists, `sed 's/redefines/:>>/'` gives the symbol
twin. `action_redefined_step_ordering.sysml` emits a pre-existing
`name conflict: total is already the name of the inherited feature Base::total` on every revision —
not a regression.

Adversarial cases for name derivation: a member with two redefinitions
(`attribute <sn> redefines x, y = 9;`) derives *no* name from them, so it answers to its short name
(`%features` shows `sn = 9` and leaves `x`/`y` at their inherited values) with no diagnostic — assert
which key the value landed under rather than assuming it was dropped. REPL call syntax: named
arguments work as `Scaled(x = 7, factor = 5)`, but *mixing* positional and named
(`Scaled(7, factor = 5)`) is a parse error on every revision — don't read that as a
parameter-naming bug.

## Quantities, unit-name resolution and prompt scope

A name in the unit position of `x [u]` is an ordinary feature reference, so it resolves to the
**nearest** declaration and then must conform to a measurement unit. Testing anything in this area:

- There are **four** evaluator paths that reach a unit name and they must agree: a slot
  (`%instantiate` + `%features`), an action (`%action`), a calc (`%calc`), and a constraint
  (`%constraint`). The constraint path historically diverged — it reached past a nearer declaration
  and silently converted in metres, giving a *wrong answer with no error*, so always include
  `%constraint` and assert the diagnostic, never just "the other three agree".
- The high-value fixture shape is a body that declares `attribute m` (mass!) next to
  `500.0 [m]`, with `public import SI::*` in the enclosing package. Expect, verbatim, from all four:
  `not a measurement unit: m resolves to the attributeUsage m declared in <NS>, shadowing the
  measurement unit SI::metre — write SI::m to name the unit`. Ready-made:
  `internal/core/runtime/testdata/conformance/unit_shadowed_by_sibling_{slot,action,calc,constraint}.sysml`,
  plus `unit_shadowed_by_local_unit` (a sibling that *is* a unit must still evaluate) and
  `unit_undeclared` (`unresolved unit furlong` — a different message, assert it stays different).
- Assert the **neighbouring** quantity too: `1000.0 [kg]` next to the shadowing `m` must still print
  `m = 1000.00 [kg]`. A too-broad fix breaks that and no error-message check would notice.
- The remedy clause (`— write SI::m …`) comes from resolving the name again with the shadowing
  declaration hidden, so it is produced for a shadow declared inside a body *and* for one declared
  at package level next to the `import SI::*` itself. A message that stops at
  `… m resolves to the attributeUsage m declared in ADV` means that second lookup failed — worth
  reporting, since the package-level shadow is the likely real-world spelling.

**Prompt scope.** `%eval`/`%calc` evaluate in the scope of the **last namespace the session
declared** (`Session.promptScope`), which is what makes `%eval 1.0 [m/s]` work for a loaded package
that imports `SI::*`. Consequences to test deliberately, because they surprise users:

- Typing a *new* package mid-session moves the prompt scope: after
  `package Demo { attribute mass = 3.0; }`, `%eval mass * 2` = `6.00` but `%eval 1.0 [m]` starts
  failing `unresolved unit m` (Demo imports nothing).
- With two packages, only the last one's members resolve unqualified (`%eval b * 3` works,
  `%eval a + b` is `unresolved reference: a`); qualify them (`%eval P1::a + P2::b` = `3.00`).
- `%eval <unit>` (e.g. `%eval m`) resolves through that scope and answers
  `error: "m" has no value to evaluate`, not `symbol "m" not found` — the latter is the pre-fix
  signature.

**`%calc` arguments** are parsed as a list of expressions, so an argument containing spaces survives.
All of these must give the same result: `%calc P::Fall 10.0 [m/s], 3.0 [s]` (comma),
`%calc P::Fall(10.0 [m/s], 3.0 [s])` (invocation form), `%calc P::Fall 10.0 [m/s] 3.0 [s]`
(whitespace only), `%calc P::Fall (1.0 + 9.0) [m/s], 3.0 [s]` (parenthesized subexpression),
a nested invocation as an argument, and a trailing comma (accepted silently). Whitespace separates
two arguments only where each side is a complete expression, so `%calc add 5 -3` is two arguments
while `%calc add 5 - 3` is one subtraction (and then reports `parameter "y" has no argument`) —
check a signed second argument both ways. Malformed input must
diagnose and leave the session usable — named args → `named arguments are not supported here; pass
arguments positionally`; unbalanced paren → `failed to parse argument "(…"`; a lone `,` →
`failed to parse argument ","`; no args / `()` → `unbound parameter: …`; too many →
`calc argument count mismatch`. Pre-fix signature: `evaluation of argument "[m/s]," failed:
unsupported node type: *ast.ErrorNode`. Follow the sweep with `%eval 1 + 1` → `= 2`.

**Quantity rendering.** Two separate printers, both worth an A/B against the parent commit:
a violated assertion renders the bracket form as source (`Assertion evaluated to false:
1.0 [m] > 500.0 [m]`; a missing `*ast.IndexExpr` case shows `index > index`), and a result table
formats the magnitude like a bare Real (`%action test::propagate` +`%continue` on
`internal/core/runtime/testdata/conformance/action_body_quantity_descent.sysml` → `t = 17.20 [s]`,
`h = -0.42 [m]`, `v = -42.86 [m/s]`; raw floats such as `17.19999999999997 [s]` are the pre-fix
signature). Note the action in that file is named **`propagate`**, not `descent`.

## Multiplicity, subsetting and collection slots in `%features`

`%features` is the cheapest window on instantiation semantics, and the interesting values are all in
its output rather than in an exit code — so assert on the exact rendered text:

- `part xs : C[*]` should print `xs = []` (an empty collection). `<error: multiplicity violation:
  lower bound too large or infinite for slot "xs">` is the signature of `[*]` being read as `*..*`;
  that error is the *correct* answer only for an explicit `[*..*]` or an absurd bound like
  `[100000]`, both of which must return instantly rather than allocating.
- A nested `part a : Sub :> xs` makes `a` one of `xs`'s values, and a redefinition
  `part ys : C[*] :>> Xs` makes `ys` and `Xs` render the *same* list — check both names, not one.
- Feature chains over a collection (`sum(xs.m)`) flatten one level per step. Useful negatives:
  an empty collection gives `total = 0` (not an error); a chain reaching an unset slot gives
  `<error: … uninitialized slot: m>`; two features subsetting each other give
  `<error: … cyclic slot dependency: … subsets itself>`; mixing a bare number with a `[kg]` value
  gives `<error: … incommensurable units …>`. None of these may panic — follow each with
  `%eval 1 + 1` → `= 2` to prove the session survived.
- `sum`/`product` over quantities must keep the unit (`totalmass = 7.00 [kg]`); a bare `7.00` is a
  regression.
- Unset attributes print `<unknown>`, and an attribute typed by a plain `Real`/`String` with no
  value may instead materialize as a nested `Instance(ID: n)` with `(no features)` — that is
  pre-existing rendering, not evidence of a new bug, so do not report it as one without an A/B
  against `main`.

## Multi-file projects: `%load <path>...` and positional dirs/globs (PR #146)

`sysml <dir|glob|file>...` and `%load <path>...` expand to model files via
`internal/core/project.Expand`, and every file is accepted before one analysis pass
(`Session.SubmitAll`), so load order does not affect name resolution. Shapes to expect:

- More than one file prints a `loaded N files:` header listing each path (a single file prints no
  header — a good tell that the multi-file path was taken).
- Only `.sysml`/`.kerml` are collected; a `.md` sibling and any **hidden** directory (`.git`,
  `.hidden`) are skipped. Put an unparseable file inside a `.hidden/` dir: any diagnostic from it
  means the skip broke.
- Errors are worth asserting verbatim: `no .sysml or .kerml files in <dir>`,
  `no model files match "<pattern>"`, `load <file>: open <file>: no such file or directory`, and
  `usage: %load <file|dir|glob>...` for a bare `%load`.
- Duplicates dedupe by absolute path, so `file dir/` where `dir` contains `file` loads it once.
- `~` and quoted paths with spaces work; quote globs in the REPL (`%load "/tmp/p/*.sysml"`) and on
  the CLI so the shell does not pre-expand them.
- `-convert` still takes exactly one file: a directory gives
  `cannot tell the format of "<dir>": it has no extension, so pass -from`.

**Two regressions fixed late in #146 — re-check both after any change to the load path:**

1. *Per-file diagnostic positions.* Diagnostics from a load are printed as
   `<file>:<line>:<col>: ...` with the line counted from that file's start. With `a-ok.sysml`
   (5 lines) sorting before `b-bad.sysml` whose line 3 is `attribute z = ;`, a directory load must
   report `/tmp/bp/b-bad.sysml:3:17:` — the same position the file reports when loaded alone. A
   joined-buffer line (e.g. `9:17`) or a missing filename is the bug returning
   (`Session.origins`/`Result.locate` attribute an offset to its file).
2. *Every loaded file survives the load.* Two files of one load declaring the same top-level name
   must both stay in the session (`%list` shows both); name-based snippet replacement only
   supersedes **earlier** submissions. Retyping a declaration at the prompt must still replace what
   a previous load put there.
3. *Symlinked directories are walked.* `%load /tmp/link-to-proj` loads the files behind the link,
   symlinked subdirectories included, each resolved directory at most once so a link cycle
   terminates; a dangling link is skipped. A project reached through a symlink is a realistic
   setup, so include one.

To prove order-independence is real rather than accidental, name the referencing file **first**
alphabetically (e.g. `a-uses.sysml` importing a package declared in `b-defs.sysml`). The pre-#146
binary emits `unresolved reference: Defs` / `Widget` / `size` on exactly that input, which is the
strongest available evidence.

Trap while driving the REPL on camera: typing `clear` at the `sysml>` prompt not only errors
(`1:1: error: expected a namespace member`) but leaves garbage in the session buffer that makes a
subsequent `%eval Pkg::part.attr` fail with a parse error. `%clear` and re-`%load` recovers.

## The stream/status contract of a non-interactive run (PR #161)

Since #161 every run that is not a prompt splits its output: **results on stdout** (evaluated values,
conversion bytes, verdict lines, `✓` echoes of what a load declared) and **findings on stderr**
(diagnostics, warnings, the `sysml: … did not analyse cleanly` note, `wrote <file> (ttl, N bytes)`),
with `0` = done, `1` = the model answered a check false, `2` = nothing could be decided. Any change in
`cmd/sysml/{main.go,status.go,check.go}` or `internal/repl/{load.go,render.go}` can break it silently,
so test it as a **table over every mode**, always with `>out 2>err </dev/null` and `echo $?`:
capturing `2>&1` hides exactly the defect. A ready-made driver pattern (one `PASS`/`FAIL` line per
row, asserting status + required stdout needles + stderr needles that must be **absent** from stdout
+ "stdout empty") lives in the session that tested #161; re-create it rather than eyeballing output,
because it is legible on camera and a leak turns a row red.

Values that held at `6079699` (broken = `package Bad { part def A { attribute x : ; } }`):

| run | exit | stdout | stderr |
|---|---|---|---|
| `-e 1+1 <good>` / `-calc`/`-action`/`-state` on a clean model | 0 | `✓ package …`, `= 2` / `= 18` / `total = 5` | empty |
| any of those over the broken model | 2 | **empty** | `error: expected a name`, `sysml: … did not analyse cleanly` |
| `-e nope <good>` (unresolved name) | 2 | `✓ package …` only | `sysml: unresolved reference: nope` |
| `-validate <broken>` | 2 | empty | `… did not analyse cleanly; no check was made` |
| plain `sysml <broken>` **non-tty** | 2 | banner only | diagnostics |
| `-convert ttl <model>` | 0 | Turtle | empty |
| `-convert ttl -o f` | 0 | **empty** | `wrote f (ttl, 1540 bytes)` |
| `-convert ttl <broken> -o f` | 2 | empty | syntax errors, **and no file is created** |
| `-constraint` false / holds / unresolved / warning-only | 1 / 0 / 2 / 0 | verdict line | warning + unresolved verdicts only |

Traps found while testing it:

- **The tty override is the one path unit tests cannot cover.** `atTerminal()` (`status.go`) forces
  status `0` when stdin is a terminal, so `./bin/sysml <broken>.sysml` at a real terminal must load,
  report on stderr, open the prompt and exit `0` on `%quit`, while
  `printf '%%quit\n' | ./bin/sysml <broken>.sysml` exits `2`. Verify the tty half in Konsole (or with
  `script -qec "./bin/sysml <broken>.sysml > /tmp/o 2>/tmp/e; echo TTYSTATUS=\$?" /dev/null < in`);
  a plain pipe silently tests the other branch.
- **`part def ;` is *not* a bad line** — it parses as an anonymous part def and prints `✓ part def`.
  To show the prompt surviving a bad line, use `%eval nope` (unresolved) and
  `part def B { attribute q : ; }` (syntax error), then a following `%eval 6*7` → `= 42` as proof the
  session continued. Expect the `note: deeper checks may not have run here: the error on buffer line
  N is unresolved …` line on the submission after the bad one.
- `-convert ttl` of most models fails on unsupported constructs — a **constraint member** is one
  (`cannot convert the constraint member at …`, exit 2). That makes a check-oriented fixture a useful
  negative row but useless as the clean-conversion row; use
  `package Demo { part def Engine; part def Vehicle { attribute mass : Real = 1200.0; part engine : Engine[1]; } }`
  (1540 bytes of Turtle) for the positive one.
- Real repo models to use instead of hand-written ones: `examples/state-machine-demo.sysml` (and 6
  others) analyse clean, while `examples/phase-c-behavioral-bodies.sysml` and
  `examples/parser_features_demo_action_semantics.sysml` do **not** — a free "broken model" that is
  more convincing than a toy fixture.
- Cheap contrast for this PR class: build the parent commit (`git worktree add /tmp/old<sha> <sha>`)
  and run `-e '1+1' <broken>.sysml` through both. Pre-fix prints the diagnostic **and** `= 2` on
  stdout with exit `0` — the single most convincing frame in the recording.

## Recording setup (Linux/Plasma box)

The GUI is on `DISPLAY=:0` (`:1` does not exist here — `wmctrl` will say "Cannot open display").

```bash
cd /home/ubuntu/repos/OpenSysML && (DISPLAY=:0 konsole --hide-menubar >/dev/null 2>&1 &)
DISPLAY=:0 wmctrl -a "Konsole"
DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Enlarge the font before recording with the `ctrl+plus` key combo a few times (`ctrl+shift+plus`
types literal `+` characters into the shell instead of zooming). Konsole starts a shell whose PATH
lacks the Python that `pip install -e python/` installed into, so `import pysysml` fails there while
it works from a tool shell; run `source ~/pysysml-venv/bin/activate` (or
whichever interpreter `python -c 'import sys; print(sys.executable)'` reports in the tool shell)
as a setup step before recording. `~/pysysml-venv` may not exist at all, and the default `python3`
on PATH can be another project's venv (e.g. `~/repos/fprime/fprime-venv`) whose older
`google.protobuf` makes `import pysysml` die with
`cannot import name 'runtime_version' from 'google.protobuf'`. The reliable fallback is a throwaway
venv off the system interpreter:
`/usr/bin/python3 -m venv /tmp/pv && /tmp/pv/bin/pip install -e python/` (~1 min), then
`source /tmp/pv/bin/activate` in Konsole. Also re-copy the freshly built service
(`make build-grpc && cp bin/sysml-grpc ~/.pysysml/bin/`) or the auto-start path serves a stale
revision. Discover expected values with the
piped-stdin form *before* recording, so the recorded run is one clean pass; anything only verified
over a pipe is not visible in the video and should be reported as weaker evidence.

**`%stop` ends a debugging session but does NOT exit the REPL.** After `%stop` you are still at
`sysml>`, so a shell command typed next (`clear; ./bin/sysml`) is submitted as SysML and leaves an
unresolved session error that taints later submissions with
`note: deeper checks may not have run here…`. Use `%quit` when you mean to get back to the shell.

**Never type `clear` at the `sysml>` prompt while recording.** It is submitted as SysML, not run by
the shell, so it leaves an unresolved session error and the next submission gains a
`note: deeper checks may not have run here: the error on buffer line N is unresolved` line that
looks like a defect in the frame. Clear the screen *before* starting the REPL (`clear; ./bin/sysml`),
scroll instead with `shift+PageUp`/`shift+PageDown` when long output (`%builtins`, `%help`) runs off
the top, or quit and restart the REPL for a clean screen.

## Tab completion and anything else readline-driven (PR #148)

This section and the next describe behavior added by PR #148 (which also adds `%search` and
`%builtins`), so they apply only once that PR is on `main`.

`cmd/sysml` installs an `AutoComplete` on the readline config, and **readline disables completion
when the terminal reports a width of 0** — which is what a plain pipe reports. So
`printf '%%bui\t\n' | ./bin/sysml` proves nothing about completion: the TAB is just swallowed.
Two ways to drive it:

1. **Konsole** (what belongs in a recording): send TAB with the computer tool's
   `key: "Tab"`, and discard a line with `ctrl+u` between cases — note `ctrl+u` kills only to the
   left of the cursor, so add `ctrl+k` first if the cursor is mid-line.
2. **A pty harness** (for discovering expected values quickly, off camera): fork a pty, set the
   window size, then write keystrokes. The essential part is the ioctl:

   ```python
   pid, fd = pty.fork()                     # child: os.execv("./bin/sysml", [...])
   fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
   os.write(fd, b"%eval sqr\t")             # then read fd after a short settle
   ```

   The captured stream is full of `ESC[2K` redraws; grep the last redraw of the prompt line rather
   than trying to read it as plain text.

Completion cases worth covering, and their shapes: a unique meta-command prefix completes in place
(`%bui`+TAB → `%builtins`), an ambiguous one lists on the second TAB
(`%s`+TAB TAB → `%satisfy %save %search %features %state %step %stop`), names after `%eval` come from
session declarations, builtin function names and the library (`sqr`→`sqrt`), a qualified prefix
offers **one segment at a time** (`ScalarValues::`+TAB lists only that package's members, never the
whole library; `ScalarValues`+TAB inserts the `::` and lists the same members), and `%load`/`%save`
complete filesystem paths with a trailing `/` on directories.
Adversarial cases that all behaved correctly and are cheap to re-check: TAB with the cursor mid-line
must keep the text to its right, TAB on an empty line lists (capped) names without hanging,
completion into a nonexistent directory inserts nothing, and completion still works on a line long
enough to wrap several terminal rows.

## Where the prompt keeps its history (PR #148)

History is `$XDG_STATE_HOME/sysml/history` when that variable is set (directory created 0700, file
0600), else `~/.sysml_history`; older builds used `$TMPDIR/sysml-repl.history`, so a stale
`/tmp/sysml-repl.history` may be left over from a contrast binary — delete it before asserting "the
new build writes nothing to /tmp". Reuse across runs is testable without a second human: run the
REPL, `%quit`, start it again and press `Up`. An unwritable location must degrade quietly (prompt
starts, no warning, history in memory only) — exercise it with `XDG_STATE_HOME` pointing at a
`chmod 500` directory *and* at a regular file, since those fail in different places
(`MkdirAll` vs `OpenFile`).

## Connector usages and their ends (PR #132)

This section describes runtime materialization of connector usages, so it applies once that PR is
on `main`; the "old" shapes double as A/B canaries against the parent commit.

- **A connector's end IS the connected feature, not a copy.** The only visible signal is the
  instance ID, so read it carefully: for
  `part def Sys { part a : A; part b : B; connection link : Link connect a.p to b.q; }` the fixed
  build prints `a.p = Instance(ID: 4)` / `b.q = Instance(ID: 6)` and then `link = Instance(ID: 7)`
  with `source = Instance(ID: 4)` / `target = Instance(ID: 6)`. A pre-fix build prints *fresh* IDs
  at the ends (`source = Instance(ID: 7)`, `target = Instance(ID: 8)`). Pair the ID check with a
  **value** check by connecting a port that redefines an attribute
  (`port spare : P { :>> rate = 7.0; }`): the end must read `rate = 7.00`, which a default-typed
  copy would show as `3.00`.
- **Untyped/anonymous connectors used to read `<unknown>`.** `interface iface connect a.p to b.q;`
  (and `connection untyped connect …`, `allocation alloc allocate a to b`, KerML `connector`)
  materialize on the stdlib base. A bare `connect a.p to b.q;` has no slot name, so `%features` renders
  it as a synthetic `(anonymous <keyword>) = Instance(ID: n)` line with its ends indented under it —
  those lines are printed *after* all the real features, and their ends are shown without nested
  values. Pre-fix, the named untyped usage printed `iface = <unknown>` and the bare one printed
  nothing at all.
- **End labels tell arity apart.** A binary connector's ends are `source`/`target`; an n-ary
  `connect (a.p, b.q, c.r)` prints three ends all labelled `end` (they occupy the library's unnamed
  `participant` feature). A `connection def Derived :> Base { end :>> source; end :>> target; }`
  redefines inherited ends **by position** and must still attach.
- **Unattachable ends are a typed `<error:` on the slot, never `<unknown>` or a default.** An end
  naming a missing feature gives
  `<error: connector end cannot be attached: anonymous connector end "a.nope" at <repl>:8:17: member nope not found in instance>`
  — assert the `<repl>:line:col` is present, since that is exactly what the pre-fix path lacked.
  A connector with multiplicity (`connection multi : Link[2] connect a.p to b.q;`) gives
  `sys.multi end "multi" at <repl>:8:9: a connector of more than one object has no set of ends to
  attach` — located too, so assert the `<repl>:line:col` on both. Follow either with `%eval 1 + 1` →
  `= 2` to show the session survived.
- **A connector naming itself at an end is `ErrCyclicSlot`, not a hang.** `connection link connect
  link to a.p;` and a mutual pair (`connection here connect there to a.p;` /
  `connection there connect here to a.p;`) both report a cycle.
- **`flow f from a.p to b.p` and `binding bnd bind a.p = b.p` now parse as named declarations**
  (pre-fix: `expected 'to' between flow ends` / `expected '{' or ';' after declaration`). They are
  *not* connector usages for materialization purposes, so a named `flow` still renders
  `f = <unknown>` and a named `binding` gets no `%features` line at all. Don't plan an end-identity
  assertion on them.
- **Variant-interface routing is only partly reachable from the REPL.** `%features` proves the
  *materialization* side: in
  `internal/core/runtime/testdata/conformance/ballandchain_variant_configuration.sysml` the selected
  `engagementRingToBand = engagementRingToBandConnected (Instance ID: 24)` holds
  `engagementRing.ringPort` / `band.ringPort`, not the disconnected variant's ports. But the
  send/accept *routing* side cannot be driven: an action usage does not inherit its `action def`'s
  nodes (`initialize action: no initial node found`), `%action` cannot resolve a nested action of a
  selecting part usage (`unresolved reference: Route::sysDirect::comm`), and a sibling
  `ref :>> link = link::direct;` in the same body does not bind the selection — the run still ends in
  `accept deadlock in action comm: nothing can post the awaited message`. What *is* testable, and
  worth doing, is the negative plus a positive control: a plain (non-variation)
  `interface link connect outPort to inGood;` in an action body routes `send 100 via outPort` and
  completes with `atGood = 100`, while the same model with an unrealized `variation interface`
  returns the typed accept-deadlock instead of delivering to the wrong port or hanging. Treat
  selected-variant routing as unit-test-only coverage.
- **Cheap end-to-end fixture for the whole family:** `internal/core/runtime/testdata/conformance/`
  `connector_end_identity.sysml`, `ballandchain_interface_connected.sysml` and
  `…_disconnected.sysml`; each `.expected.json` has an `identical` / `distinct` array that names
  exactly which end must be which port — the cheapest source of the IDs `%features` should tie together.
- Ball-and-chain reference numbers, for asserting the cost roll-up did not shift:
  `totalCost = 1450.00`, `band` `bandCost`/`ringCost` `= 400.00`, `engagementRing`
  `engagementRingCost`/`ringCost` `= 500.00`, `diamondCost = 550.00`, and
  `engagementRingToBandConstraint: <constraint: satisfied>`.

## Port routing: direction, conjugation and connector end paths (PR #195)

Routing a `send … via p` is only observable through an `accept` that either receives or does not, so
every fixture needs **both** a sender node and a reader node writing an attribute, e.g.
`action reader accept n : Integer via dst { assign got := n; }` — then `%continue` prints
`Results: got = <n>` / `n = <n>`. A model with no reader completes either way and proves nothing.

- **Drive it with `%load <file>` + `%action <name>` + `%continue`** (or `%state <m>` + `%advance 1` +
  `%current` for a transition `accept … via p`). Put `import ScalarValues::*;` in every fixture,
  otherwise the load is noisy with `unresolved reference: Integer — did you mean
  ScalarValues::Integer?` (harmless, but it costs screen space on camera).
- **The two performer paths are different code and both need a run:** `%action Pkg::Part::ship`
  (no object → connectors of the *nearest enclosing part*) and `%instantiate <part usage>` followed
  by `%action Pkg::Part::ship <usage>` (object → connectors of the object's *type*). Same model, same
  expected value; a regression in either one is invisible if you only run one.
- **Direction/conjugation shape:** `port def Chan { out attribute v : Integer; }` with
  `port src : Chan; port dst : ~Chan; connect src to dst;`. `send via src` delivers; `send via dst`
  must fail with
  `error: execution failed: send reaches no receiving port: port "dst" is joined only to outbound
  ends (src)`. The mirror shape with an `in` feature (`port def Ctrl { in attribute c; }`,
  `cin : Ctrl`, `cout : ~Ctrl`) must fail on `send via cin` — worth running, because it catches a
  conjugation applied in only one direction.
- **A port with no flow features is undirected** (receives either way) and an `inout` feature also
  receives — assert a *successful* delivery for both, otherwise an over-strict direction check looks
  like a pass.
- **Nested-path routing needs a decoy to be a real test.** Declare `port p { port q; }` *and*
  `port other { port q; }`, `connect p.q to sink`. Two runs: `send via p.q` → reader gets the value;
  `send via other.q` → `port "other.q" is joined to no port that can receive it` **and** the reader's
  attribute stays at its initial value. Without the decoy the test passes on a build that routes by
  bare last segment.
- **Contrast binary is very cheap here and worth it** (`git worktree add /tmp/old<sha> <sha>`): on the
  pre-fix commit the direction fixture *completes silently* (message delivered nowhere, no error) and
  the nested/part-level fixtures end in `accept deadlock in action ship: nothing can post the awaited
  message (…)`. Those two failure shapes are the proof that the new run means something.
- **`connect a, b, c;` is a parse error** (`expected 'to' between connector ends`); the n-ary form is
  `connect (a, b, c);`. If a multi-end routing test "fails" with `joined to no port that can receive
  it`, check the load diagnostics first — the connector may never have lowered.
- `connect a to a;` (self-connection) reports `port "a" is joined to no port that can receive it` (the
  sending port is excluded from its own receivers), and an end naming a nonexistent feature
  (`connect a to nosuchthing;`) is *deliberately* treated as able to receive, so the action completes
  with no error and no panic. Assert both — they are the "must not crash" cases.

### Routing from a state machine declared inside a part (PR #195 follow-up, 8cffc9e)

`send … via p` inside a `state machine` nested in a `part def` has to walk out through state,
region and transition scopes to reach the part's `connect`. Testing it needs shapes the REPL will
actually descend into:

- **Fixture shape:** `part def Radio { port src : PingPort; port dst : PingPort; connect src to dst;
  state machine { attribute received : Integer = 0; … } }` with `port def PingPort { in item ping :
  Ping; }`. Assert on `%current`'s `State data:` block (`received = <n>`), and drive with
  `%state <Pkg>::<Part>::machine` — a bare `%state Radio::machine` fails with
  `unresolved reference`, the name must be fully qualified (or just `machine` if it is the only one).
- **Cover both send sites**: a state entry (`state waiting { entry send Ping() via src; }`) *and* a
  transition effect (`transition go first waiting do send Ping() via src then sent;`). At the
  pre-fix commit the entry form can already work while the **transition effect** fails with
  `error: event processing failed: transition effect: send reaches no receiving port: port "src" is
  joined to no port that can receive it` — so the transition-effect case is the discriminating one.
- **Nesting: `initial`/`then` inside a composite state does not descend** in this build — a
  `state outer { initial s0; state a { entry … } s0 then a; }` machine stays in `outer` and the inner
  entry never runs (reproducible with no part/ports involved, so it is a pre-existing state-machine
  limitation, not routing). Use the **entry-succession form instead**, which does descend and shows a
  `State stack (active configuration): 0. outer / 1. a` in `%current`:
  `state machine { entry; then outer; state outer { entry; then a; state a { entry send … via src; } … } }`
  A `region main { initial start; … }` wrapper also works with plain `initial`.
- **Add a negative for over-reach:** a machine at package level with an unconnected port, and a
  second part with its own `connect ox to oy`. The send must still fail with the typed error — if the
  scope walk goes too far it could pick up the unrelated part's connectors.

## Driving a state machine and its transition effects on camera

- **`-e` is not a file flag.** `sysml -e <expr> [file]` evaluates an expression; to load a model
  non-interactively use `sysml file.sysml` (it loads, then starts the REPL, so pipe `%quit` in) or
  `%load` from the prompt. Passing a path to `-e` yields the misleading
  `error: no declarations loaded (literals work, but feature references need declarations)`.
- **`sysml -trace <file>` plus `%state <name>` then `%advance <n>` is the strongest available
  evidence that a transition effect ran and in what order.** The trace prints
  `exit: <src>` → the effect's `eval …` lines → `enter: <tgt>` → `transition: src -> tgt`, so an
  effect that was parsed but dropped is visible as a missing `eval operator …` line. Note the
  trace of a state machine's own attribute is *not* readable with `%eval <attr>` afterwards —
  that reports the declared default (e.g. `= 0`), not the executed value, so assert on the trace
  line (`eval operator + -> 1`) instead of on `%eval`.
- **`%state` works on a `state def` as well as a state usage** (`%state P::S`); the executor starts
  in whatever state the `entry; then <s>;` chain reaches.
- **Conformance fixtures under `internal/core/runtime/testdata/conformance/` often write bare
  `Integer`**, which the conformance harness resolves but the REPL does not: loading them prints
  `error: unresolved reference: Integer`. That is REPL-only noise, not a regression — when a test
  asserts "loads with no diagnostics", copy the fixture through
  `sed 's/\bInteger\b/ScalarValues::Integer/g'` first.
- **A `do perform <Action>` effect needs the action to have a body.** `action def Bump;` with no
  body parses fine but fails at run time with
  `event processing failed: transition effect: invoke action Bump: initialize action: no initial
  node found in action Bump` (the constructor-succeeds/`initialize()`-errors contract). For an
  executing `perform` effect use a fixture whose action has `first`/`then` nodes, e.g.
  `conformance/state_transition_effect_perform.sysml` (counter 1 → 11).
- For transition-terminator/`;` work specifically, the discriminating inputs are:
  `transition a to b do assign x := x + 1;` (compact), `transition first a do … then b;`,
  `do { … ; };` (braced), `do perform A;`, plus the negatives `;;`
  (`error: expected a body member`), no `;` (both `expected ';' after assignment` *and*
  `expected ';' after transition`), braced with no trailing `;`
  (`expected ';' after transition`), and a nested statement inside a braced effect missing its own
  `;`. An A/B against a binary built from the parent commit is what separates "fixed" from
  "never broken" here.

## Reading a state machine's attribute values, and notation that actually drives it

Discovered while testing inline `entry action { … }` bodies and calc `out` assignment (PR #135).

- **`%current` is the way to read a state machine's attributes.** It prints a `State data:` block
  (`c = 6`) plus the active configuration / state stack for composite and orthogonal machines.
  `%eval <attr>` is *not* a substitute: it reports the declared default, and for a name declared in
  two `state def`s of one package it fails with
  `error: symbol "c" is ambiguous: P::S1::c, P::S4::c (use a qualified name)`.
  `%advance <n>` additionally prints `Do behavior actions run: <k>`, which is how you tell a do
  behavior actually ran from a machine that merely idled (`No pending work - simulation time is now …`).
- **`transition first a accept when true then z;` never fires.** A `when` trigger is a change
  trigger evaluated on an event, and a machine with no events sits in `a` forever — so a fixture
  written that way silently tests *only* the entry behavior and never the exit behavior. To exercise
  exit behaviors and ordering, use **completion transitions**: `initial start; … then start work;
  then work done;` (the `state_anonymous_action_body.sysml` conformance fixture is the model to copy).
- **An inline body is one action per do round.** After a do body has run to its end the state has
  no more pending work, so further `%advance` calls do not re-run it; a counter incremented by a
  `do action { … }` reaches 1 and stays there unless a transition re-enters the state. The
  one-action-per-statement `do { … }` form is what interleaves and re-runs per statement.
- **Notation gotchas that cost fixture rewrites:**
  - a self-send must name the machine, statement style: `entry action { send Ping to Driver1; }`
    with `item def Ping;` — `send Sig() to self` with an `attribute def` parses but never delivers.
  - inside a body write `perform Work;`, never `perform Work();`
    (`error: expected ';' or '{' after 'perform' action reference`).
  - an action a `perform` targets needs `first`/`then` nodes, else
    `initialize action: no initial node found`.
  - `then work done do assign x := 1;` does not parse (`expected ';' after succession edge`);
    use the `transition work to done do assign x := 1;` form for an effect.
- **`perform <Action>;` inside an inline body** works as of PR #135 (before it, it failed with
  `state behavior <anonymous>: action usage "Work" in a body is not executable` while the same
  `perform` in the statement-form body `entry { … perform Work; }` executed). It is a good
  discriminating fixture whenever body lowering changes.
- **Calc `out` features:** read them through a usage (`calc c : Def { in n = 5; } attribute a :
  Integer = c.a;`) and inspect with `%features <part>`; a failing read shows inline as
  `a: <error: slot p.a: …>` rather than aborting the listing, so one `%features` can carry several
  independent negative assertions. Useful expected strings: `no value: output never assigned`,
  `output bound more than once` (declaration value plus a body assignment; two *body* assignments
  are legal, the last one winning), `assignment outside the calculation body: <name> is not declared
  by the calculation`, `no result expression: calc … has no return expression`.

## "Did you mean" suggestions on unresolved references (PR #167)

The suggestion is produced in the resolver (`internal/core/suggest` + `internal/core/resolve/suggest.go`),
so it belongs to the diagnostic and every surface renders the same string. Verify all three surfaces —
they used to disagree, and the REPL used to post-annotate its own copy:

```bash
./bin/sysml -validate f.sysml            # CLI: "... — did you mean ScalarValues::Integer?", exit 2
printf 'part def A { attribute x : Integer = 1; }\n' | ./bin/sysml | grep -c 'did you mean'   # must be 1
```

- Canonical fixture: `part def A { attribute x : Integer = 1; }` → `unresolved reference: Integer —
  did you mean ScalarValues::Integer?` and exit `2`. A bare library name is *supposed* to fail:
  only top-level packages are in the root namespace, so the fix is `private import ScalarValues::*;`
  inside a `package`, which must then validate `✓ … no errors` / exit `0`.
- **Duplication is the regression to watch in the REPL.** Count `did you mean` occurrences rather
  than eyeballing them, and compare the text after `error: ` with the CLI's byte for byte. A
  parent-commit binary is the cheapest contrast: pre-fix the CLI prints *no* suggestion while the
  REPL prints one, so "old REPL also showed a hint" is expected and not evidence of duplication.
- The LSP path is checkable in ~15 s without VS Code: drive `./bin/sysml-lsp` over stdio with a
  three-message script (`initialize`, `initialized`, `textDocument/didOpen`) and read
  `textDocument/publishDiagnostics`; the message must match the CLI exactly.
- Shapes with deliberately different behavior, all worth asserting:
  - a **qualified** unresolved name gets NO `did you mean` — it reports
    `(no namespace "Nowhere" is loaded; "Integer" is declared as …)` instead;
  - operator members are never offered (`typable()` filters `IntegerFunctions::+`, `#`, `..`), so a
    typo like `Plu` yields no suggestion while `Abz` → `RealFunctions::abs`;
  - a long garbage identifier (≥ 100 chars) must yield zero suggestions and still exit `2`;
  - edit-distance tolerance widens with length (1 / 2 / 3 at ≥ 6 / ≥ 9 runes), so a 6-char typo like
    `Intger` legitimately offers noisy extras (`Items::Item::boundingShapes::edges::inter`,
    `VectorFunctions::inner`) after the right answer — assert the *first* candidate, not the list.
- **`examples/` is a gate.** All 20 files under `examples/` (excluding `examples/sysml-v2-training`)
  must validate with exit 0:
  ```bash
  for x in $(find examples -name '*.sysml' -o -name '*.kerml' | grep -v sysml-v2-training); do
    ./bin/sysml -validate $x >/dev/null 2>&1 || echo "FAIL $x"; done
  ```
- Fixtures for the parser/semantics fixes shipped alongside (each fails on the parent commit):
  `binding b of f = v;` (old: `type must be a definition, found attributeUsage`),
  a **constraint usage** — not `constraint def` — declaring `in` params plus `assert`/`assume`
  conditions (old: `expected '{' or ';'`), and
  `vertices->ControlFunctions::exists{p : Point; p.x > 0}` (old: `no scope for member lookup in p`).
- **REPL adversarial gotcha:** typing unbalanced braces (`part def }}} {{{ ;;;`) puts the prompt into
  multi-line continuation (`...>`) and silently swallows subsequent lines. Press `ctrl+c` to abandon
  the buffer — it flushes the accumulated parse errors and returns a clean `sysml>` prompt, which is
  itself a good no-panic assertion.

## Constraint/requirement checks: which object the verdict is about (PR #176)

`-constraint`/`-requirement` (and `%constraint`/`%requirement`) pick their subject in this order:
the object instantiated under the name the condition was reached by, else the *single* session
object whose type conforms to the type declaring the condition, else declared defaults.

- The suffix is the tell: a verdict about an object reads
  `✗ Constraint P::Sensor::inRange failed (on P::hot ID: 1)`, a defaults-only verdict has **no
  `(on …)` suffix**. Assert the suffix, not just ✓/✗ — a defaults evaluation of a fixture whose
  defaults happen to hold looks like a pass either way.
- Exit statuses (`cmd/sysml/status.go`): 0 holds, 1 fails, **2 unevaluable/unresolved**. The
  ambiguity answer is exit 2 with the message
  `X is carried by more than one object of this session (a, b): check it on one of them, …`,
  written to **stderr** with a `sysml:` prefix (`error:` in the REPL) and with **no ✓/✗ line** —
  so a build step never reads a verdict that was not decided.
- Build a fixture where the *def* declares a default that satisfies the condition and the *usage*
  redefines it to violate it (`attribute reading = 50.0` vs `:>> reading = 150.0`). Without the
  default, the pre-fix binary errors with `unresolved reference: reading` instead of the silent
  `✓ passed`, and the contrast is much less convincing.
- Carriers include subtypes: `part def Heavy :> Sensor`, `part hotter :> hot`, an abstract def's
  usage, and a `variation part` all count — instantiating two of them makes the check ambiguous.
  An instance of an unrelated type does **not** count and leaves the defaults path.
- Since PR #180 a condition that **could not be evaluated** (uninitialized slot, unbound subject)
  prints `? <what> could not be evaluated` + `  Error: <why>` and exits **2**; it keeps the
  `(on …)` suffix when it is about an object. Before #180 it printed `✗ … failed` / `✗ … fails`.
  Assert all three states in one pass — `✓ … passed` exit 0, `✗ … failed` exit 1 (with
  `Assertion evaluated to false: <condition>`), `? … could not be evaluated` exit 2 — and grep the
  **verdict line only** for the old wording: the `Error:` reason legitimately contains the words
  "evaluation failed", so a bare `grep -c failed` over the whole output is a false positive.
  `%features` mirrors the three states as `<constraint: satisfied>` / `<constraint: violated>` /
  `<constraint: not evaluated: …>` (the `not evaluated:` prefix is #180's). `-json` reports
  `"status": "unresolved"`, `"exit": 2` and the same `?` line.
- CLI checks refuse to run at all on a model that does not analyse cleanly
  (`… did not analyse cleanly; no check was made`, exit 2), so an unevaluable-verdict fixture must
  itself be error-free — put the `assert nonexistent > 0` style constraint in a *separate* file
  from the unbound-subject requirement, or the CLI never reaches the verdict.
- PR #180 made `%eval` pick the same subject as the checks (`Session.subjectFor`): with `P::hot`
  instantiated, `%eval P::Sensor::reading` answers `= 140.00 (on P::hot ID: 1)`; with `P::hot` and
  `P::cold` both instantiated it refuses with `error: … is carried by more than one object of this
  session (P::cold, P::hot): name one of them, …`; with nothing instantiated it answers the
  declared default and prints **no** `(on …)`. Subtype carriers (`part hotter :> hot`) work too.
- Still a blind spot after #180: a **nested part** whose value is redefined on the instantiated
  object is not the subject. With `part o : Outer { part :>> b { attribute :>> c = 99.0; } }`,
  `%features A::o` shows `c = 99.00` and `small: <constraint: violated>`, yet `%eval A::Outer::b::c`
  answers `= 5.00` and `%constraint A::Inner::small` says `passed` (identical on the parent
  commit, so it is pre-existing, not a regression — but it reads as a contradiction and may be
  worth flagging). Also `%eval A::b::c` (skipping the def segment) is `unresolved reference` in
  both revisions; the resolvable spellings are `A::Outer::b::c` and `A::Inner::c`.
- **The nested-subject blind spot above was fixed (PRs #206/#219/#236).** Now `%constraint`,
  `%requirement` and `%satisfy` evaluate the nested object and *name* it: with
  `part o : Outer { part :>> b { attribute :>> c = 50.0; } }`,
  `%constraint A::Inner::small` answers `✗ Constraint A::Inner::small failed (on A::o::b ID: 2)` —
  `<session-held root>::<feature path>`, matching the `b = Instance(ID: 2)` line of `%features A::o`.
  Assert the *whole* suffix: a build that regressed to the outer holder still prints a `✗`, and the
  pre-#236 binary prints the same `✗` line with **no suffix at all**, so ✓/✗ alone proves nothing.
  Ordinary (non-nested) checks keep labelling the object handed in (`(on A::w ID: 3)`), and a
  no-object run still prints no suffix.
  - Such a label is **descriptive, not addressable**: `%features A::o::b` answers
    `error: no instance of "A::o::b"` and `%instances` lists only `A::o`. Worth reporting whenever
    label spelling changes, and a good adversarial step after any "name the subject" PR.
  - Multiplicity-materialized subjects read `(on A::car::wheels ID: 2)` in the REPL while gRPC still
    reports the *definition* (`instance_type_id 'A::Bolt'`/`'A::Wheel'`) — the two surfaces disagree
    by design so far; check both rather than assuming one from the other.
- **Ambiguity carrier labels: check every fixture shape, they regress independently.** The message is
  `ambiguous subject: <cond> is carried by <Def> #<id> (<feature path>), …`. Three shapes worth
  running together, because a change that improves one can flatten another:
  `front`/`rear` nested parts → `Bolt #4 (front::bolt), Bolt #5 (rear::bolt)` (pre-#236 both read
  `(bolt)`); subsetting into a shared collection (`part subsystem : Component[*]` fed by
  `part small : Component :> subsystem` / `large`) → at `658076e9` `Component #2 (small),
  Component #3 (large)` but at #236 both read `(subsystem)`, because the label now shows the
  features *walked* (the collection slot) rather than the declaration materialized; recursive
  containment → `Leaf #2 (leaf), Leaf #5 (next::leaf)`. Always time the recursive/mutual fixtures
  (`time ./bin/sysml recursive.sysml < cmds`, ~0.22 s) whenever the occurrence key changes.
- **gRPC classifies an ambiguity since PR #236:** `FailureReason.FAILURE_REASON_AMBIGUOUS_SUBJECT`
  (enum 3, `api/proto/sysml.proto`). pysysml exposes **no public property** for it — read
  `verdict._pb.failure_reason` and name it with
  `from pysysml.proto import sysml_pb2 as pb; pb.FailureReason.Name(...)`; `Verdict` has no
  `failure_reason` attribute and `connection._failure_of` maps only `WRONG_KIND`, so a client
  wanting to branch on ambiguity today reaches into a private field. An ordinary violated condition
  comes back `holds=False` with `FAILURE_REASON_UNSPECIFIED` (0), i.e. "plain violation" is the unset
  value — assert the ambiguous *and* the violated case in the same run so a hard-coded reason cannot
  pass both.
- `-instantiate` objects are created **before** `-e` expressions since PR #180, so
  `sysml -instantiate P::hot -e P::Sensor::reading m.sysml` prints the instance line first and
  answers `140.00 (on P::hot ID: 1)`; the parent binary answered `0.00`. Any change to
  `cmd/sysml/check.go`'s ordering needs the other combinations re-walked: multiple `-e` (still in
  flag order), `-e` with `-constraint` (exit 1), a failing `-e` (aborts with exit 2 before later
  `-e`s), `-validate`, and `cat m.sysml | sysml -e '1+1'` (piped stdin works for `-e`, but named
  checks over piped stdin refuse with `no model to check; name the file …`).
- Konsole typing trap: the `✗` glyph does not survive the computer tool's `type` action into a
  shell command (it arrives empty, and `grep -e ''` then matches every line). Build the pattern in
  the shell instead: `X=$(printf '\u2717'); … | grep -nE "$X|could not be evaluated"`.
- `%features <usage>` is the independent oracle — `inRange: <constraint: violated>` must agree with
  the verdict for the same object. Declaring anything new in the REPL drops all instances, so a
  following check silently reverts to defaults (assert the missing `(on …)` suffix there).

## A lone `-` as standard input (PR #179)

`internal/core/project.ReadFile` reads `os.Stdin` once (memoized with `sync.Once`) whenever a path
is exactly `-`, names it `<stdin>` in diagnostics, and refuses a terminal with
`standard input is a terminal; redirect it or name a file`. That makes stdin usable in every
path-taking mode: `-validate`, `-e`, `-calc`, `-constraint`, `-action`, `-state`, `-convert`, and a
plain positional load.

The convincing test is a **paired run**: `bin/sysml -validate m.sysml` against
`cat m.sysml | bin/sysml -validate -`, for a clean *and* a broken model. Everything but the model
name must match, including the line:column of each diagnostic and the exit status (0 / 2). Build the
parent revision (see the contrast-binary recipe) to show the same pipe answering
`sysml: cannot read -: no such file or directory` before the change.

Cases worth walking, with what they should answer:

| command | expected |
|---|---|
| `cat m | sysml a.sysml - -validate` | `✓ a.sysml, <stdin>: no errors` — flags may follow the paths |
| `cat m | sysml -validate - -` | `✓ <stdin>, <stdin>: no errors`, exit 0 (one memoized read, no hang) |
| `: | sysml -validate -` | `✓ <stdin>: no errors`, exit 0 — empty stdin is an empty model, not an error |
| `head -c 200000 /dev/urandom | sysml -validate -` | ~1800 `<stdin>:L:C: error: expected a namespace member`, exit 2 |
| `sysml -convert ttl -` | refused: `standard input carries no file name to take the format from`, exit 2 |
| `cat m | sysml - -convert ttl -from sysml` | conversion on stdout; `-o f` writes `wrote f (ttl, N bytes)` |
| `cd d && sysml -validate ./-` | reads a file *really* named `-`; a bare `-` in that directory still reads the pipe |
| `%load -` at the interactive prompt | `error: cannot read <stdin>: standard input is a terminal…`, and the REPL keeps going |

Two traps when testing this:

- **Always wrap in `timeout`.** The whole risk of the change is a run that blocks on a stdin nobody
  redirected. For the terminal case a pipe is not enough — give the process a real tty with
  `script -qec "timeout 10 bin/sysml -validate -; echo TTYSTATUS=\$?" /dev/null </dev/null`; it must
  print the refusal immediately with status 2.
- **Do not read the exit status through `| head`.** `$?` is then `head`'s, and `${PIPESTATUS[0]}` is
  `cat`'s; a SIGPIPE kill shows up as 141 and looks like a defect. Redirect to files
  (`>out 2>err; echo $?`) and grep them instead.
- Piping a *model* into a plain `sysml -` also consumes the prompt stream, so the REPL prints its
  banner and exits at once with the load's status — expected, not a hang. Inside a piped prompt
  session, `%load -` swallows the remaining script lines as model text, which is worth noting but is
  inherent to one stdin.

## `sysml-lsp` command line (PR #179)

`cmd/sysml-lsp` parses args with a `flag.FlagSet`: `-version`/`--version`/`-v` print the version and
`-h`/`-help` the usage, both on **stdout** with status 0; an undefined flag or a stray positional
argument prints `flag provided but not defined: …` / `sysml-lsp: unexpected argument "…"` plus the
usage on **stderr** with status 2, and nothing on stdout. Assert the stream split
(`>out 2>err`, then `wc -c <out`), not just the text — the point of the change is that a misuse no
longer enters protocol mode and dies with `failed reading header line: EOF` (which is what the parent
binary does, exit 1).

With no args it still speaks LSP. Drive it from Python: write
`Content-Length: N\r\n\r\n<json>` frames to stdin, read the header line, blank line, then N bytes.
`initialize` must answer `"capabilities"` with `completionProvider`, and `shutdown` `{"result":null}`.
**Pre-existing, not a regression:** after the `exit` notification the process only ends when stdin is
closed, and then reports `sysml-lsp: failed reading header line: EOF` with status 1 — identical on the
parent binary, so verify against the contrast binary before reporting it.

## Static dimension (unit-commensurability) warnings (PR #184)

`checkDimensions` (`internal/core/passes/typecheck_dimension.go`) emits a **type-tier warning** for
`+ - < > <= >= == !=` when both operand dimensions are statically known and incommensurable, e.g.
`operator '<' combines incommensurable quantities: ISQBase::MassValue (dimension M) and m (dimension L)`.
It is a warning, so `-validate` still exits **0**; evaluating the same constraint still fails with
`incommensurable units: cannot express m (SI::metre) in kg (1000·SI::gram)` and exit **2**. `-quiet`
suppresses it; `-json` reports it as `"severity":"warning","pass":"type","code":"type.expr"` with
`"exit": 0`.

Fixture gotchas that cost time here:

- Every fixture needs `public import ScalarValues::*;` if it mentions `Real`/`Boolean`, else
  `unresolved reference: Real — did you mean ScalarValues::Real?` aborts before any check runs
  (`did not analyse cleanly; no check was made`, exit 2) and the test proves nothing.
- Unit names are the stdlib's: `t`, `deg`, `degC` do **not** resolve; `°`/`°C` do not even lex in a
  `[...]` unit position. Use `g`, `K`, `rad`, `Hz`, `Bq`, `N * m`, `J`. `ISQ::TemperatureValue` is an
  **alias** for `ISQBase::ThermodynamicTemperatureValue`.
- **Naming an attribute `m` shadows `SI::metre`**, so `1.0 [m]` inside that scope is not a quantity
  at all and every dimension check there goes silent. The runtime says so explicitly
  (`m resolves to the attributeUsage m declared in …, shadowing the measurement unit SI::metre`).
  Never name a fixture attribute after a unit unless shadowing is the thing under test.

Cache interaction: `libs.formatVersion` bumped 15→16 and the cache persists `DimensionFacts`
(`~/.cache/sysml-ls/libs`, or `$XDG_CACHE_HOME/sysml-ls/libs`). To exercise both paths,
`rm -rf ~/.cache/sysml-ls/libs` and run **the very first** validation on the file you care about —
only that first run is cold, since it repopulates the cache for every later file in the same loop.
Known cosmetic divergence: cold names the quantity type by leaf name (`MassValue`), warm by
qualified name (`ISQBase::MassValue`); assert verdicts and exit codes rather than byte-identical text.

Silent-by-design shapes (assert zero `warning:` lines): commensurable comparisons (mm vs m, m/s vs
km/h), dimensionless arithmetic, `xs#(1)` indexing, an untyped attribute later `assign`ed a quantity,
a calc *invocation* result operand, and a calc body written without `return` (`calc def C { in q : …;
q < 1.0 [m] }` never warns, while the `return : Boolean = …` form does). A **typed** parameter
(`in q : ISQ::MassValue`) *does* warn — its dimension is statically known — and an operand whose type
is reached through a stdlib **alias** currently does **not** warn even though the un-aliased twin does.

Cheap false-positive sweep, worth running for any diagnostic-adding pass:

```bash
for f in $(find examples testdata -name '*.sysml'); do ./bin/sysml -validate "$f" 2>&1 \
  | grep 'incommensurable quantities'; done   # expect no output (403 files, ~90 s)
```

## Verifying hand-written release notes against built binaries

When asked whether release notes are honest, judge each sentence separately and re-measure every
number rather than quoting the table.

- **Transcripts are layout-sensitive.** A quoted diagnostic like `model.sysml:6:9: warning: …` is
  reproducible only if the fixture puts the offending expression on that exact line/column. Put the
  comparison on its own line at the quoted column (`constraint c {` on the line above) and copy the
  fixture to the quoted file name (`/tmp/model.sysml`) before deciding a transcript is wrong.
- **Model sweeps: quote your loop.** `examples/sysml-v2-training/*` directory names contain spaces,
  so `for f in $(find examples -name '*.sysml')` word-splits and inflates the count (395 tokens for
  110 files) while every conversion fails on the broken path. Always use
  `while IFS= read -r f; do … done < <(find examples -name '*.sysml' | sort)`.
- **Round-trip claims need a validity control.** Many training files do not validate standalone
  (their dependencies live in sibling files), so a ttl→sysml re-validation failure is not proof that
  the round trip is lossy. Measure three numbers: converted, converted-and-original-validates, and
  round-tripped. Over the 120 `.sysml` + `.kerml` models under `examples/` at 0.0.8 that is
  71 / 44 / 44 (65 / 38 / 38 counting the 110 `.sysml` files alone — state the denominator, since
  the published limitation counts both languages).
- **Counted rows go stale fast, and the Python row depends on the environment.** With no service
  listening `pytest python/tests/ -q` was 369 passed / 26 skipped at 0.0.8; with a service already
  listening the integration tests run instead of skipping (since PR #204 nothing fails either way,
  and CI now starts a service). Say which way a row was measured. `go test -race -count=1 ./...`
  was 3,682 pass / 5 skip / 3,687 total at 0.0.8, and 4,440 / 7 / 4,447 at 0.0.9 — recount rather
  than trusting either. Count **first-level** subtests: `variant_connection_per_owner` registers one
  sub-subtest per owner, so counting every `=== RUN` line reported 299 conformance cases where 297
  fixtures exist, and inflated the robustness row from 173 to 199. Four documents are allowed to
  state counts (`docs/project/spec-compliance.md`, `README.md`, `docs/project/roadmap.md`,
  `docs/project/training-examples.md`) and must be updated in one commit; anywhere else, link to
  spec-compliance rather than adding a fifth copy.
- **Error-class claims: check the export path.** A class can exist in `pysysml.errors` and be absent
  from the package surface — `hasattr(pysysml, name)` is the check, and
  `TestPackageSurface` in `python/tests/test_errors.py` now locks every exception in
  `errors.__all__` onto `pysysml`.
- **LSP capability claims** are cheap to check with a framed JSON-RPC driver: assert
  `semanticTokensProvider` has `full: true`, `range: true` and no `delta` key anywhere, that
  `range` returns strictly fewer tokens than `full`, that `semanticTokens/full/delta` answers
  `-32601` and the server still serves a later request, and that each code action carries a
  `WorkspaceEdit` whose `newText` actually fixes the file. A missing-semicolon fix needs a fixture
  the parser recovers with a fix on — `action def A { first start }` yields `Insert ';'`; a plain
  attribute declaration yields a diagnostic with no fix.

## Subject-aware `Evaluate`, attribute metadata and generated classes (Track P, PR #218)

- **`pandas` is not installed by the blueprint but `Symbol.to_dataframe()` needs it.** If
  `to_dataframe()` raises `ModuleNotFoundError: No module named 'pandas'`, run
  `$HOME/pv/bin/pip install pandas -q` first — the failure is environmental, not a product defect.
- **Subject evaluation is the difference between the *declared default* and the *object's slot*.**
  A fixture that distinguishes them is mandatory: `part def Vehicle { attribute mass = 1500.0; }`,
  `part def Sedan :> Vehicle { attribute :>> mass = 1200.0; }`, `part sedan : Sedan;`. Then
  `m.eval('mass', context_symbol_id='Demo::Vehicle')` must be `1500.0` while
  `m.eval('mass', subject='Demo::sedan')` must be `1200.0`. Without the redefinition both paths
  return the same number and the test proves nothing.
  - On `Model` the kwarg is `subject=`; on `Connection.eval` it is `subject_symbol_id=` and the
    model hash comes **second**. Module-level `pysysml.evaluate` also takes `subject=`; the
    deprecated `pysysml.eval` forwards to it and warns `DeprecationWarning`.
  - A `subject=` plus an explicit `context_symbol_id=` still reads the object, and derived slots
    (`attribute doubled = mass * 2.0`) follow the object too.
  - A *definition* as subject is accepted (it is instantiated and reads its own redefinition), and an
    unrelated subject (e.g. an attribute) is tolerated rather than rejected — record those as
    observed behavior, not failures. An unknown FQN gives `ExecutionError: subject not found: <fqn>`.
  - A subject with a cyclic slot raises `ExecutionError: ... cyclic slot dependency: <feature>` and
    the service must still answer afterwards — always follow such a case with `m.eval('1+1') == 2`.
- **Cross-checking against the REPL needs fully-qualified expressions.** `%eval mass` on a model with
  several `mass` features fails with `symbol "mass" is ambiguous`, whereas gRPC `subject=` resolves
  the unqualified name inside the subject's scope. Use `%instantiate Demo::sedan` then
  `%eval Demo::sedan::mass` / `%features Demo::sedan` to compare; that is a surface divergence in name
  resolution, not a wrong value. Drive it non-interactively with
  `printf '%%load …\n%%instantiate …\n%%eval …\n%%quit\n' | ./bin/sysml`.
- **Attribute metadata checks worth making**: own attributes come before inherited ones; a
  redefinition masks the attribute it redefines (`Sedan.mass` = 1200.0, not two `mass` rows);
  `attributes()` ids point at the *declaring* supertype (`Demo::Vehicle::name`); a **non-constant**
  default such as `mass * 2.0` must report `value=None` rather than a guessed number; a quantity
  default `120.0 [kg]` reports value and `unit='kg'` separately; an element with no attributes yields
  `[]` and an empty DataFrame that still has all 8 columns
  (`name,kind,id,type,multiplicity,value,unit,inherited`).
- **Stdlib elements themselves are not reachable through `Model.find`/`m['ISQ::MassValue']` (returns
  `None`)**, so "attributes of a stdlib element" can only be covered indirectly: declare
  `attribute m : ISQBase::MassValue = 3.0 [kg];` in your own model and assert the *resolved* type
  string `ISQBase::MassValue`.
- **Generated classes**: `python -m pysysml.generate model.sysml -o out.py` then actually `import` the
  module — an MRO bug only surfaces at import as
  `TypeError: Cannot create a consistent method resolution order`. Assert `cls.__mro__` names against
  the model (include multiple supertypes and a diamond), that a `:>>`/`subsets` feature inherits the
  type/multiplicity it does not restate (`String[0..*]` → `-> list[str]`), and that any base Python
  cannot linearize, or that has no generated class, is dropped **with a comment** such as
  `# specializes G2::Hybrid, left out: Python cannot linearize it with the bases above` /
  `# specializes ISQBase::MassValue, which has no generated class`.
- **Quantity results carry the scalar on `Quantity.magnitude`, not `.value`** — a probe using `.value`
  reports a false failure even when the runtime is correct.
- **A stale `~/.pysysml/bin/sysml-grpc` silently blocks the subject/attribute surface.** These features
  are capability-gated (`evaluate_subject`, `symbol_attributes` in `pysysml/capabilities.py`), so a
  service built before they landed makes `conn.eval(..., subject_symbol_id=…)` /
  `attribute_facts()` / `to_dataframe()` raise `MissingCapabilityError` instead of answering — which
  looks like a client bug. Always reinstall the binary before testing a merge:
  `make build-grpc && pkill -x sysml-grpc && rm -f ~/.pysysml/sysml-grpc-50051.pid ~/.pysysml/sysml-grpc-50051.lock && cp bin/sysml-grpc ~/.pysysml/bin/`
  (the `cp` fails with `Text file busy` while the old one still runs), then assert
  `sorted(conn.server_info().capabilities)` contains both names before trusting any result.
- **The auto-started service dies with the session that started it.** After an interactive
  `pysysml` REPL exits, `PYSYSML_REQUIRE_SERVICE=1 pytest tests/` aborts during collection with
  `$PYSYSML_REQUIRE_SERVICE is set … but none answers on localhost:50051`. Start one yourself first:
  `nohup ~/.pysysml/bin/sysml-grpc -port 50051 >/tmp/svc.log 2>&1 &`.
- **`PINNED_SHA256` is nested `repo -> version -> asset`.** To exercise the *contradicted* digest arm
  you must inject the key for the repository actually in use, e.g.
  `binary.PINNED_SHA256['Open-MBEE/OpenSysML'] = {'v0.0.8': {'sysml-grpc-linux-amd64': 'de'*32}}`
  (`DEFAULT_GITHUB_REPO`, looked up case-sensitively, so the spelling must match exactly);
  a mis-cased, flat or `in`-substring patch leaves the pin absent and you silently re-test the *unpinned* arm
  (`UnpinnedReleaseError` + kept cache) while believing you tested the contradiction.

## Symbol *kind* changes: `%search` is the only REPL probe (PR #210)

When a change moves a declaration from one `symbols.SymbolKind` to another (modifier-driven usage
kinds, KerML classifier classification, `classifyUsage` edits), `-validate` proves nothing: a model
can be clean on both revisions while every kind is wrong. The probe is the REPL's
`%search <prefix>`, which prints `<fqn>  <kind>` from `sym.Kind.String()`
(`internal/repl/discover.go`, names in `internal/core/symbols/symbol.go`):

```bash
printf '%%load /tmp/m.sysml\n%%search Pkg::\n%%quit\n' | timeout 30 ./bin/sysml -quiet
```

- **Always search a `Pkg::` prefix, not a bare word.** `%search Observe` matched ~20 stdlib symbols
  (ISQLight, VerificationCases) and truncated with `(9 more; narrow the search)`, burying the
  model's own symbols. The trailing `::` also drops the package row itself.
- `%list` only echoes the submitted source text and `renderMember` never reaches action-body
  parameters, so it cannot show a parameter's kind. There is no `%kind`/`%hover` in the REPL.
- **Anonymous usages are invisible to `%search`** (no name to index). To show that
  `individual part : Vehicle;` or `in snapshot ;` still built something, use
  `./bin/sysml <m> -convert sysml -o /dev/stdout` and assert the member round-trips
  (`in snapshot;`, `in event;`, `in;`).
- Warning-only changes need the **exit code** asserted explicitly: `-validate` prints warnings and
  still ends `✓ no errors` with exit 0, so `echo "exit=$?"` is the difference between "warning" and
  "error" in the recording. Count them with `-validate f 2>&1 | grep -c 'reserved keyword'`.
- The A/B main binary is essential here: on `main` the same fixture printed
  `attributeUsage`/`partUsage` for `individual i : V` and `in snapshot atStart : Flight`, which is
  the only visible difference between fixed and broken.

Pre-existing traps worth not re-reporting as regressions:

- A bare `event e : O;` **declaration** reports `error: unresolved reference: e — did you mean
  Demo::e?` on every revision — `event <name>` is read as naming an existing occurrence. Use
  `occurrence takeoff : Flight; event takeoff;` for a clean fixture; the symbol is still built as
  `occurrenceUsage`.
- Fixtures using `Real`/`Integer` need `private import ScalarValues::*;` or `-validate` fails with
  `unresolved reference: Real` and masks the kind assertions.
- **Run the suite with a cold library index when you change classification.** The on-disk index
  (`$XDG_CACHE_HOME/sysml-ls/libs`) caches stdlib symbol kinds, so a warm cache keeps returning the
  old kinds and `go test ./...` passes locally while CI fails. Gate with
  `export XDG_CACHE_HOME=$(mktemp -d) && go test -count=1 ./...`; this is how PR #210's
  `function` → `kermlType` regression (every `IntegerFunctions` operator stopped being a calc,
  `internal/repl/discover_test.go` pinned `ScalarValues::Integer attributeDef`) stayed hidden.

## Multiplicity of a feature's default value (PR #199, Track A / A2)

A default bound to a feature is checked against the feature's multiplicity in **two independent
tiers**, and testing one proves nothing about the other:

- **Static (type tier)**: `sysml <m> -validate` reports counts it can see in the source, from
  `passes.checkValueCount` over `exactCount(u.Value)`. Wording:
  `N value(s) bound to a feature with multiplicity lower|upper bound M`, exit **2** with
  `did not analyse cleanly; no check was made`.
- **Runtime**: `Context.checkDefaultCount` fires only when the slot is **materialized**, i.e. when
  something reads it. Wording adds a prefix:
  `slot <Type>.<name>: multiplicity violation: N value(s) bound to …`.

Consequences worth checking on every change here:

- `%features <instance>` renders a bad slot inline as `name: <error: slot …>` and the REPL still
  **exits 0** — a violation is not a session error. For an exit status you need a run that reads the
  slot, e.g. `sysml m.sysml -instantiate test::bad -eval 'test::bad.few'` → exit **2** with
  `evaluation failed: slot bad.few: multiplicity violation: …`.
- `sysml m.sysml -instantiate X -validate` prints `no errors` and exits **0** even when every
  default in the model violates its multiplicity, because `-instantiate` creates the object without
  materializing its slots. Do not use it as the "does the runtime check fire" probe.
- **The two tiers can disagree, and a collection holding a reference is where they do.** `exactCount`
  flattens nested collection literals just as binding does, so
  `attribute nested : Real[4] = ((1.0, 2.0), (3.0, 4.0));` counts 4 statically and materializes as
  `[1.00, 2.00, 3.00, 4.00]` with no diagnostic; but a collection whose elements are references has
  no statically known count, so `attribute two : Real[2] = (src, src);` (with `src : Real[3]`)
  passes statically and errors at runtime with "6 value(s) … upper bound 2". Include both shapes in
  any regression fixture.
- A feature that declares **no** multiplicity is deliberately not held to the assumed `1..1`
  (`EffectiveFeature.MultiplicityStated`), so `attribute anyN = both.volume;` over a `[0..*]`
  sibling must show `[2.00, 3.50]` with no diagnostic. A test asserting an error there is wrong.
- Multiplicity is inherited through `:>>` when the redefinition does not restate it, in both tiers:
  `part def Base { attribute xs : Real[2]; }` + `attribute :>> xs = (1.0, 2.0, 3.0);` must
  `-validate` as an error (exit 2) — this is the cheapest static probe of that path, and it is
  clean on any build predating the fix.

Fixtures live in `internal/core/runtime/testdata/conformance/multiplicity_default_*.sysml`
(merged / composite / nonconforming / redefinition). Drive them over a pipe to discover values:

```bash
printf '%%load internal/core/runtime/testdata/conformance/multiplicity_default_merged.sysml\n%%instantiate test::ranges\n%%features test::ranges\n%%quit\n' | timeout 60 ./bin/sysml
```

Expected: `exact = [1.00, 2.00, 3.00]`, `star = [1.00, 2.00]`, `empty = []`, `plus = [5.00]`,
`masses = [1.00 [kg], 2.00 [kg]]` (units survive), and in the composite fixture
`stowed = [Instance(ID: 2), Instance(ID: 3)]` reusing **left/right's own IDs** rather than fresh
ones — comparing IDs is the only way to tell "held the objects the default names" from
"instantiated two new ones".

Two gotchas when recording this area in Konsole: the REPL does **not** expand shell variables, so
`%load $CONF/x.sysml` fails with a path error — type full relative paths; and `part derived : …`
draws a `"derived" is a reserved keyword` warning that is unrelated to the defaults under test.

## String values and StringFunctions (PR #211)

A model that declares `String`/`Integer`/`Boolean` attributes needs `import ScalarValues::*;` —
without it every type annotation reports `unresolved reference: String — did you mean
ScalarValues::String?` and `-validate` exits 2 before any string behavior runs, which looks like a
feature failure but is a fixture bug. Add `import StringFunctions::*;` to call `Length`/`Substring`
unqualified inside a model (the REPL's `-e` mode resolves those two unqualified with no import,
because they are aliased in `library_functions.go`'s unqualified map; the other StringFunctions
names always need the `StringFunctions::` prefix).

Discriminators that separate a working string runtime from a broken one:

- **Characters, not bytes.** `Length("héllo")` is 5 (6 bytes), `Length("日本語")` is 3 (9 bytes),
  `Substring("héllo", 2, 3)` is `"él"`. A byte-based implementation answers 6/9 or returns mojibake,
  so always use a non-ASCII fixture — an ASCII-only test passes either way.
- **Substring is 1-based and inclusive**, with `lower > upper` returning `""` rather than erroring,
  while `lower < 1` or `upper > Length` gives `sequence index out of range: function
  StringFunctions::Substring lower 0 is outside 1..5` and exit 2.
- **The `==` split is deliberate**: `"a" == 3` evaluates to `false` (it specializes
  `DataFunctions::'=='`), while the explicit `StringFunctions::'=='("a", 3)` is a type mismatch.
  Test both; only one of them being right is the likely regression.
- A string against an Integer/quantity/sequence must say
  `type mismatch: operator '<' is not defined for a string and a quantity` (naming both types).

### Driving these cases through a GUI terminal

- **xdotool cannot type CJK into Konsole** — `type` with `日本語` silently delivers nothing, so
  `Length("日本語")` arrives as `Length("")` and answers 0, which reads as a failure in the video.
  Latin-1 accents (`héllo`, `naïve`) do type fine. Put any CJK (and any quoted-operator name such as
  `StringFunctions::'=='`, which shell quoting mangles when typed inline) into a small
  `/tmp/*.sh` written from a tool shell, then `cat` it and `bash` it on camera — the `cat` shows the
  reviewer exactly what ran.
- At the `sysml>` prompt a bare expression like `("a","b")->includes("b")` is parsed as a
  declaration (`1:1: error: expected a namespace member`). Prefix expressions with `%eval`.
- `%features <PartDefName>` (not the part usage path) is what follows `%instantiate <PartDefName>`;
  `%features Msg::g` reports `no instance of "Msg::g"`.
- **Escapes are stored raw**, so `Length("a\"b")` is 4 and the value renders as `"a\\\"b"`. This is
  pre-existing lexer behavior (identical on the parent commit) — verify against a contrast binary
  before reporting it as a string-runtime defect.
- A 512 KiB string built by doubling inside a `for` loop (`acc = acc + acc` ×16, then
  `Length(acc)` → 524288) completes in seconds; use it as the cheap no-hang/no-quadratic check.
  Note that a calc whose body assigns to a `return acc` feature returns no value when *invoked from
  another calc's expression* (`calculation returned no value`), so compute the length inside the
  same body rather than wrapping the loop calc in another one.

## Runtime budgets, and `calc` recursion in particular (PR #198)

Every budget in `internal/core/runtime/budget.go` is reachable from the CLI as an env var and is
listed by `%budget` in the REPL — that meta-command is the cheapest proof a new bound was wired
into `internal/repl/meta.go`. `SYSML_MAX_CALC_DEPTH` (default 10000) bounds nested `calc`
invocations, i.e. recursion depth, and is the only budget with a **ceiling** (25000).

Four distinct surfaces to assert for any budget change, since they fail independently:

1. **Under the bound the run must succeed with the right number**, not merely "not error".
   `%calc P::sumTo 5000` → `= 12502500`. Deep-but-terminating recursion is fast (~0.3 s), so a
   long pause is itself a signal.
2. **Over the bound: a typed error, promptly, with a sane exit status.** Both
   `%calc P::spin 1` in the REPL and `./bin/sysml m.sysml -eval 'P::spin(1)'` (exit **2**, wall
   time well under a second). Wrap in `timeout` so a hang shows as exit 124, and read the output
   for `fatal error: stack overflow` — a Go runtime crash rather than a reported error is the
   failure mode the budget exists to prevent. Follow the REPL error with `%eval 1 + 1` → `= 2` to
   show the session survived.
3. **A low value must refuse input that otherwise evaluates** (`SYSML_MAX_CALC_DEPTH=50` on a
   200-deep recursion). Without this contrast the env var could be ignored entirely and the test
   would still look green.
4. **Invalid/out-of-range values are refused at startup**, before any model work, naming the
   variable: `=abc` → `is not an integer … (default 10000)`; `=30000` → `must be at most 25000 …`.
   Also assert the ceiling value itself (`=25000`) is *accepted* — an off-by-one in the comparison
   is otherwise invisible. Both refusals exit 2 and print no evaluation output.

Budget interactions matter: under `SYSML_MAX_STEPS=100` a runaway recursion must report the *step*
limit, not the depth limit. Whichever budget is spent first wins, so a change that reorders the
checks shows up here and nowhere else.

### Fixture notes for recursive `calc` models

- Bare `Integer`/`Boolean` do **not** resolve in a hand-written file (the conformance harness
  supplies the imports). Start the package with `private import ScalarValues::*;` or every line
  errors with `unresolved reference: Integer — did you mean ScalarValues::Integer?`.
- A recursive calc is just `calc f { in n : Integer; return : Integer = if n <= 1 ? 1 else n * f(n - 1); }`.
  An **implicit-result** body drops `return :` and ends in a bare expression:
  `calc f { in n : Integer; if n <= 1 ? 1 else n * f(n - 1) }`. Both must be exercised — they take
  different paths in `passes/typecheck.go` (`checkBehaviorMember`) and only the explicit one was
  type-checked before #198.
- Adversarial shapes worth having in one file: a non-recursive calc *usage* nested inside a
  recursive calc; recursion through a calc named like a library function (`max`); mutual recursion
  (`isEven`/`isOdd`, whose result for an odd argument is `false`).
- **There is no `%check` command.** To evaluate an `assert constraint` whose body calls a recursive
  calc, use `%instantiate <DefName>` then `%features <DefName>` — note both take the **def** name, not
  the usage (`%features P::w` answers `no instance of "P::w"`). A satisfied constraint renders
  `deepOK: <constraint: satisfied>`; a runaway one renders per-slot as
  `spinny: <constraint: not evaluated: … calc recursion limit exceeded …>` and the session survives.

### Type diagnostics on implicit-result bodies

`-validate` is the surface: an implicit result is typed like any expression, so
`calc def C { in n : Integer; n + "one" }` must report
`error: operator '+' is not defined for Integer and String` (exit 2) and an incommensurable pair
(`ISQ::LengthValue` + `ISQ::DurationValue`) must report
`warning: operator '+' combines incommensurable quantities: … (dimension L) and … (dimension T)`
(exit 0). Always pair these with a **clean** implicit-result body (`n + 1` → `no errors`) — a pass
that over-reports would look identical otherwise. Because the pre-fix binary printed `no errors`
for all three, the contrast binary from the parent commit is what makes this evidence conclusive.

## pysysml service ownership and the require-service gate (PR #204)

Ownership is the claim worth testing by hand, because pytest can pass while the invariant is
broken. Three probes, each with a state dir of its own so nothing collides:

```bash
PY=~/pysysml-venv/bin/python          # ls -d /home/ubuntu/*venv* if it is missing
cp bin/sysml-grpc ~/.pysysml/bin/     # what CI does; otherwise ensure_binary downloads
PORT=$($PY -c 'import socket;s=socket.socket();s.bind(("localhost",0));print(s.getsockname()[1])')
```

- **Foreign service:** start `bin/sysml-grpc -port $PORT` from the shell, then in one
  `PYSYSML_STATE_DIR=/tmp/stateN` python process `Connection(port=P, auto_start=False)`,
  `pysysml.connect(port=P)` (the adopt path) and a connection left open at exit. Expect the
  shell's pid to still be alive and the state dir to hold **only** `sysml-grpc-<port>.lock` — a
  `sysml-grpc-<port>.pid` for a service pysysml did not spawn is the bug.
- **Own service:** `pysysml.connect(port=<free>)` writes
  `{"pid","create_time","port","owner_pid","owner_create_time"}`; assert `create_time` equals
  `psutil.Process(pid).create_time()` and `owner_pid == os.getpid()`. Two connections must keep it
  alive when the first closes, and the last close (or plain interpreter exit, via `atexit`) must
  stop it and delete the `.pid`.
- **Pid spoof:** `bash -c 'exec -a "sysml-grpc -port P (decoy)" sleep 600'` plus a hand-written
  pidfile naming that pid with a `create_time` off by a few hundred seconds. The decoy must survive
  and the record must be replaced/removed. The old cmdline-substring scheme killed it, so this is
  the one probe that distinguishes the schemes.
- Use `conn.load(path)` (not `load_model`) for a real RPC that proves the connection talked to the
  service rather than failing early.

Suite counts as PR #204 merged (`cd python && $PY -m pytest tests/ -q`): **413 passed / 13 skipped**
with no service; **423 passed / 3 skipped** with a service on 50051 and
`PYSYSML_REQUIRE_SERVICE=1`, the
3 remaining skips being mypy-not-installed and a manual-binary-cache case, never a service skip.
With `PYSYSML_REQUIRE_SERVICE=1` and no service, collection must **error** (exit 2,
"none answers on localhost:50051"), never skip. A whole run must leave an operator-started service
on 50051 with the same pid.

## Orthogonal regions and cross-region transitions (`internal/core/runtime/state_region_transition.go`)

A transition whose source and target sit in different orthogonal regions of the same composite
state must exit only its source side, not the enclosing composite. The REPL surfaces needed to
see that at all:

- **`%trace on` is the only surface that shows exit/entry order.** `%current` shows the resulting
  configuration; only the trace distinguishes "exited the source" from "exited and re-entered the
  whole composite". Turn it on *before* the `%advance` that fires the transition, and assert on the
  absence of lines too (`no exit: <composite>`, no second `enter: <composite>`).
- **`%current` prints one active state per active region**, joined by ` | `, and each name is the
  state the region itself **declares** — a composite (`wrapper`), not the nested target inside it.
  So `wrapper | rtarget` is the correct shape for a target nested one level deep, and a **repeated
  name (`rtarget | rtarget`) is a bug signal**, not a rendering quirk: it means an outer region
  recorded a state it does not declare. Do not dismiss duplicates as legitimate.
- An emptied source region simply disappears from the list, so "the source region has no active
  state" is asserted by its absence.
- **Counter attributes, not state names, are the discriminator.** Give every state on the path an
  `entry`/`exit` action incrementing its own counter (`wrapperExits`, `runningEntries`, …) and read
  them from `%current`'s "State data". Exactly-once claims (`exit ran once`, `composite entered
  once`) are unprovable from the configuration alone, and double-exit bugs are invisible without
  them.
- **Always build a contrast binary and run the same model through both** (recipe earlier in this
  file). Cross-region models are easy to write so that they pass on the broken build too; the
  contrast run is what proves the model is adversarial. **Pick the contrast commit by where the
  defect lives:** the parent commit for a regression introduced by the PR, but `origin/main` when the
  bug is pre-existing — a main-based contrast additionally proves the PR fixes a shipped defect
  rather than one of its own making. Useful discriminating values seen historically: a pre-fix build
  re-enters the composite forever and dies at the event budget; a build with the wrong recorded
  active state prints a duplicated name; a build missing the widened concurrency test leaves the
  target region's old state running (`<state>Exits = 0`) and prints it alongside the target
  (`lstate | mid`); a build with the old `leaveRegion` clearing rule runs the same exit actions twice
  (`midExits = 2`, `wrapperExits = 2`).

Shapes worth covering, in increasing order of subtlety — each has broken independently:

1. plain sibling-region target (`state_transition_cross_region.sysml`);
2. a third sibling region that must keep its state (`..._third_region.sysml`);
3. target nested one and two levels under a composite the target region is **already running**
   (`..._nested_target.sysml`) — the abandoned inner state must exit;
4. target nested under a composite that is **not** active there (`..._inactive_wrapper.sysml`);
5. target region running a **different** composite, whose nested regions must be torn down;
6. **source nested deeper than the target's region** (`..._deep_source.sysml`) — the source side
   must be exited up to the shared region-set level (`exit: ideep` → `exit: wrapper`) while the
   composite is untouched;
7. outward exit past the composite, and a nested non-orthogonal move that must still stop at the
   LCA (regressions for the `leaveRegion` / `getLCA` paths);
8. **a region whose owning state is a plain substate** — the single highest-value shape, because it
   broke the dispatch walk and the exit walk independently, in consecutive commits. Write it as
   `region right { initial rs; state wrapper { state mid { region inner { … state ideep … } } }
   then rs mid; }`: `mid` is declared directly in `wrapper`'s body, so it is **not** in
   `graph.RegionOf`, and `then rs mid;` is what makes the inner region active. Any outward walk
   written as `RegionOf[RegionOwner[…]]` returns nil here and silently takes a wrong branch, and any
   "clear the region only when its entry is exactly this state" rule misses that `regionStates[right]`
   is `mid`, a *descendant* of the `wrapper` being exited. Cover both directions from `ideep`: a
   cross-region target (sibling region keeps a stale state → `lstate | mid`, `lidleExits = 0`) and a
   target **outside** the whole composite (`mid`/`wrapper` exit actions run twice). The shipped
   fixtures are `..._cross_region_substate_owner.sysml` and
   `state_transition_leave_composite_substate_region.sysml`;
9. ping-pong: two cross-region transitions targeting each other must stop with the typed
   `Stopped at the event budget (N events; raise SYSML_MAX_EVENTS to allow more)`, never hang or
   panic. Run the REPL as `SYSML_MAX_EVENTS=2000 ./bin/sysml` so the stop takes a second, and
   follow it with `%eval 1+1` → `= 2` to show the session survived.

**History interacts with region bookkeeping — always test it, with a control model.** A `deep
history resume;` vertex inside a composite with orthogonal regions is the natural victim of any
change to how a region records its active state: at ca7f3a89 the restore entered the region's
*initial pseudostate* instead of the recorded composite, so the inner region restarted and the deep
state was lost. To attribute such a failure correctly, write a control model with **no cross-region
transition at all** (region reaches a nested state, machine leaves the composite, history resumes)
and run it on both binaries — that separates "the cross-region path broke history" from "history is
broken for any composite-in-a-region". Shallow history legitimately restarts the inner region, so
only the deep variant discriminates. Read the restore in `%trace on`: `enter: <region initial
pseudostate>` in place of `enter: <recorded state>` is the signature. Cover the **plain
non-orthogonal** nesting too (one region, `first -> second`, `second::inner` running `a -> b`): it
broke separately from the orthogonal case and its signature is an *inconsistent* `%current` such as
`first | b`, where the outer region records a state whose inner region holds a descendant of a
different one. Any change to `leaveRegion`/`exitState` bookkeeping can move history even when it
looks like a pure exit-ordering fix, so re-run `restarts = 1` (shipped
`state_deep_history_region_composite.sysml`) plus a control model whose restored configuration is
visible (`lidle | wrapper | deepState`) as part of every such pass.

**Write history/nesting models with an explicit `region <name> { … }` wrapper.** Substates declared
directly in a state body (`state outer { state first; state second; }`) did not start in a history
model — the machine stalled at `outer` and no transition fired — so a model written that way looks
like a runtime failure when it is really an authoring shape the engine does not drive. Declare
`state outer { region only { initial os; state first; … then os first; } }` instead. Note the
exception exercised deliberately by shape 8 above: a substate *owning* a region
(`state mid { region inner { … } }`) does run, provided the enclosing region's initial transition
names it (`then rs mid;`).

### Engine limitations that constrain how these models can be written

All reproduce on several commits (not caused by any one PR), but they silently make a test vacuous:

- **A transition whose source is a composite state never fires.** Put the "later trigger" that
  leaves a composite on a nested **leaf** state instead; it exercises the same `leaveRegion` path.
- **A timer declared directly on a composite state entered by a cross-region move is never
  scheduled.** Use `%events` to confirm what is actually queued before trusting a `%advance`; leaf
  timers are scheduled correctly.
- **A guard-only transition is not re-evaluated for regions other than the one where the event
  occurred**, so "a timer in region C flips a flag that fires a guarded transition in region A"
  never happens. Drive each region with its own timed transition.
- Stale-reaction probes are still the best evidence that an abandoned state is really gone: give it
  `accept after N then boom` where `boom` sets a counter, advance past N, and assert the counter is
  still 0 — a state left running in a region reacts and flips it.

Practical notes for a recorded pass: shipped conformance fixtures under
`internal/core/runtime/testdata/conformance/` emit harmless
`unresolved reference: Integer — did you mean ScalarValues::Integer?` diagnostics (they omit the
`ScalarValues::*` import); the runtime still executes and the `.expected.json` next to each fixture
is the cheapest source of expected counter values. `/tmp` is wiped between sessions, so adversarial
models and contrast binaries must be recreated each run — and when you recreate a model, **re-derive
its state-def name from the file** (`grep -n 'state def' /tmp/…/model.sysml`) instead of trusting a
name written in a previous run's plan: a rebuilt model can end up with a different name, and
`%state Adv::WrongName` just reports `unresolved reference`, which is easy to misread as a runtime
bug mid-recording. Typing `clear` at the `sysml>` prompt is parsed as SysML and errors — `%quit`
first, then clear at the shell (this also rules out `clear; %load …` as a one-liner).

`python/scripts/pin_release_checksums.py --check` hits the GitHub releases API for every pinned
asset and dies with `HTTP Error 403: rate limit exceeded` once the unauthenticated budget is spent;
it reads `$GITHUB_TOKEN`. Set it without putting the token on camera:
`read -rs GITHUB_TOKEN; export GITHUB_TOKEN`. Careful with `--version <tag> --write`: it edits
`python/pysysml/binary.py`, so `git checkout python/pysysml/binary.py` afterwards. For the
"release publishes no assets" refusal use an old tag (`v0.0.4`) — v0.0.5..v0.0.8 all publish
binaries now. The unpinned-download refusal is testable offline-ish with
`HOME=/tmp/fakehome $PY -c "...ensure_binary(version='v9.9.9')"`, which keeps the real
`~/.pysysml/bin` cache intact; the opt-in out of it is per repository
(`PYSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>`, or `=1` for any).

## Quantities on the wire: `Value.quantity` and the Python `Quantity` (PR #200)

Once the service can marshal `runtime.ValQuantity`, a quantity slot no longer reads as
`SlotError: slot 'm': unsupported: quantity value` but as `pysysml.values.Quantity`. The
highest-value evidence is the **parent-commit contrast** (build `/tmp/old-sysml-grpc` from the
commit before the change, swap it into `~/.pysysml/bin/sysml-grpc`, clear
`~/.pysysml/sysml-grpc.{pid,refcount}`, run the same script): the old service raises the SlotError
while a plain `ScalarValues::Real` slot still reads `2.0`, so the frame proves the delta.

What to assert on the decoded value, and why each one distinguishes working from broken:

- **The magnitude must stay in the unit written.** `5.4 [SI::km/SI::h]` must decode as magnitude
  `5.4` with `unit.text == "SI::km/SI::h"` and `scale_num/scale_den == 5.0/18.0`; a reduction done
  behind the caller's back shows up as `1.5`. `in_unit(Unit(text="m/s", factors=(m^1, s^-1)))`
  must be **exactly** `1.5` — a float-sloppy conversion gives `1.4999999999999998`.
- **Integer and Real stay apart:** `3 [SI::m]` → `isinstance(magnitude, int)`.
- Base units are reported by **FQN of the base unit**, not the written unit: `SI::kg` reduces to
  `1000/1·SI::gram`, `SI::W` to `1000·SI::gram·SI::metre^2·SI::second^-3`. Assert
  `unit.exponents()`, which is order-independent.
- A **dimensionless** ratio (`3.0 [SI::m] / 1.5 [SI::m]`) does not arrive as a quantity at all — the
  runtime reduces it to a plain `float`. Don't write a test that requires a `Quantity` there.
- An exponent survives: `(2.0 [SI::m])**2` → magnitude `4.0`, `unit.text == "(SI::m)**2"`,
  `SI::metre` exponent `2.0`.
- Quantities also come back from `conn.eval("5.4 [SI::km/SI::h]", hash, context_symbol_id=pkg)`,
  from a nested child (`inst.engine.power`), inside sequences (`LengthValue[3]` → `list[Quantity]`),
  and on a `verify_constraint` verdict — filter `verdict.instances` on `verdict.instance_id` and
  read the slot off that instance.
- Comparison semantics worth pinning: `1 [SI::km] == 1000.0 [SI::m]` is True and the two hash
  alike; ordering/adding incommensurable units raises `IncommensurableUnitsError` naming both
  reductions (`cannot order a quantity in [SI::km] and one in [SI::kg]: … 1000·SI::metre …
  1000·SI::gram`); ordering against a bare `float` raises `TypeError`, while `q == 5.0` is just
  `False`.

**Inbound (client → service) is a different code path.** `Connection._python_to_value` has no
`Quantity` arm, so a quantity argument cannot be sent through `conn.calc(...)` — build the request
proto by hand and send it on the client's stub (still the real service over gRPC):

```python
q = sysml_pb2.Quantity(real_magnitude=2.0, unit="SI::m",
    unit_term=sysml_pb2.UnitTerm(scale_num=1.0, scale_den=1.0,
        factors=[sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0)]))
c._stub.EvaluateCalc(sysml_pb2.EvaluateCalcRequest(model_hash=m.hash,
    symbol_id="q2::Double", arguments=[sysml_pb2.Value(quantity=q)]))
```

Only the paths that decode with the model's `symbols.Index` work. `EvaluateCalc` does, and returns
typed errors worth asserting verbatim (`calc argument could not be read: unknown base unit:
SI::nope` / `unit scale is not a usable ratio: 1/0` / `quantity in "SI::m" carries no magnitude`,
all with `FAILURE_REASON_EVALUATION`, and the service stays alive). `ExecuteAction` (in
`internal/grpc/service.go`) decodes its `inputs` map the same index-aware way, so a quantity input
binds (`ExecuteActionRequest(inputs={"mass": Value(quantity=q)})` on an action whose `in mass :
ISQ::MassValue` body does `assign heavier := mass + 1.0 [SI::kg]` returns `6 [SI::kg]`), and a
malformed one comes back as `ExecuteActionResponse.Error` = `input "<name>" could not be read:
<err>` with **zero** outputs rather than a bound `ValInvalid`. If you ever see outputs coming back
`null: "unsupported"` for a quantity input, that call site has regressed to a context-free decoder.
Worth checking cheaply: a commensurable-but-different unit (`2000 [SI::g]`), an un-normalized
reduction (`SI::gram^2 · SI::gram^-1`, which the service re-normalizes), and a bad quantity nested
inside a `ValueSequence` input (the error still names the top-level input). Traps: the field is
`action_symbol_id`, not `symbol_id`, and an action **usage** (`action showIt : Show;`) reports
`no initial node found`, so execute the **def** that carries the body.

### Generated typed classes and mypy

`pysysml-generate <model> -o out.py` (or `python -m pysysml.generate`) types a quantity property
`-> _t.Quantity` with `_t.slot(self, "x", _t.as_quantity)`. Only slots with a **declared** quantity
type get it: an untyped derived attribute (`attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];`)
has no type facts and still generates `-> object` / `_t.as_object`, even though the runtime value is
a `Quantity`. Don't read that as a bug in the quantity typing.

To make mypy actually enforce it, **set `MYPYPATH` to the repo's `python/` directory** — without it
mypy cannot resolve the editable-installed `pysysml`, silently treats `_t.Quantity` as `Any` and
reports *no* errors on obvious misuse (a false pass that looks like a passing test):

```bash
cd /tmp/qw && MYPYPATH=/home/ubuntu/repos/OpenSysML/python \
  $HOME/pv/bin/python -m mypy --no-incremental --no-error-summary --follow-imports=silent misuse.py
# -> Unsupported operand types for + ("Quantity" and "float")  [operator]
# -> Incompatible types in assignment (expression has type "Quantity", variable has type "float")
```

`mypy` must be present in the same venv as `pysysml` ($HOME/pv here); otherwise the typed-codegen
tests skip rather than fail.

## The evaluate/eval split and generated-base planning (PR #218)

- **Since the rename, `pysysml.evaluate` is the real module-level evaluator and `pysysml.eval` is a
  forwarder that emits `DeprecationWarning` and is out of `pysysml.__all__`.** Test both sides:
  `evaluate(...)` must produce **zero** DeprecationWarnings (catch them with
  `warnings.catch_warnings(record=True)` + `simplefilter("always")`), `eval(...)` must return the
  identical value and warn exactly once with a message naming `pysysml.evaluate`, and
  `from pysysml import *` must bind `evaluate` but not `eval`. `subject` is the **last** parameter of
  `evaluate` precisely so a pre-rename positional call
  `eval(expr, None, hash, None, host, port)` still binds host/port — prove that argument really is the
  host by also calling it with a bogus address (`"203.0.113.9", 59999`) and requiring a
  `ConnectionError`; that call takes ~30-60s to time out, so give the runner a generous timeout.
- **`pysysml.errors.RuntimeError` is a warn-on-access alias of `ExecutionError`** served by the module
  `__getattr__` and absent from `errors.__all__`. Check it by *catching a real failure* with it
  (`except pysysml.errors.RuntimeError` around a cyclic-slot eval), not just by identity.
- **`generate.py` elides a base that another declared base already specializes, silently and by design**
  (`_without_implied`): `part def Backwards :> Vehicle, Hybrid` where `Hybrid :> Vehicle` emits
  `class Backwards(Hybrid):` with **no** comment, because `Vehicle` is still in the MRO. Only a base the
  class genuinely does not inherit gets `# … left out: Python cannot linearize it with the bases above`.
  To exercise the comment path you need a real C3 conflict, e.g. `X :> A, B`, `Y :> B, A`, `M :> X, Y`
  → `class M(X):` plus the `left out` note. Asserting the comment on a merely redundant base is a
  false negative.

## State-machine transition endpoint checking (PR #225 class)

The check that a transition names a vertex of *its own* machine (and that a routing pseudostate is
left by something) is a **name-resolution-tier pass**, so it is fully observable from
`./bin/sysml -validate <f>` — no execution needed. Codes: `endpoint-not-of-machine`,
`no-outgoing-transition`; messages are
`transition endpoint <as written> names a <state|pseudostate|start marker|end marker|element> that
is not a vertex of this state machine` and
`<kind> <name> has no outgoing transition, so a transition reaching it terminates nowhere`.

Because the pre-fix binary reports **nothing** for these models, the parent-commit contrast binary
(see the worktree recipe above) is the strongest single frame: old = `✓ no errors` exit 0,
new = error + `did not analyse cleanly` exit 2 on identical input.

Fixture shapes that actually discriminate (all one-liners; `state def` and `state` usage both work):

- Illegal, target of another machine: two `state def`s in one package, `transition busy to
  Other::running;` → reported at the endpoint's column.
- Illegal, marker as target: `first begin then off;` plus `transition on to begin;` → the message
  says **"start marker"**, which is how you tell `VertexKind` is being consulted.
- Illegal, dangling routing pseudostate: `junction route;` + `transition busy to route;` and no
  transition out → reported at the `junction` declaration, not at the transition.

False-positive traps to always include as *legal* rows, since each exercises a different branch:

- A transition into a **sibling orthogonal region** (`transition lidle to rtarget;` across
  `region left` / `region right`) — legal per UML §14.2.3.9.
  `internal/core/runtime/testdata/conformance/state_transition_sibling_region.sysml` is the shipped
  one; run it with `%state TransitionSiblingRegion` + `%advance 1` → `Current state: lidle | rtarget`
  and `crossed = 1`.
- `entry point into;` / `exit point outOf;` as endpoints (`state_entry_exit_points.sysml`).
- A sourceless `accept after 5 then <s>;` written *inside* a state (source is implicit).
- `first start then off;` with no `initial`.
- A junction left by a **succession** (`route then finishedUp;`) rather than a `transition`, and a
  `fork`/`join` reached by one — the pass tracks succession sources *by name*, so a regression here
  shows up as a bogus `no-outgoing-transition`.
- Two states with the **same name** in sibling regions (both `idle`) — must stay clean.

Cheap whole-repo false-positive sweep (~40 s, 83 models at 11d0ed72, expect `differing: 0`):

```bash
for f in $(grep -rl 'state def\|state .*{' --include=*.sysml examples \
             internal/core/runtime/testdata/conformance testdata | sort); do
  a=$(./bin/sysml -validate "$f" 2>&1; echo $?); b=$(/tmp/old-sysml -validate "$f" 2>&1; echo $?)
  [ "$a" != "$b" ] && echo "DIFFERS: $f"
done
```

Error-timing contract worth re-checking whenever this area moves: a structurally empty
`state def Empty { }` must `-validate` **clean** (exit 0) and only fail at
`-state Empty` with `failed to create executor: initialize state machine: no initial state found in
state machine Empty`. A check-time complaint there would be the regression.

Two message hygiene probes: grep the diagnostics for `ast\.|StateNode|%!` (must be zero hits — the
messages come from `lower.NotAVertexFormat`/`VertexKind`, not `%T`), and endpoints written as deep
qualified names (`Outer::Inner::Deep::busy`) must be echoed **in full** in the message.

Also note `done` is an inherited feature of `States::StateAction`, so naming a state `done` in your
own fixture draws `name conflict: done is already the name of the inherited feature …` plus a
reserved-keyword warning — pick `finished`/`finishedUp` instead, or the legal row fails for an
unrelated reason.

### Proving a succession or a pseudostate edge really fires (not just validates)

`-validate` cannot tell you whether lowering silently *dropped* an edge — a dropped succession looks
identical to a clean model. Make the edge observable instead: give the destination state an
`entry { flag = 1; }` and read both the active state and the attribute back:

```bash
printf '%%state <Def>\n%%advance 5\n%%current\n%%quit\n' | timeout 60 ./bin/sysml <model>.sysml
```

`%current` prints `Current state: …` plus a `State data:` block with the attribute values, so
"reached the state" and "ran its entry action" are separate evidence. Compare against a binary built
from the *previous revision of the same branch* (not the PR base) to attribute the delta to one
change. Useful discriminators in this area:

- `x then <junction>; <junction> then <s>;` — an implementation that resolves succession endpoints by
  simple *state* name drops the pseudostate hop entirely and the machine just stalls one state early.
- Same-named `junction route;` in two sibling orthogonal regions, each routing to its own region's
  end state with its own flag — if pseudostates are keyed by simple name, one overwrites the other
  and *neither* flag gets set. Run it ~5 times; the ordering must be identical every run.
- A regression row worth keeping: a succession to a **qualified nested** target
  (`gate then P::Def::outer::r::deep;`) usually works on both old and new revisions, so it proves the
  rewrite lost nothing rather than proving anything new.

Two traps when writing these fixtures:

- An unqualified target in another region resolves by ordinary name resolution first, so
  `lidle then rtarget;` fails with `unresolved reference: rtarget — did you mean A::B::right::rtarget?`
  on *every* revision. That is not a false positive; write the qualified name.
- A succession carries **no guard**, so a cross-region succession re-enters the composite state, the
  source region restarts, and the edge re-fires forever: the run ends on `Stopped at the event budget
  (1000000 events; raise SYSML_MAX_EVENTS to allow more)`. That is a clean bounded stop, not a hang —
  the state/attribute evidence is still valid. Use the guarded `transition a to b if flag == 0;` form
  when you want the run to settle instead.
- Running a shipped conformance model straight from the CLI can report `unresolved reference: Integer`
  because the conformance harness supplies imports the file omits; add `import ScalarValues::*;` in
  your own fixtures, and treat that one as pre-existing (the sweep confirms it is identical on both
  binaries).

## Hierarchical / composite state machines: outward transitions, timers, history (PR #230)

Whole families of state-executor changes (transitions declared on a composite state while a
substate is active, exit-chain ordering, timer withdrawal, change triggers) are testable from the
REPL with **no ports and no signals** if you encode observations as *decimal digits* in one
attribute. This is the single most useful technique in this area:

```
attribute log : Integer = 0;
state Outer { entry assign log := log * 10 + 1; exit assign log := log * 10 + 2; }
```

`%current` then prints e.g. `log = 32419`, which reads left-to-right as the exact order of the
entries/exits/effects that ran. Give **every nesting level its own digit** — a duplicated exit
(`…22…`) or a missing one is otherwise invisible, and a doubled exit is exactly the defect class
that shows up when exit chains are refactored. Keep the digits distinct per level and put the
transition effect last.

Driving and reading:

- Put fixtures under `$HOME` (e.g. `~/fixtures/pr<N>/`), never `/tmp` — `/tmp` is wiped between
  sessions and you will lose an afternoon of fixtures. `%load` needs an **absolute** path.
- A nested state reference must be **qualified**: `start then Working::Step1`, not `then Step1`.
- A nested `initial` only takes effect inside an explicit `region { … }`. `state Outer { initial o;
  state Mid { … } transition o to Mid; }` enters **Outer only** — `ActiveStates()`/`%current` show
  no substate at all, and any test built on that shape silently probes the wrong thing. Wrap both
  levels in regions and assert the pre-condition configuration before the interesting step.
- `ActiveStates()` for a region-wrapped chain lists the *innermost* composites and leaves
  (`map[Mid:true leaf:true]`), not every enclosing composite — assert on the leaf plus the state
  you expect to survive, not on the outermost name.
- In Konsole use `%quit`; `Ctrl-D` closes the terminal window and kills the recording. `ctrl+plus`
  (not `ctrl+shift+plus`) changes the font size.
- Batch a whole scenario into a `.script` file and pipe it (`timeout 30 ./bin/sysml < z.script`) to
  discover values, then re-run the interesting ones interactively for the camera.
- A per-fixture expectation table plus a tiny runner that prints `fixture | final state | log |
  PASS/FAIL` turns a 30-fixture regression matrix into one screen of camera-friendly evidence, and
  it fails loudly if a refactor moves any established value.

Timers (`accept after N` + `%advance`) are the cheapest scheduling probe:

- Advance to the **exact** expected instant and read `Remaining events:` (`1` = armed/re-armed,
  `0` = spent) plus `No pending work - simulation time is now …` — that line is the clearest
  signal that a timer was never re-armed.
- To prove a queued time event is *withdrawn* when its state exits, the state must be left **and
  re-entered**: `accept after 10` left at t=3 and re-entered at t=5 must fire at t=15, not t=10.
  Add a second region with its own clock (`after 12`) so the ordering of two digits (`21` vs `12`)
  is the discriminator instead of one absolute time.
- Bound repeated exit/re-entry with a counter guard (`if trips == 0 do assign trips := trips + 1`)
  or the fixture becomes an infinite loop.
- **Pre-existing on every binary tested (not a PR regression):** a state that owns a
  time-triggered transition ignores its own *signal* transitions, and signal transitions on leaves
  inside top-level regions never fire. So drive timer fixtures purely with `accept after`, and
  verify any such oddity against a contrast binary before reporting it.
- `deep history resume;` works in REPL fixtures (`transition away to resume`) and doubles as an
  exit-chain probe, since the return path re-runs the entry digits.
- Once composite self-transitions re-enter their regions, a fixture whose signal poster sits
  *inside* the self-transitioning composite becomes an infinite event loop; post from a sibling
  region that is never re-entered.

Change triggers (`accept when`) **cannot be driven from the REPL at all**: `pollChangeEvents` has
no non-test caller (`grep -rn pollChangeEvents --include='*.go' .` → definition plus `*_test.go`),
and no meta-command reaches it (`%advance`/`%step`/`%continue` are time and the *action* debugger).
The REPL therefore parks in the source state with the condition already true — that is the
documented ⚠️ Approximate row in `docs/project/spec-compliance.md`, not a defect. Show the grep on
camera to justify covering the behavior with go tests instead:

```go
exec := stateExecutorForSource(t, "sm", `package test { state sm { … } }`)
exec.RunToCompletion()
exec.stateData["ready"] = boolValue(true)
exec.pollChangeEvents()
// read exec.ActiveStates() and exec.stateData["log"]
```

For an internal fix like this the strongest evidence is a worktree A/B:
`git worktree add ~/wt<sha> <parent-sha>`, **copy the new test file (and any probe file) into the
worktree** so `state_executor.go` is the only delta (`cmp -s` each file on camera to prove it),
then run `go test -count=1 -run … -v ./internal/core/runtime` in both trees.

Designing discriminating probes here is genuinely tricky — always confirm a probe **FAILs on the
parent** before trusting it:

- `getParentChain` is **leaf-first**, so a single active leaf already gets innermost-first
  treatment; a probe with one leaf plus enclosing composites will pass on both revisions and prove
  nothing.
- The discriminating shape for innermost-wins/dedupe in the poll path needs a **sibling region
  whose leaf watches nothing**, so its outward walk reaches the composite's own watched transition
  while another region's leaf has its own: only a correct implementation keeps the composite alive.
- Keep probe files out of the repo tree before running the gates (`rm` it, keep a `.txt` copy in
  the fixtures dir) and confirm `git status --porcelain` is empty on camera, so the gates clearly
  reflect the committed tree.

## Guarded successions in action flows (PR #266 and anything touching `enabledSuccessions`)

Guards on successions out of *ordinary* nodes (`first s1 if c then s2;`, `then s1 s2 if c;`) are
evaluated in `ActionExecutor.enabledSuccessions`; a failing guard prunes the edge and the flow just
ends there (`Final state: Completed`, not an error). Test it end to end as
`%load <file>` → `%action <pkg>::<action>` → `%continue` → read the `Results:` block.

Traps and recipes:

- **The result values are the only evidence.** A guarded flow that skips its successor still prints
  `✓ Action completed / Final state: Completed`, identical to one that ran it — only the attribute
  values differ. Always design the skipped branch to write a distinctive attribute (`y := 9`) that
  stays at its initial value when the guard prunes the edge.
- **Guards are evaluated when the token LEAVES the node**, so they see what that node wrote: with
  `x = 3`, `action s1 assign x := x + 4;` and `first s1 if x > 5 then s2;` the branch is taken
  (`x = 7`, `y = 9`). A "reads the pre-write value" regression shows up as `y = 0`.
- **A/B against a binary from the parent commit is cheap and decisive here**, because a pre-fix
  build silently ignores the guard: same file, `y = 9` / `ignited = 1` instead of `0`. Build it with
  the worktree recipe above and run both in the recording.
- **Restart the REPL between fixtures.** All the guard fixtures declare `package test`, so a second
  `%load` in the same session makes `%action test::seq` ambiguous/unresolvable
  (`unresolved reference: test::seq — did you mean SequenceFunctions::union::seq1 …`). Ctrl-D and
  relaunch per model; do not chain loads.
- Write fixtures with `import ScalarValues::*;`, otherwise every `attribute x : Integer` emits
  `unresolved reference: Integer` noise on camera (execution still works).
- **The REPL still executes a model with parse errors.** `first s1 if c;` (guard with no `then`) is
  rejected at parse time with `expected 'then' after guard condition`, yet the rest of the model
  loads and runs — read the diagnostics, don't just read the `Results:`.
- Error wordings to assert: non-Boolean guard →
  `error: execution failed: type mismatch: node s1: guard must evaluate to boolean, got constant`;
  unresolvable name in a guard → `error: execution failed: eval guard of node s1: unresolved
  reference: nosuch`. Two guards out of one action node that both hold →
  `error: execution failed: more than one succession is enabled: action node check has multiple
  successors` (wraps the `ErrAmbiguousSuccession` sentinel; the run stops with the token still on the
  node and neither branch's attribute written — assert that with `%tokens`, not just the message).
- Places a guard could still be silently ignored, all worth a one-liner fixture: succession out of a
  real initial node (`then start s1 if …;`), out of a `merge`, out of a `join` (must not deadlock —
  the branch tokens are consumed either way), and one whose target is `done`.
- **A guard inside a nested action body is not observable**: an action declared inside an action body
  that itself declares `first … then …` is never executed (pre-fix binaries behave identically), so
  the enclosing flow's later nodes run while the inner writes never happen. Prove it is pre-existing
  with the A/B binary and report it as untested rather than a failure.

- **A guard on the succession out of a `merge` needs a token-ordering fixture to be tested at all.**
  First-token-wins must count the token that *traverses*, not the one that arrives, so the
  discriminating shape is a fork with one short branch (`a -> mg`) and one longer branch
  (`b1 -> b2 -> b3 -> mg`) where only the last node of the long branch writes the feature the guard
  reads. `%step` steps every live token per step, so a guard reading a value written by a *sibling*
  branch node in the same step is not discriminating — put at least one extra node after the write.
  Expected on a correct build: `%tokens` shows the short branch's token at `mg` with the guard still
  false, that token disappears (retired), then the long branch's token traverses `mg` and the node
  after it runs. A build that closes the merge on arrival instead discards the second token, never
  runs the node after the merge, and then spins to
  `error: execution failed: execution exceeded max steps (1000000 steps; ...)` — so "hangs to the
  step budget", not just a wrong value, is the pre-fix signature here.

## Verifying the RDF "experimental" marking, `%print`, `%view`, guards and addressed sends

The wave-1/0.1.0 surfaces below were verified end to end at `870da1fd`. Each entry is the
assertion that actually distinguishes working from broken.

### The single experimental notice (`internal/core/export/experimental.go`)

`export.ExperimentalNotice` is the one wording; `export.IsExperimental(from,to)` is true iff either
side is Turtle (notation→notation is never experimental). **Read the constant from source at the
start of a run instead of hardcoding it — the wording has already changed once** (220 chars on the
#261 branch, 239 chars on main; the current text names "the behavior its bodies state").

Assertions that hold, and how to word them:

```bash
./bin/sysml examples/rdf-interop-demo.sysml -convert ttl >out 2>err   # exit 0
grep -c '^note:' err            # 1 — the notice is stderr-only
grep -c experimental out        # 0 — the graph on stdout stays machine-readable
./bin/sysml m.sysml -convert sysml -o n.sysml 2>&1 | grep -c '^note:'  # 0 (notation→notation)
```

- Do **not** assert "stderr is empty" for notation runs: the `wrote … (sysml, N bytes)` line also
  goes to stderr. Assert *no `^note:` line*.
- `-o /dev/stdout` still keeps the graph clean; refusals print the notice *before* the error, exit 2,
  write nothing.
- REPL: `%save x.ttl` prints the notice then `saved N bytes of ttl`; `%save x.sysml` prints none, and
  neither written file contains a `note:` line.
- Cross-surface identity is one diff: compare the CLI note (minus the `note: ` prefix) with
  `pysysml.conversion.EXPERIMENTAL_NOTICE`, the gRPC `ConvertResponse.experimental_notice` and the
  docs string. All four must be byte-equal.
- pysysml: one `ExperimentalFeatureWarning` per RDF conversion, `Conversion.experimental` True with
  the notice; sysml→sysml gives `False`/`""`/0 warnings; a refused RDF conversion warns *then* raises
  `ConversionError`; `warnings.simplefilter("ignore", ExperimentalFeatureWarning)` silences it while
  the conversion still returns the graph.

### Corpus claim (`README.md`, `docs/reference/rdf-mapping.md`)

The claim is a measured number and has moved (71/120 → **102/120** after behavioral nodes were
mapped). Re-measure, never trust the doc:

```bash
ok=0; bad=0
while IFS= read -r f; do
  if ./bin/sysml "$f" -convert ttl -o /dev/null >/dev/null 2>&1; then ok=$((ok+1)); else bad=$((bad+1)); fi
done < <(find examples \( -name '*.sysml' -o -name '*.kerml' \))
echo "converted=$ok refused=$bad total=$((ok+bad))"     # 102 / 18 / 120 at 870da1fd
```

Use process substitution (not `basename`) — many training paths contain spaces. Every refusal must
name a construct **and** `file:line:col` (`succession`, `operator expr`, `prefix metadata`,
`` `snapshot` declaration``, `duplicate declaration of "X"`); a bare "cannot convert" or a silent
drop is the failure shape.

### Round-trip fidelity for behavior (`.sysml → .ttl → .sysml`)

A subaction's `do` and comment contents are the fragile parts. Fixture shape that works:

```
state off { entry do { perform Warm; } }
//* a note naming entry and do and exit */
state starting {
    // a line comment naming exit
    do action running : Warm;
    /* a block comment naming entry do */
    exit perform Warm;
}
```

`entry do { … }`, `do action running : Warm;` and `exit perform Warm;` must all come back, and the
kind keywords inside the three comment styles must introduce **no** spurious members. Note plain
comments themselves are not carried through RDF — that is expected, not a failure. `entry do action
heat : Warm;` does **not** parse (`expected an action, an action reference or '{' after 'entry'`).

### `%print`

Read-only printer of the session buffer or one element:

- `%print` → whole model with its comments; `%print Top::'My Pkg'::Car` → just that element (quoted
  segments work). Empty session → `nothing to print: the session is empty`; a bad name →
  `error: unresolved reference: …`. No RDF wording, no `note:` line.
- Round trip without a GUI: pipe `%load f\n%print\n%quit`, strip the banner/`goodbye` lines with
  `sed '1d;2d;$d'`, reload that output and print again — the two must be byte-identical.
- Regression to keep: `%print` during a live `%action`/`%state` debugger leaves it alone —
  `%instances` still reports none created and the next `%step`/`%continue` still works.

### `%view` (viewpoint conformance)

`%view Demo::report` prints `exposes` then `viewpoint conformance` with
`satisfy structure (from Demo::StructureView): violated` and one line per concern
(`conforms` / `violated (framed by the viewpoint but not by the view)` / `unevaluable` + reason).
A satisfy target that is a `requirementUsage` is diagnosed at load *and* reported as
`unevaluable (satisfy target spec is a requirementUsage, not a viewpoint)` — never a silent pass.
`%view` registers no object: `%instances` afterwards says none created, a repeat is identical, and a
following `%constraint`/`%satisfy` gives a typed message (`not a constraint: MassBudget is a concern
def …`) rather than an ambiguity error.

### Succession guards and addressed sends

- A guard on a succession leaving an *ordinary* action node is evaluated: fixture with `level = 4`
  and branches `if level > 10` / `else` must end with the false branch's counter at 0.
- Fork fixtures under `internal/core/runtime/testdata/conformance/` (e.g.
  `action_succession_guard_fork_branch_pruned.sysml`) run fine in the REPL but **emit unresolved
  `Integer` diagnostics** because they rely on the test harness's implicit `ScalarValues` import.
  Copy the fixture and add `import ScalarValues::*;` if you want a clean transcript.
- Addressed sends carry object identity: a send to `Ident::alpha::reader` must **not** be consumed by
  the sending object's own same-named `reader`. The proof of correctness is a *deadlock error*
  (`accept deadlock in action talk … accept n waiting since step 3`); a broad-delivery bug would
  complete instead. An unresolvable address gives the typed
  `send reaches no receiving port: "Unrouted.alpha.count" names no port of an object the sender can
  address`.
- For the performed/indirect case use `send_identity_performed_object.sysml` (a state entry sends to
  `worker`, a two-level `perform`ed action accepts it) and drive it with `%instantiate` + `%state` +
  `%advance 1` → the machine reaches `heard`. Hand-writing this is easy to get wrong: `send 7 to
  reader` from a package-level scope reports `unresolved reference: reader`, and a qualified
  package-level receiver is *unroutable* because it belongs to no object.

### Change/time triggers that are still absent (issue #268 at 870da1fd)

Confirm absence rather than assuming it, and keep the fixture shape right — the body must hang off
the **state usage**, not the `state def`, or you get
`initialize state machine: no initial state found`:

```
part def Machine {
    attribute ready : Boolean = true;
    state def Modes;
    state modes : Modes { entry; then idle; state idle; state done;
        transition idle_to_done first idle accept when ready then done; }
}
```

`%state …::modes`, `%advance 5`, `%current` → still `Current state: idle` (nothing in
`RunToCompletion` polls change events; documented as ⚠️ Approximate in
`docs/project/spec-compliance.md`). A unit-carrying time trigger (`accept after 5 [SI::s]`) fails
with the typed `initialize state machine: schedule events: time duration must be constant, got
quantity`, while the unitless `accept after 5` fires at t=5 — that pair is the cheapest way to show
the feature is absent-but-typed rather than half-present.

## Source-preserving edits: `ApplyEdits` / `model.edit()` (PR #282)

The edit surface is only reachable through pysysml (`model.edit()` → `set_value` / `rename` →
`apply()` → `save(path)`); there is no REPL meta-command for it, so the REPL is only useful
afterwards, to prove the edited file still parses/instantiates.

Setup that actually matters: the client auto-starts `~/.pysysml/bin/sysml-grpc`, so a stale copy
there serves an old build and `apply()` fails as `MissingCapabilityError('apply_edits')` — which
looks like a client bug. Always `go build -o bin/sysml-grpc ./cmd/sysml-grpc`, then
`pkill -x sysml-grpc` (the file is `Text file busy` while it runs) before
`cp bin/sysml-grpc ~/.pysysml/bin/`.

Fixture shape that discriminates a broken implementation: one file with line comments, a block
comment, blank lines and **deliberately mixed tab/space indentation** (some lines space-indented
inconsistently). A reformatting implementation normalizes those lines, so the assertion is
`diff -u orig edited` showing exactly **2** changed lines per operation (`diff | grep -c '^[<>]'`),
never "the file still validates". `cat -A` the fixture on camera to show the tabs are real.

Targets and evals worth using (they cover the interesting locate.go branches in one file):

| target | notes |
|---|---|
| `Demo::sc::unitMass` (`attribute redefines unitMass = …;`) | the `redefines` shorthand path |
| `Demo::SC::margin` (`attribute margin : ISQ::MassValue;`) | value **added**: `AppliedEdit.length == 0`, `new_text == ' = 5.0[SI::kg]'` (a space is inserted before `=`) |
| `Demo::SC::avionics::board::count` | deeply nested; eval it as `eval("avionics.board.count", subject="Demo::sc")` |
| `Demo::SC::total = unitMass * 2` | expression referencing another feature; evaluates against the *redefined* value (1200 → 2400) |

Refusals and the exact class each raises (all leave the file byte-identical — sha256 it before and
after every case, since "an exception was raised" says nothing about writes):

| case | class / `failure` |
|---|---|
| unknown FQN, **and any stdlib element** (`ISQ::MassValue` reports `no element named … in this model`) | `EditTargetError` / `EDIT_FAILURE_UNKNOWN_TARGET` |
| a `part def` target | `EditTargetError` / `EDIT_FAILURE_NOT_VALUED` |
| `"1050.0[SI::kg"`, `""`, rename to `part` or `2bad` | `InvalidEditError` (`INVALID_VALUE` / `INVALID_NAME`) |
| a value that parses but does not resolve (`Nope::missing`) | **`EditResultError` / `EDIT_FAILURE_RESULT_INVALID`** — not `InvalidEditError`; it is caught by re-analysis, and `diagnostics` names the *model* file and line |
| two `set_value`s on one feature | `OverlappingEditsError` |
| `apply()` twice | plain `builtins.RuntimeError` (client-side, **not** a `PySysMLError`) |
| `apply()` with no ops | `NoEditsError` |
| non-`str` value | `TypeError` |
| renaming a referenced declaration | `RenameReferencedError`, `referring_elements == ['Demo::SC', 'Demo::sc']` |

Two cases need process work rather than a Python call:

- **Evicted model → `ModelNotFoundError`.** Hand-start the service (`bin/sysml-grpc -port 50123
  -health-port 8123`) with `pysysml.connect(port=50123, auto_start=False)`, load, kill and restart
  it, reconnect, then rebuild the editor against the *new* connection with the *old* hash:
  `pysysml.edit.Editor(m._hash, c2)`. Killing the auto-started 50051 service mid-script instead
  tends to hang the run — a `connect()` right after a `pkill -x sysml-grpc` did not return.
- **`MissingCapabilityError` before the RPC.** Don't chase the v0.0.7 download; build the merge-base
  with `git worktree add /tmp/oldmain main` + `go build -o /tmp/old-sysml-grpc ./cmd/sysml-grpc`
  and run it on port 50099 (`-health-port 8099`; 8081 collides). Assert the old service still
  serves reads (`load` + `eval`) and that the raise is `MissingCapabilityError`, not
  `UnsupportedOperationError`/UNIMPLEMENTED — that class difference is the proof the check ran
  client-side. The service logs no per-RPC lines at INFO, so "no ApplyEdits in the log" proves
  nothing on its own.

`Model.find` returns **None** for a missing symbol (only `model[name]` raises), so the
"old name is gone after a rename" assertion must compare against `None`; a `try/except` around
`find` passes even when the rename did nothing.

### Traps that cost time when re-testing the edit surface

- **`python/tests/test_edit.py`'s `real_service` fixture prefers `<repo>/bin/sysml-grpc` over
  `~/.pysysml/bin/sysml-grpc`** (`GRPC_BINARIES`, test_edit.py:61). A stale `bin/sysml-grpc` left
  from an earlier snapshot therefore fails all 13 `TestEditRoundTripAgainstRealService` cases with
  `MissingCapabilityError('apply_edits')` / `assert has('apply_edits') == False`, which reads like a
  product bug. Always `make build-grpc` and check `./bin/sysml-grpc -version` prints the current
  commit *before* running the suite; the same applies to the copy in `~/.pysysml/bin`.
- `MissingCapabilityError` lives in **`pysysml.capabilities`**, not `pysysml.errors`
  (`pysysml.errors.__getattr__` raises `AttributeError` for it). It is still a `PySysMLError` and is
  *not* an `UnsupportedOperationError`.
- `test_generate_golden.py::test_typed_codegen_modules_are_mypy_clean` may fail for reasons unrelated
  to any change: with mypy 2.3.x and the venv's numpy stubs it reports
  `numpy/__init__.pyi:737: error: Type statement is only supported in Python 3.12 and greater`.
  Reproduce with `echo 'import numpy' > /tmp/nm.py; $HOME/pv/bin/python -m mypy --no-incremental
  /tmp/nm.py` before blaming a PR; pinning/refreshing numpy stubs is the likely fix.
- Non-ASCII **identifiers** (`package Démo`) do not parse, so a Unicode fixture must keep the
  accents/emoji in comments and strings only. That still exercises the byte-vs-rune offset risk:
  assert `saved == orig[:a.offset] + a.new_text.encode() + orig[a.offset + a.length:]`, which is the
  strongest single assertion available for `AppliedEdit` (it proves offsets are byte offsets and
  that nothing outside the span moved).
- CRLF: build the fixture as `orig.replace(b"\n", b"\r\n")` and assert the saved file has the same
  `\r` count, **zero bare LFs** (`b.count(b"\n") - b.count(b"\r\n") == 0`) and no `\r\r`. Test the
  *insertion* case (a feature with no value) on CRLF too, not just replacement.
- Unwritable targets: `save()` raises plain `PermissionError`/`FileNotFoundError` from `open()`, so
  sha256 the victim file before/after — the point of the case is that it is not truncated first.
- An edit is allowed on a file that already has errors elsewhere (name *or* syntax), and the saved
  file keeps exactly those pre-existing diagnostics; a refusal there is the regression the
  "baseline diagnostics" fixes in PR #282 were about.

## `%check` and the SMT solver driver (PR #285)

`%check <name>` asks an external solver about a constraint def, requirement def or satisfaction
assertion. z3 may already be installed (`/usr/bin/z3`, 4.8.12 seen); discovery is `OPENSYSML_SMT`
(path to an executable) → `z3` → `cvc5` on PATH, and `OPENSYSML_SMT_TIMEOUT` (default `10s`) bounds
one query. Every solver-availability path is reachable from the shell without touching code, so
drive each in its own REPL and show the env on camera:

```bash
OPENSYSML_SMT_TIMEOUT=1ms ./bin/sysml            # "? … is undecided (z3, 1ms)" + "Reason: the solver ran out of time after 1ms"
OPENSYSML_SMT=/tmp/fake_unknown.sh ./bin/sysml   # sh script: printf 'unknown\n(:reason-unknown "incomplete")\n'; then `while read -r line; do :; done`
OPENSYSML_SMT=/tmp/fake_fail.sh ./bin/sysml      # sh script: exit 3 → "error: the SMT solver did not answer: … failed at check-sat"
env -u OPENSYSML_SMT PATH=/tmp/emptybin ./bin/sysml   # "error: no SMT solver found: install z3 …"
```

The fake-unknown script *must* keep draining stdin after printing, or the driver's later writes hit
a closed pipe and you get a process error instead of `unknown`. Note `env -u … PATH=/tmp/emptybin`
also removes `timeout` from PATH — call `/usr/bin/timeout` by absolute path in piped dry runs.

Fixture shapes that actually discriminate:

- **Naming a satisfaction:** `%check` on a `requirement usage` reports the *requirement*, not the
  assertion. To reach a satisfaction, `%check` the element whose body holds `assert satisfy r by o;`
  (e.g. a `part context { assert satisfy fastEnough by car; }`) → `✓ Satisfaction satisfy fastEnough
  by car is satisfiable`. Bare `satisfy X by y;` at package level is rejected at parse/semantics
  ("satisfy target must be a requirement usage") — always go through a named requirement usage.
- **Truncating `/` and `%`:** use a *symbolic* dividend/divisor (`a == -7 and b == 2 and q == a / b`),
  because a constant `-7 / 2` is const-folded to the evaluator's real quotient (`%eval -7 / 2` = `-3.50`),
  so `q : Integer == -7 / 2` is reported **unsatisfiable** by design. The discriminating pair is
  `q == a / b and q == -3` (sat) vs `q == -4` (unsat, the floor answer); same for `r == a % b` with
  `-1` vs `1`. A literal zero divisor is refused with "operator `/` by zero not translatable".
- **Untranslatable:** `i ** 2 == 4` → `error: constraint X: operator `**` not translatable for
  solving: …` (no verdict). Incommensurable units are refused the same way (`L against M`).
- **Read-only:** start `%action <A>`, run `%check`, then `%step` — the token must still advance
  (`Token 1 @ start` → `Token 1 @ step1`).
- **Assignments use qualified OpenSysML names** (`Check::Satisfiable::i = 4`,
  `Check::SpeedReq::'craft.topSpeed' = 100`, enum as `Check::Gear::high`). A quantity is a magnitude
  in the *base units* a written unit reduces to, named as such (`1500.0 [kg]` comes back as
  `1500000.0 [gram]`), so don't expect the unit as written.

## `%explain <name>` — unsat cores (PR #291)

`%explain` asks the solver which conditions of an unsatisfiable constraint/requirement/satisfy
element conflict. It shares `solveQueries` with `%check` (`internal/repl/check.go:149`), so the two
must always reach the same verdict; the split is that `%check` prints a satisfying assignment and
`%explain` never does. Rendering lives in `internal/repl/explain.go`, core reduction in
`internal/core/solve/core.go`.

`internal/repl/testdata/explain_conflicts.sysml` is the fixture that covers every core-row shape at
once, so prefer it over hand-rolled models. Values observed at 04d0c4b with z3 4.8.12, loading that
file **alone** (locations are buffer-relative — see below):

| `%explain Conflicts::…` | header | rows |
|---|---|---|
| `Contradictory` | `✗ … 2 conditions conflict` | `required condition: \`i > 8\`` @10:29, `\`i < 3\`` @11:29 |
| `NatBound` | `✗ … 2 conditions conflict` | `declared domain: \`a Natural is not negative\` — declaration Conflicts::NatBound::n` @20:9, `required condition: \`n < 0\`` @21:29 |
| `ZeroDivisor` | `✗ … 2 conditions conflict` | `well-definedness: \`a / b == 1\`` @28:29 **then** `required condition: \`b == 0\`` @27:29 |
| `Derived` | `✗ Requirement … 2 conditions conflict` | `\`x > 10\` — requirement Derived, declared by Base` @34:30, `\`x < 1\` — requirement Derived` (no "declared by") @38:30 |
| `Satisfiable` | `✓ … is satisfiable, so no conditions conflict` | none, plus `Use %check … for a satisfying assignment.` |
| `rig::always` | `✗ Constraint always (negated) … 1 condition conflicts with itself` | `denied conditions: \`not (i == i)\`` @47:9 |

Every unsat header is followed by one minimality line before the numbered rows.

Things that look like bugs but are not, and traps:

- **Rows are in query-assertion order, not source order.** `ZeroDivisor` lists the hoisted
  divisor guard (line 28) before the condition that sets the divisor to zero (line 27). Don't
  report this as mis-ordering without checking `translate.go`'s assertion order first.
- **The location column always names `<repl>`, never the loaded file, and counts lines from the
  start of the joined buffer.** With one file loaded the numbers happen to match the file; load a
  second file first and the same condition moves (`<repl>:23:29` for what is really
  `explain_conflicts.sysml:10:29`), while load-time *analysis* diagnostics in the same session do
  name the real file and count from its start. The unit tests assert `<repl>`, so this is the
  REPL's implicit-document convention rather than a regression — but it is misleading, and any
  location assertion must therefore fix the load order. Always `%load` the fixture **alone**, or
  pass it as a CLI argument (`./bin/sysml <fixture>`), when asserting line/col.
- **A 1-member core has its own minimality wording**: `The condition below is the whole conflict:
  nothing else is needed for it.` (`internal/repl/explain.go` `minimality`), not the multi-condition
  `dropping any one leaves the rest satisfiable`. `rig::always` is the case that exercises it.
- **`String` IS in the translatable subset** (`SortString`, `internal/core/solve/reference.go:214`),
  so `constraint { s == "x" }` answers `satisfiable` and is useless as an "outside the subset" case.
  Untranslatable cases that do work: a **calc invocation** in a condition
  (`assert constraint { Twice(i) > 4 }` → `invocation not translatable for solving: it is outside
  the subset`) and an **unresolved reference** (→ `it resolves to nothing`).
- **z3 decides nonlinear reals**, so `x*y == 7.5 and x*x + y*y == 2.0` comes back `unsat` with a
  real core, not `unknown`. The `? … is undecided, so there is nothing to explain` branch is hard
  to reach on purpose; use `SYSTEMICA_SMT_CORE_BUDGET` (a tiny value) if you need to exercise the
  non-minimal `Note` wording instead.
- **Header durations include core-reduction time** (since 04d0c4b), so an unsat `%explain` reads
  ~3x the matching `%check` (24–30ms vs 7ms). Never assert exact milliseconds.
- **`%explain` is read-only.** An `%action Debug::tally` session (fixture
  `internal/repl/testdata/action_debug.sysml`) must survive it: `%step` after `%explain` still
  prints `✓ Step complete` and the run ends `total = 5`. A regression here shows up as
  `error: no active action session`.

The no-solver path is the one case needing a special launch: `mkdir -p /tmp/nosolver` and run
`env PATH=/tmp/nosolver ./bin/sysml`, which yields `error: no SMT solver found: install z3 … or set
SYSTEMICA_SMT …; looked for [z3 cvc5] on PATH`. Note `Discover()` is consulted per command, so this
must be set on the process, not toggled mid-session.
