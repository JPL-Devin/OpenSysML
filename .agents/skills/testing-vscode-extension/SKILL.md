---
name: testing-vscode-extension
description: How to build, install, drive and record end-to-end GUI tests of the OpenSysML VS Code extension (editors/vscode) and its sysml-lsp language client — server discovery, completion, diagnostics, and the xdotool pitfalls that waste time.
---

# Testing the OpenSysML VS Code extension

## Build & install (always rebuild; a stale vsix looks identical)

Run everything from the repo root (`AGENTS.md` §2: never `cd`):

**First check that a real VS Code desktop exists.** On some boxes `code` on PATH is only the Devin
CLI standalone (`code --version` prints `Devin CLI Standalone`), which refuses
`--install-extension` ("No installation of Devin stable was found"), and the pre-running
`serve-web` instance on `localhost:6789` may render a blank workbench. `sudo` is usually
passwordless, so the fastest fix is installing the real desktop build:

```bash
curl -sL -o /tmp/code.deb "https://update.code.visualstudio.com/latest/linux-deb-x64/stable"
sudo dpkg -i /tmp/code.deb        # takes ~1 min; then /usr/bin/code is the desktop editor
```

Launch it detached or it dies with the shell that started it:
`DISPLAY=:0 setsid nohup /usr/bin/code --no-sandbox --disable-gpu <workspace> >/tmp/code.log 2>&1 </dev/null &`

```bash
make build                # produces bin/sysml-lsp (the server the client discovers)
make vscode-package       # npm ci + typecheck + esbuild + vsce -> editors/vscode/opensysml-sysml.vsix
DISPLAY=:0 code --no-sandbox --disable-gpu --install-extension editors/vscode/opensysml-sysml.vsix --force
DISPLAY=:0 code --no-sandbox --list-extensions --show-versions   # expect open-mbee.opensysml-sysml@<ver>
```

Restart VS Code after installing, otherwise the old extension host keeps running:

```bash
pkill -f "no-sandbox --disable-gpu /home"      # plain `pkill -f /usr/share/code` may miss it
DISPLAY=:0 nohup code --no-sandbox --disable-gpu "$PWD" &   # $PWD = repo root, the workspace to open
DISPLAY=:0 wmctrl -r :ACTIVE: -b add,maximized_vert,maximized_horz
```

Workspace trust must be granted — Restricted Mode silently disables the extension (no LSP, no outline).
The trust banner appears on the Welcome tab; "Manage" → "Trust" reloads the window.

**Always open the repo *folder*, not a lone `.sysml` file.** `code <file.sysml>` gives the window no
workspace folder, so `resolveServer` skips the `<workspace>/bin/sysml-lsp` fallback, finds nothing on
PATH, and you get an empty "SysML v2" channel plus 0 problems — which looks exactly like a broken
server. Launch with the repo root as the argument, then open the file from the Explorer.

## Server discovery (`editors/vscode/src/extension.ts`)

**`--stdio` crash loop (seen at b3f16e4, pre-existing on `main`).** The client uses
`TransportKind.stdio`, which makes vscode-languageclient append `--stdio` to the server argv, while
`cmd/sysml-lsp/main.go` parses flags with `flag` and rejects unknown ones. The symptom is a toast
`Client SysML v2 Language Server: connection to server is erroring. write EPIPE`, then in the
"SysML v2" output channel `flag provided but not defined: -stdio` / `Usage: sysml-lsp [options]` /
`Server process exited with code 2` and finally
`server crashed 5 times in the last 3 minutes. The server will not be restarted.`
`pgrep -a sysml-lsp` is empty and the Problems panel stays empty (which looks exactly like "the
feature under test produces no diagnostics" — always check `pgrep` before believing that).
Reproduce outside the editor with `./bin/sysml-lsp --stdio </dev/null`.
Workaround for testing (no repo edit needed): a wrapper that drops argv, pointed at from the
**Settings UI** (`Ctrl+,` → search `opensysml.server.path`, User scope) — the setting has an
`onDidChangeConfiguration` handler, so the server restarts on Enter and
`Starting /tmp/lsp-wrap.sh` appears in the channel:

```bash
printf '#!/bin/sh\nexec /home/ubuntu/repos/OpenSysML/bin/sysml-lsp\n' > /tmp/lsp-wrap.sh
chmod +x /tmp/lsp-wrap.sh
```

The proper fix (report it, do not apply it while testing) is either accepting/ignoring `-stdio` in
`cmd/sysml-lsp/main.go` or dropping `transport` from the `ServerOptions` in `extension.ts`.

Order is `opensysml.server.path` → `<workspace>/bin/sysml-lsp` → `sysml-lsp` on PATH. On this box
`sysml-lsp` is normally NOT on PATH, so a green run genuinely exercises the workspace `bin/` fallback.
Proof lives in the **"SysML v2" output channel**: `Starting /home/.../bin/sysml-lsp`.
Cross-check the process with `pgrep -a sysml-lsp` — the pid is the cheapest, most reliable signal for
"did the server restart/crash?" (the fixed double-read-loop bug manifested as repeated restarts and
`missing Content-Length header` lines in that channel).

Setting `opensysml.server.path` to a bogus path is expected to show a warning
("Could not find sysml-lsp ... Syntax highlighting still works"), kill the server (pgrep empty),
empty the Outline view, but keep TextMate colors. Clearing the setting auto-restarts (there is an
`onDidChangeConfiguration` handler), and `SysML: Restart Language Server` restarts it too.
`[Error - hh:mm:ss] Server process exited with code 0.` in the channel is benign shutdown noise from
vscode-languageclient after a clean stop — not a crash.

## Completion expectations (`internal/lsp/completion.go`)

Trigger characters are `.` and `:`. Inside a body:
- `engine.` → only that type's members with real kinds/details (`power` → `attributeUsage`,
  `start` → `actionUsage`). A member whose nearest declaration is an untyped redefinition (e.g.
  `attribute redefines power = 110.0;` in `part myCar`) shows no `: Type` suffix — that is correct,
  the nearer declaration wins.
