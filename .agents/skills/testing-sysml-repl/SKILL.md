---
name: testing-sysml-repl
description: How to build, launch and drive the Systemica `sysml` REPL binary for end-to-end runtime testing, including GUI-terminal recording and known REPL quirks that can be mistaken for bugs.
---

# Testing the Systemica `sysml` REPL end to end

The REPL is the user-facing deliverable, so feature verification must go through the
binary, not only `go test`.

## Build and run

```bash
export PATH=/usr/local/go/bin:$PATH     # the VM snapshot may be stale and lack Go
go build -o bin/sysml ./cmd/sysml
./bin/sysml                             # interactive
printf '%%load foo.sysml\n%%slots X\n' | ./bin/sysml   # scripted rehearsal
```

Pipe a script into stdin first to rehearse a scenario and learn the exact output
wording; only then repeat it interactively for the recording. `%` must be escaped as
`%%` inside `printf`. No credentials or network are required.

## Recording a terminal app

The REPL is a user-facing CLI, so drive it in a GUI terminal so the recording shows
real interaction:

```bash
echo $DISPLAY                      # do not assume :1 — check /tmp/.X11-unix
DISPLAY=:0 konsole >/dev/null 2>&1 &
DISPLAY=:0 wmctrl -a Konsole && DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Increase font size with `ctrl+plus` a few times so the recording is legible. The shell
`exec` tool's `DISPLAY` may differ from the desktop's; query it rather than guessing.

## Useful fixtures already in the repo

- `internal/repl/testdata/derived_package.sysml` — package with derived attributes
  (`doubled = mass * 2.0`, `total = mass + engine.derated`), a nested part, and two
  constraints with opposite verdicts (`Derived::Vehicle` passes, `Derived::Heavy` fails).
- `internal/repl/testdata/action_debug.sysml` — `Debug::tally` action for exercising
  `%action` / `%step` / `%continue` (completes with `total = 5`).

Prefer these over hand-typing models; `%load <path>` takes a repo-relative path.

## Gotchas that look like bugs but are not your test's fault

- **Never type a bare shell word (e.g. `clear`) at the `>` prompt.** Anything not
  starting with `%` is parsed as SysML and permanently pollutes the session buffer.
  Use `%clear` to reset the session, or Ctrl-D and relaunch for a truly clean slate.
- **Stale diagnostics are re-echoed.** The REPL re-analyses the whole accumulated
  buffer on every submission, so one bad earlier line reprints its error after every
  later submission, and diagnostic line numbers (`30:42`) refer to the cumulative
  buffer rather than the line you just typed.
- **A successful declaration prints no confirmation while any buffer error exists** —
  `renderResult` emits diagnostics instead of the summary. Do not read the absence of
  `package Foo` as a failed submission.
- **Requirement syntax:** `requirement r { assert x > 0; }` does not parse inside a
  part def; use `requirement r { require constraint { x > 0 } }`.
- **Constraints/requirements only bind to an instance after `%instantiate`.** Before
  instantiation a derived expression legitimately reports
  `unresolved feature: <name>`; instantiate first, then expect an
  `(on <Owner> ID: n)` suffix on `%eval` / `%constraint` / `%requirement` output.
- **`%slots` renders a constraint or requirement usage as a verdict**
  (`<constraint: satisfied>`, `<requirement: violated>`), not as a value; only a
  feature with neither a value nor a verdict shows `<unknown>`.

## Devin Secrets Needed

None — the REPL runs fully offline.
