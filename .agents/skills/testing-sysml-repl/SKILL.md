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

## Things to exercise (and their expected shapes)

- Symbol-taking commands: `%instantiate %slots %eval %calc %constraint %requirement %action %state`.
  All go through one helper (`internal/repl/lookup.go`), so test each with a **simple** name and a
  **qualified** one.
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

Things that must fail rather than hang: the REPL builds its runtime context with **maxSteps =
100000** (`internal/repl/session.go`), and every loop iteration spends one step, so a runaway loop
(or an empty loop body, whose condition can never change) returns
`error: execution failed: eval … : evaluation step limit exceeded (100000 steps)` in well under a
second. Always follow the failure with another meta-command (`%tokens`, `%instances`) to prove the
session survived — `%tokens` still shows the token parked at the node with its partial value. A
`for` over a non-collection gives
`action node <n>: 'for' collection must be a sequence or a set, got constant`.

A name declared inside a loop or branch body lives in a block frame and must **not** appear in the
action's `Results:` — check for the *absence* of the line, not just the right total.

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

`docs/QUICKSTART.md` and `README.md` contain REPL transcripts that are easy to let rot. The
traffic-light state-machine transcript (QUICKSTART ~line 396) is real captured output — recreate the
model in a file, replay `%state TrafficLight`, `%advance 25`, `%current`, `%advance 5`, `%advance 30`
and diff line for line. The **action-debugging** transcripts in both files are illustrative, not
captured: they show `Action: MyWorkflow` / `State: Ready` / `Tokens: 1 (at compute)` /
`Token 1: compute { }` / `✓ Completed` / `Result: 42`, whereas the binary prints
`✓ Started action executor for "…"` / `State: Running` / `✓ Step complete` / `Token 1 @ compute` /
`✓ Action completed` + `Final state:` + `Results:`. Don't treat that mismatch as a new regression,
but it is worth flagging.

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
- `null: '<error text>'` is the real error arm. Force it with cyclic derived attributes
  (`attribute a = b + 1.0; attribute b = a + 1.0;`) — expect
  `slot Loop.a: slot Loop.b: cyclic slot dependency: Loop.a`, promptly, and prove the service is
  still alive afterwards with a follow-up `pysysml.eval('1 + 1', ...)`.
- A nested `part engine : Engine;` marshals as bare `instance_id=N` and **no RPC resolves an id**,
  so the REPL expands the child's slots and Python cannot (`docs/ROADMAP.md` §P2).

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
`75 passed, 18 skipped` (~35s). The 18 skips are the integration tests gating on a live service.

Download paths (`python/pysysml/binary.py`) are testable without a real release: move
`~/.pysysml/bin/sysml-grpc` aside, unset `PYSYSML_GRPC_VERSION`, and call `ensure_binary()`,
`resolve_latest_version()`, `download_binary('latest')`. All three must raise `ConnectionError`
naming the path or URL. `PYSYSML_GITHUB_REPO` overrides the repo. Beware: these hit the
unauthenticated GitHub API, so repeated runs flip from a truthful `HTTP Error 404: Not Found` to a
misleading `HTTP Error 403: rate limit exceeded` — rehearse sparingly and report the 404 wording,
not whichever one the recording happened to catch.

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
it works from a tool shell; run `source /home/ubuntu/repos/fprime/fprime-venv/bin/activate` (or
whichever interpreter `python -c 'import sys; print(sys.executable)'` reports in the tool shell)
as a setup step before recording. Discover expected values with the
piped-stdin form *before* recording, so the recorded run is one clean pass; anything only verified
over a pipe is not visible in the video and should be reported as weaker evidence.