- Unresolved path (`zzz.`) → LSP returns nothing. VS Code then shows its own **word-based**
  suggestions (plain `abc` icons, no detail column). Do not mistake those for a keyword dump; the
  real LSP keyword items carry the detail text `keyword`. A good contrast shot: Ctrl+Space on an
  empty line shows LSP items with `keyword` details and `{}` library packages.
- `ScalarValues::` → library members (`Real`, `Boolean`, `Integer`, ... with `attributeDef` detail).

## Semantic tokens (`internal/lsp/semantictokens.go`, `internal/core/highlight`)

The client enables `textDocument/semanticTokens/full` automatically; only `editor.semanticHighlighting.enabled`
gates it (note a workspace `.vscode/settings.json` value overrides the User setting — flip it in the
**Workspace** tab or the toggle looks like a no-op).

The reliable, screenshot-able proof is Command Palette → **Developer: Inspect Editor Tokens and
Colors**: it shows a `semantic token type` / `modifiers` block plus the TextMate scope it *struck
through*. Move the cursor with **Ctrl+G `line:col`** while the inspector is open — the panel covers
the code and blocks clicks, and VS Code's Col is UTF-16 based, so it doubles as the UTF-16 column
check for tokens after astral-plane characters (a 🚗 counts 2).

Toggling semantic highlighting off is only a *sometimes* visible before/after: this TextMate grammar
gives type names the same `#4EC9B0` the semantic `class` gets, so compare a **keyword**
(`package`: semantic `#C586C0` vs TextMate `#FF7B72`), not a type name.

Deltas (`semanticTokens/full/delta`) are deliberately unimplemented — the server answers -32601;
verify that over stdio JSON-RPC, not from the GUI.

## Quick-fix code actions (`internal/lsp/codeaction.go`, `internal/core/resolve/fixes.go`)

Cursor on the diagnostic + **Ctrl+.** (`ctrl+period` via xdotool works). Copilot always injects its
own `Fix`/`Explain` entries, so "no server fix offered" looks like a menu with *only* those two —
not the "No code actions available" message. Expected titles/edits:
- near miss: `Change 'Wheeel' to 'Wheel'` (preferred)
- resolvable elsewhere: `Change 'Integer' to 'ScalarValues::Integer'` **and** `Import 'ScalarValues::*'`,
  the latter inserted as its own line before the first member, matching that member's indentation
- missing `;`: `Insert ';'` — the diagnostic sits on the *next* token (often the closing `}` a line
  below), but the edit must land at the end of the previous statement
- ambiguous (`expected '{' or ';' after declaration`): no fix, by design

A fast way to learn exact titles/ranges before driving the GUI is a small stdio JSON-RPC probe
script against `bin/sysml-lsp` (initialize → didOpen → semanticTokens/full → codeAction).

## Lifecycle / process-leak testing (`cmd/sysml-lsp/main.go`, `internal/lsp/lifecycle.go`)

- The client always appends `--stdio` (`vscode-languageclient/lib/node/main.js`: `TransportKind.stdio`
  → `args.push('--stdio')`), so the server binary must accept that flag or the client crash-loops.
