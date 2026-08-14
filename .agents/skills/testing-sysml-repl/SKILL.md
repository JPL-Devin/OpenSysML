---
name: testing-sysml-repl
description: How to build, drive, and record end-to-end tests of the Systemica sysml REPL (bin/sysml) and the sysml-grpc service with its pysysml Python client — meta-command behavior, symbol lookup, action/state debugging, gRPC slot serialization, and GUI-terminal recording setup.
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

## Two ways to drive it

1. **Non-interactive (fast, for exploration and expected-value discovery).** The REPL reads a
   script from stdin fine:
   ```bash
   printf '%%load internal/repl/testdata/vehicle_package.sysml\n%%instantiate Vehicle\n%%slots Demo::Vehicle\n%%quit\n' | timeout 30 ./bin/sysml
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

Models that convert to Turtle are a narrow set: anything with state substates or a `calc` result
member still fails with `cannot convert the *ast.SubstateMember/…ResultMember at …`, so use a
plain `package Demo { part def Engine { attribute power = 300.0; } }` for `.ttl` assertions rather
than a file from `examples/`.

**Not one file in `examples/*.sysml` or `internal/repl/testdata/*.sysml` converts to `ttl`** — every
one fails on an unsupported construct (`*ast.SubstateMember`, `*ast.ResultMember`, `*ast.StateNode`,
`*ast.StateRegion`, `*ast.ConstraintMember`, `*ast.AssignmentActionNode`, `*ast.InitialNode`,
`*ast.SubjectMember`, or a `duplicate declaration of "result"`). So a Turtle test needs a
hand-written fixture; this one round-trips and yields exactly 1180 bytes of Turtle:

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
is the cheapest source of the values `%slots` should print.

- **Variant rendering.** A bound variation slot prints `name = variantName (Instance ID: n)` with the
  variant's nested values indented under it (`engine = electric (Instance ID: 2)` / `power = 150.00`).
  A plain `name = Instance(ID: n)` for a variation feature means nothing was bound.
- **Assert a computed attribute, not just the variant name.** `curbMass = 1200.00` (= `900 + mass`)
  distinguishes the `electric` variant (mass 300) from `petrol` (mass 200 → 1100); the variant label
  alone would look the same if the wrong nested values were materialized.
- **Always include a constraint that must be *violated*.** `variation_attribute_selection` asserts
  both `isIdeal` (satisfied) and `notShallow` (violated). An implementation where
  `x == x::variantName` returned true for any variant would still show `isIdeal: satisfied`, so the
  violated one is the only real discriminator. `%slots` renders these inline as
  `name: <constraint: satisfied|violated>` — note `%satisfy` answers
  `no satisfaction assertion in the session` for `assert constraint` members, so use `%slots`.
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
- **Unset scalar attributes render as `= Instance(ID: n)` / `(no features)`** (also `ringCost`,
  `ringPort`). Pre-existing on main, unrelated to variation work — don't report it as a regression.
- **Known limits, so don't plan around them:** `%slots` takes only the instantiated usage's own name
  — `%slots test::electricVehicle::engine` answers `no instance of …`, and
  `%eval test::electricVehicle.engine.power` answers `usage test::electricVehicle has no value`.
  Nested traversal is only observable through the indented nested rendering of the top-level `%slots`.
- **Careful with `clear` while recording:** typed at the `sysml>` prompt it is parsed as a
  declaration (`1:1: error: expected a namespace member`) *and* drops previously created instances,
  so the next `%slots` says `no instance of …`. Use `%clear` to reset the session, and clear the
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
  and `%slots M::p` still shows `k = 2.00`, `x = 1.00`, `total = 3.00`, with `%eval M::p.x` → `1.00`.
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
  RHS: unresolved feature: localGain`.
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
  surface for "does this name resolve?" is `%instantiate` / `%slots` / `%eval` (a `part def` is
  easiest via `%instantiate`, an attribute via `%eval`), all funnelling through
  `internal/repl/lookup.go`. A request phrased as "`%what`/lookup" means those.
- Symbol-taking commands: `%instantiate %slots %eval %calc %constraint %requirement %action %state`.
  All go through one helper (`internal/repl/lookup.go`), so test each with a **simple** name and a
  **qualified** one.
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
  stop resolving (`symbol "DescentR::propagate" not found`). Type shell and REPL commands in
  separate turns, and `%clear` (or restart) if a stray line lands in the buffer.
- `%satisfy` takes no argument (every satisfaction assertion the model states) or the name of the
  element stating them, since `assert satisfy … by …` is anonymous.
- Instances are keyed by resolved FQN, so `%instantiate Vehicle` then `%slots Demo::Vehicle` must
  hit the same `ID`, and the reverse spelling too. Differing IDs = broken keying.
- Qualified attribute access works with a full FQN (`%eval Demo::Engine::power` → `= 300.00`) but a
  **partial** qualification (`%eval Engine::power`) is `symbol ... not found` — the qualified path
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
  fixtures prints a tier-2 `unresolved reference: n` diagnostic for the `assign` that reads the
  accepted parameter; the runtime still binds it, so don't mistake that for a new failure.
- Unsatisfiable: write a one-accept action with nothing sending to it. `%step` → `State: Waiting`,
  and `%continue` must return a typed
  `accept deadlock in action <name>: nothing can post the awaited message (...)` — **never** hang.
  A breakpoint on the parked node still fires, and `%stop` then `%continue` gives
  `no active action session`.

Always run these under `timeout` when driving over a pipe; a hang is the failure mode to catch.

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
`action node <n>: 'for' collection must be a sequence or a set, got constant`.

### Raising the budgets (PRs #83, #87)

Five variables, one per runaway bound, each counting a different unit — raising one says nothing
about the others:

| Variable | Default | Counts |
|---|---|---|
| `SYSML_MAX_STEPS` | 10000000 | expression evaluations |
| `SYSML_MAX_ACTION_STEPS` | 1000000 | action token-flow steps |
| `SYSML_MAX_EVENTS` | 1000000 | state machine events, and the events one `%advance` drains |
| `SYSML_MAX_DO_STEPS` | 5000000 | do-activity actions, ditto for `%advance` |
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

`Session.accept` (internal/repl/session.go) drops any earlier snippet whose **declared names**
intersect the new submission's. So typing `package Demo { part def Trailer { ... } }` to *add* a
member to an already-loaded `package Demo` **replaces the whole package** — `Demo::Vehicle` and
friends become `symbol ... not found`, while `%instances` still lists the now-orphaned instance and
instance IDs restart. When you just want to add declarations mid-session, use a **different package
name**. When you want to prove that newly-typed declarations are visible to the qualified-name path
(the `s.idx`/`s.rtCtx`/`s.instances` invalidation in `Submit`), a fresh package avoids conflating the
two behaviors.

`Submit` also resets `s.instances`, so **any** submission wipes previously created instances:
expect `%instances` → `(no instances created)` right afterwards, the next `%instantiate` to get
`ID: 1`, and `%slots` on a still-valid symbol to say `no instance of "…" (use %instantiate first)`
rather than printing stale slots. An active `%action`/`%state` debugging session is *deliberately*
not reset by a submission — it keeps running against the executor built from the older document. Per
the maintainer this is intended; do not report it as a bug.

Also: analysis still runs over the whole accumulated buffer, but since PR #65 the **report** is
scoped to the submission just made, so one bad snippet no longer keeps re-printing its error on
later submissions. Two consequences when testing:

- Reported line/column numbers are **relative to what you just typed** (`Result.Offset` /
  `baseLine()` in `internal/repl/render.go`), so a one-line submission reports `1:36:` no matter how
  much is already in the buffer. Only `%verbosity debug` numbers against the whole buffer.
- While an earlier error is unresolved, a later clean submission prints
  `note: an earlier session error is unresolved, so deeper checks may not have run here (see it with
  -debug)` before its summary. That note is expected, not a failure.

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
%instantiate P::Widget      -> error: symbol "P::Widget" not found  (re-export unwound)
%instantiate Lib::Widget    -> ✓ Created instance of Lib::Widget    (purge must not over-remove)
package P { public import Lib::*; }
%instantiate P::Widget      -> ✓ ... again, and never "is ambiguous"
```

Notes that save time:

- A re-export registers the symbol under the importing namespace but **never copies its subtree**:
  `P::Widget` resolves while `%eval P::Widget::size` is `symbol ... not found`. Use the declaring
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
them (`internal/core/runtime/trace.go`). `%slots` on a model with derived attributes is the easiest
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

`docs/QUICKSTART.md` and `README.md` contain REPL transcripts that are easy to let rot. Verify them
by **typing them by hand** at the prompt in a GUI terminal, not over a pipe: some failure modes (a
blank line inside a braced declaration ending the submission early) only exist interactively.
Discover the expected values over a pipe first, then do one clean recorded pass.

As of PR #107 the QUICKSTART/README action- and state-debugging transcripts are real captured output
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
  content-dependent (`saved 181 bytes of sysml` / `saved 1872 bytes of ttl` for the QUICKSTART
  `MyModel` file) — recompute them whenever the sample model changes.
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
fail — that is the documented `docs/ROADMAP.md` §P1 gap, not a new bug.

Client API shapes that are easy to get wrong:

- `Model.find(name)` returns **one `Symbol` or `None`**, not a list — iterating it raises
  `TypeError: 'Symbol' object is not iterable`. Use `.id` (FQN), `.name`, `.kind`.
- `pysysml.instantiate(fqn, file_path=...)` and `pysysml.eval(expr, file_path=..., context_symbol_id=...)`
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
  `pysysml.eval('1 + 1', ...)`.
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

Suite baseline: `cd python && python -m pytest tests/ -q` with no service running should be
`148 passed, 24 skipped` (~40s; it was `75 passed, 18 skipped` before the Tier 1/Tier 2 client
work), and `158 passed, 14 skipped` with a service running. The skips are the integration
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
`~/.pysysml/sysml-grpc.{pid,refcount}`; clear them before the next liveness test.
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
`internal/core/libs/stdlib/Systemica Libraries/SystemicaMathFunctions.kerml`.
Testing notes that generalize to any future built-in:

- The fastest end-to-end surface is the batch flag, which loads a model *and* evaluates
  expressions against it: `./bin/sysml -e "exp(1.0)" -e "log(8.0, 2.0)" model.sysml`. Several
  `-e` flags are allowed and each prints `✓ <expr>` then `  = <value>`; a failure prints
  `error: evaluation failed: ...` and the process still exits 0, so assert on the text, never on
  the exit code.
- **`-e` is evaluated in the root scope, not inside the model's package.** So a model that declares
  its own `calc def exp` is *not* exercised by `-e "exp(2.0)"` (that hits the built-in). To prove
  shadowing, either use the FQN (`-e "OwnExp::exp(2.0)"`) or read it out of an attribute default
  with `%instantiate`/`%slots`. Getting this wrong looks exactly like a shadowing bug.
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
  report it as expected, and use `import SystemicaMathFunctions::*;` in fixtures to avoid it.
- Fixture gotchas when writing action fixtures by hand: `done` is a reserved keyword (use another
  name), successions must use the `first start; … done end; then a b;` form (the
  `first a then b;` form yields `action has multiple initial nodes`), and `and`/`or` are
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
   `%instantiate`/`%slots` to see what the member actually did (there, nothing).
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
   assertions that separate working from broken are: `%slots` lists the member as **`x = 5`**
   (broken revisions show `sn = 5` with `x = 1`, or a bogus `redefines = <unknown>` slot);
   `%eval T::A::x` **and** `%eval T::A::sn` both evaluate; and an action body doing
   `assign total := total + x` completes instead of `unresolved feature: x`. Load-level `✓ package T`
   proves nothing here.

Go may not be at the blueprint's `/usr/local/go/bin` on every box; check `~/sdk/go/bin` too
(`PATH=$HOME/sdk/go/bin:$HOME/go/bin:$PATH`) before concluding the toolchain is missing.

### Naming changes that reach lowering and the runtime

When a parser change alters which declarations carry a name (e.g. `ast.EffectiveName` deriving the
name from a `redefines`/`references` target), the parse is the *least* interesting surface —
consumers reading `Usage.Ident.Name` break silently downstream. The probes that actually distinguish
fixed from broken, each with a visible A/B against `main`:

- **Attribute default** — `attribute redefines x = 5;` in an action body with a statement reading
  `x`; a lost name surfaces as `error: execution failed: eval assignment RHS: unresolved feature: x`,
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
(`%slots` shows `sn = 9` and leaves `x`/`y` at their inherited values) with no diagnostic — assert
which key the value landed under rather than assuming it was dropped. REPL call syntax: named
arguments work as `Scaled(x = 7, factor = 5)`, but *mixing* positional and named
(`Scaled(7, factor = 5)`) is a parse error on every revision — don't read that as a
parameter-naming bug.

## Quantities, unit-name resolution and prompt scope

A name in the unit position of `x [u]` is an ordinary feature reference, so it resolves to the
**nearest** declaration and then must conform to a measurement unit. Testing anything in this area:

- There are **four** evaluator paths that reach a unit name and they must agree: a slot
  (`%instantiate` + `%slots`), an action (`%action`), a calc (`%calc`), and a constraint
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
  `%eval a + b` is `unresolved feature: a`); qualify them (`%eval P1::a + P2::b` = `3.00`).
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

## Multiplicity, subsetting and collection slots in `%slots`

`%slots` is the cheapest window on instantiation semantics, and the interesting values are all in
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

## Recording setup (Linux/Plasma box)

The GUI is on `DISPLAY=:0` (`:1` does not exist here — `wmctrl` will say "Cannot open display").

```bash
cd /home/ubuntu/repos/Systemica && (DISPLAY=:0 konsole --hide-menubar >/dev/null 2>&1 &)
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
  materialize on the stdlib base. A bare `connect a.p to b.q;` has no slot name, so `%slots` renders
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
  `f = <unknown>` and a named `binding` gets no `%slots` line at all. Don't plan an end-identity
  assertion on them.
- **Variant-interface routing is only partly reachable from the REPL.** `%slots` proves the
  *materialization* side: in
  `internal/core/runtime/testdata/conformance/ballandchain_variant_configuration.sysml` the selected
  `engagementRingToBand = engagementRingToBandConnected (Instance ID: 24)` holds
  `engagementRing.ringPort` / `band.ringPort`, not the disconnected variant's ports. But the
  send/accept *routing* side cannot be driven: an action usage does not inherit its `action def`'s
  nodes (`initialize action: no initial node found`), `%action` cannot resolve a nested action of a
  selecting part usage (`symbol "Route::sysDirect::comm" not found`), and a sibling
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
  exactly which end must be which port — the cheapest source of the IDs `%slots` should tie together.
- Ball-and-chain reference numbers, for asserting the cost roll-up did not shift:
  `totalCost = 1450.00`, `band` `bandCost`/`ringCost` `= 400.00`, `engagementRing`
  `engagementRingCost`/`ringCost` `= 500.00`, `diamondCost = 550.00`, and
  `engagementRingToBandConstraint: <constraint: satisfied>`.

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
  Integer = c.a;`) and inspect with `%slots <part>`; a failing read shows inline as
  `a: <error: slot p.a: …>` rather than aborting the listing, so one `%slots` can carry several
  independent negative assertions. Useful expected strings: `no value: output never assigned`,
  `output bound more than once` (declaration value plus a body assignment; two *body* assignments
  are legal, the last one winning), `assignment outside the calculation body: <name> is not declared
  by the calculation`, `no result expression: calc … has no return expression`.