- `pgrep -af sysml-lsp` run from a shell whose own command line contains the string `sysml-lsp`
  matches that bash process and gives a false positive. Put the check in a tiny script
  (`/tmp/lspcheck.sh`) and call it, so the output is only real servers.
- Expected argv while a window is open: exactly one `<repo>/bin/sysml-lsp --stdio`. After **File →
  Close Window** it must disappear within a few seconds; a surviving process is the leak bug.
- Cheap, high-signal stdio probe for exit statuses (no GUI): initialize → `shutdown` → any request
  (expect error `-32600`) → `exit` ⇒ process status 0; initialize → `exit` with no shutdown ⇒ status 1.
  A process still alive after 10 s is the old bug.
- A convincing **negative control** without touching the branch: `git worktree add /tmp/wt-main
  origin/main`, `go build -o /tmp/sysml-lsp-main ./cmd/sysml-lsp` in it, copy over `bin/sysml-lsp`,
  reopen the folder — the channel then shows `flag provided but not defined: -stdio`,
  `Server process exited with code 2` and `crashed 5 times in the last 3 minutes`. Restore the fixed
  binary and run **SysML: Restart Language Server** to recover in the same window (no relaunch needed).

## GUI driving pitfalls (cost real time here)

- `ctrl+shift+u` via xdotool types a literal `u` into the editor instead of opening Output. Use the
  **View → Output** menu, or the Output tab in the bottom panel.
- The **"SysML v2" channel is often missing from the Output panel's channel dropdown**. Reliable route:
  Command Palette → **"Output: Show Output Channels..."** → type `SysML` → Enter.
- `shift+alt+a` (Toggle Block Comment) does not reach VS Code; use Command Palette
  "Toggle Block Comment" instead. `ctrl+slash` works fine.
- Typing long lines with `type` while the suggest widget is open silently swallows characters
  (`Engine` → `ngine`). Press `Escape` between chunks, or accept that stress-test text is garbled.
- The bottom panel is read-only: click into the editor area before typing, and close the panel
  (X at its top-right) before triggering completion so the popup has room.
- Undo a stray edit with Command Palette **"File: Revert File"** — it is far more reliable than
  counting Ctrl+Z presses, and leaves the git tree clean.

## Multi-file / workspace-indexing testing (`internal/lsp/files.go`, `sync.go`)

The cleanest fixture is a **throwaway folder outside the repo** (e.g. `/home/ubuntu/ws-multifile`)
holding only a couple of tiny models, so the Problems count is entirely about the feature:

```
lib.sysml   package Lib { part def Widget; }
main.sysml  package Main { import Lib::*; part w : Widget; }
```

- A workspace outside the repo has no `bin/sysml-lsp`, so point `opensysml.server.path` at
  `/home/ubuntu/repos/OpenSysML/bin/sysml-lsp` (User `settings.json` or the Settings UI) — the
  setting takes precedence and restarts the server on change.
- Writing `~/.config/Code/User/settings.json` with `"security.workspace.trust.enabled": false` and
  `"workbench.startupEditor": "none"` avoids the trust banner and the Welcome tab entirely.
- The **Problems panel (`ctrl+shift+m`) plus the status-bar error count** is the high-signal oracle
  for indexing tests: "unresolved reference: Lib/Widget" appearing/disappearing is the whole test.
- A convincing negative control is a one-line switch of `opensysml.server.path` to a binary built
  from `origin/main` (`git worktree add /tmp/wt-main origin/main && go build -C /tmp/wt-main -o
  /tmp/sysml-lsp-main ./cmd/sysml-lsp` — build *in the worktree*, or you rebuild the branch);
  the pre-indexing server shows the unresolved references on the same file.
  Switching the setting back auto-restarts — no window reload needed. Remember `git worktree remove`.
- Watcher tests (create/change/delete a `.sysml` outside the editor) are driven from the shell with
  `printf > file` / `rm`; VS Code's `**/*.{sysml,kerml}` watcher forwards them and the Problems panel
  updates within ~1-2 s. A deleted file whose tab is still open keeps the buffer authoritative — the
  diagnostics only change when that tab is closed.
- Verify the server was not silently restarted between steps: `pgrep -af sysml-lsp` pid must be
  unchanged (put it in `/tmp/lspcheck.sh` per the note above).

### More GUI driving pitfalls found here
- `shift+F12` (Find All References) does not reach VS Code through xdotool — use Command Palette
  **"References: Find All References"**. `F12` (go to definition) does work.
- Closing an editor tab: **middle-click the tab**. Clicking the tab's little `x` needs a hover first
  and the coordinates shift whenever the sidebar collapses; `ctrl+w` is risky (can close the window).
- `key` actions take ONE combo: `"shift+Down shift+Down"` errors with `unknown key`; send two actions.
- Opening a file by name with `ctrl+p` → type `lib.sysml` → Enter is far more reliable than clicking
  the Explorer tree, especially after the sidebar has been toggled.
- **Do not use "completion after `Qualifier::`" as the completion oracle.** As of 0b239642 typing
  `attribute a : ScalarValues::` and pressing `ctrl+space` yields "No suggestions", and with a
  trailing `::` the file is also a syntax error (`expected a name after '::'`), which suppresses
  semantic completion. Use a *plain* name prefix instead: in a syntactically valid file, `Wh` +
  `ctrl+space` returns LSP items with a type detail (e.g. `Wheel  partDef`) — the detail column is
  what distinguishes real LSP items from VS Code's word-based (`abc` icon) suggestions.
- A stray `u` from `ctrl+shift+u` is easiest to remove by selecting the whole buffer (`ctrl+a`) and
  retyping the fixture; `ctrl+z` after an LSP-driven edit sometimes only reverts part of it.
- After rebuilding `bin/sysml-lsp`, run Command Palette **"Developer: Reload Window"** so the client
  respawns against the new binary; killing the server process alone can leave the old one in use.

## Diagram panel / `opensysml/render` testing (`editors/vscode/src/diagram.ts`, `src/webview/diagram.ts`)

- Fixtures: `examples/views-demo.sysml` declares 7 views (3 drawable trees, 3 `not drawable`);
  `examples/state-machine-demo.sysml` declares none and therefore exercises the `#tree` pseudo-view
  fallback. `views-demo.sysml` has ~13 pre-existing Problems on the branch — do not use the Problems
  count as the pass/fail oracle for rendering.
- Learn every expected value *before* driving the GUI with a stdio JSON-RPC probe against
  `bin/sysml-lsp`: `initialize` (check `capabilities.experimental.openSysmlRender`), `textDocument/didOpen`,
  `opensysml/views`, `opensysml/render` (`{textDocument, view, form:"mermaid"}`), then `didChange` to see
  the `opensysml/renderChanged {textDocument, version}` push. That gives you node ids, node counts,
  origin line numbers and notice counts to assert against pixels.
- With a declared-view document the panel opens with **no view selected** and the status line asks you to
  name one — that is the implemented initial state, not a failure. Pick a view in the picker.
- **Tables are not Mermaid.** As of 54118526 the client requests *no* `form`, so the server returns
  `form=markdown` for a table kind and the webview shows the artifact text in a `<pre>` (header row
  `| Element | Kind | Type | Declared in |`). If a client build ever hardcodes `form:"mermaid"` again,
  `#table` fails with `a table rendering is not written as mermaid; ask for text or markdown`.
  A **declared** `render asElementTable` view (`LanderViews::partsTable`) does render — a probe from a
  workspace holding only `views-demo.sysml` reports it `supported:true`, `form=markdown`. It only lists
  `supported:false` when the workspace *is this repository*, because the parser fixture
  `internal/core/parser/testdata/parse/view_expose.sysml` declares a `package Views` that shadows the
  standard library's, so `render asElementTable` no longer resolves to a standard rendering. Test the
  diagram panel from a scratch folder, not the repo root, or expect that shadowing.
- "Never blank" needs the webview-state cache. Hiding the panel (switching the other tab group to a
  different tab) tears the webview down; on re-show it reconstructs from `vscode.getState().last`. Test
  it explicitly, and test it with a view whose render *errors* — that is the case that blanked before
  the cache existed.
- **Click-to-source / cursor-highlight break easily on Mermaid id mangling.** Mermaid 11 decorates node
  ids on *both* sides — `<diagramId>-flowchart-<serverNodeId>-<n>` (e.g.
  `opensysml-diagram-3-flowchart-n2-0`), and `statediagram-state-<id>-<n>` for state diagrams — so a
  `bareID()` that only strips a *leading* `flowchart-`/`state-` never matches the server's `n0…nN`.
  Diagnose it (not with the GUI) in Command Palette → **Developer: Open Webview Developer Tools**, pick
  the `active-frame (index.html)` context in the context dropdown, then run
  `document.querySelectorAll('#diagram svg g.node').length` vs
  `document.querySelectorAll('#diagram .opensysml-node').length` and
  `[...document.querySelectorAll('#diagram svg g.node')].map(g=>g.id)`.
  `0 marked` ⇒ ids don't match; clicks and highlights will silently do nothing.
  Always check a **state** diagram as well as a flowchart — they take different id shapes.
- A second, independent trap: a highlight rule like
  `.opensysml-selected > rect { stroke: var(--vscode-focusBorder) }` loses to Mermaid's own
  `<style>` block injected *inside* the SVG (same specificity, later in document order). Check
  `getComputedStyle(rect).stroke` — if it is still Mermaid's grey, the highlight is invisible even
  though the class is applied. `!important` on `stroke`/`stroke-width` fixes it.
- **Judging "the node is outlined" needs the right fixture.** `LanderViews::overview` draws 14 boxes
  scaled down to ~30 px wide, where a 3 px stroke is unreadable in a screenshot. Use a small view —
  `LanderViews::safetyView` renders 4 large boxes — so the focus-border outline is unmistakable.
  For an objective check independent of eyeballing, crop the same diagram rectangle out of two
  full-resolution screenshots and run `compare -metric AE a.png b.png null:` — `0` means nothing
  changed on screen at all. (Screenshots land at 1600x1200 while the tool's coordinate space is
  1024x768, so multiply crop coordinates by 1.5625.)
- **Cursor-to-node ranges run declaration-start → next-declaration-start**, so a cursor in the *leading
  indentation* of a declaration line resolves to the **previous** element, and column 1 of the first
  declaration line matches nothing. Drive this test with `Ctrl+G line:col` (e.g. `112:20`) or by
  clicking on the declaration text, never `Ctrl+G <line>` alone, or you will report a false failure.
- Live-redraw proof: type a new member (e.g. `part booster : Tank;`) inside a `part def` that the view
  exposes and screenshot before/after — a new box must appear with no manual refresh. Undo with
  Command Palette **File: Revert File** to keep the git tree clean.
- Parse-error proof: delete a closing `}`. The previous SVG must stay on screen at reduced opacity
  (`.stale`) with a red status line; zoom on the status line, and compare box brightness before/after.

## TextMate grammar changes (`editors/vscode/syntaxes/*.tmLanguage.json`, `tools/gengrammar`)

- A *real* before/after for a grammar rule needs no branch switching and no repackaging: install the
  vsix once, then swap the single JSON in the **installed** extension and reload the window.
  ```bash
  git show origin/main:editors/vscode/syntaxes/sysml.tmLanguage.json > /tmp/sysml-main.tmLanguage.json
  EXT=~/.vscode/extensions/open-mbee.opensysml-sysml-0.1.0/syntaxes/sysml.tmLanguage.json
  cp /tmp/sysml-main.tmLanguage.json "$EXT"                                    # BEFORE
  cp editors/vscode/syntaxes/sysml.tmLanguage.json "$EXT"                      # AFTER (restore!)
  ```
  Command Palette → **Developer: Reload Window** picks the new grammar up; nothing else is needed.
  Always restore the branch copy when done — the installed file is not under git, so `git status`
  will not remind you.
- **Semantic tokens beat TextMate for declared names**, so a word that a new TextMate rule colours
  may still render plain where the LSP marks it as a usage/variable. Example: with the F9
  `keyword.other.contextual` rule, `initial off;` / `region r { }` / `entry point p;` colour as
  keywords, but `attribute point : ScalarValues::Real;` keeps the plain identifier colour. Pick the
  screenshot line accordingly, and prove the scope with **Developer: Inspect Editor Tokens and
  Colors** (`region` → `keyword.other.contextual.source.sysml`, foreground `#FF7B72`).

### Completion oracle for keyword-ish items

Type a real prefix (`poi`, `def`, `var`) then `ctrl+space` and read the **detail column** in a `zoom`:
`keyword` = reserved, `contextual keyword` = the F9 list, no detail + `abc` icon = VS Code word-based
noise. Language gating is visible this way too (`var` only in `.kerml`).
Beware the server **dedupes by label**: in a document that already declares `attribute point`, the
contextual item `point` disappears (a document symbol wins). Use a fixture *without* those names when
asserting the contextual list, and a separate fixture with them for the "still ordinary names" test.
Confirm the whole expected list cheaply first with a stdio JSON-RPC completion probe against
`bin/sysml-lsp`, then prove it in the GUI.

## Recording tips

Record the VS Code window maximized (wmctrl above). Verify visual claims by `zoom`ing the status bar
(language indicator "SysML v2"/"KerML", problem counts) and the completion popup — the popup's detail
column is too small to read in a 1024x768 full screenshot.

## Devin Secrets Needed

None.
