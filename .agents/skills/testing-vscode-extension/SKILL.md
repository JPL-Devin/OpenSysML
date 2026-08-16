---
name: testing-vscode-extension
description: How to build, install, drive and record end-to-end GUI tests of the Systemica VS Code extension (editors/vscode) and its sysml-lsp language client — server discovery, completion, diagnostics, and the xdotool pitfalls that waste time.
---

# Testing the Systemica VS Code extension

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
make vscode-package       # npm ci + typecheck + esbuild + vsce -> editors/vscode/systemica-sysml.vsix
DISPLAY=:0 code --no-sandbox --disable-gpu --install-extension editors/vscode/systemica-sysml.vsix --force
DISPLAY=:0 code --no-sandbox --list-extensions --show-versions   # expect open-mbee.systemica-sysml@<ver>
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

Order is `systemica.server.path` → `<workspace>/bin/sysml-lsp` → `sysml-lsp` on PATH. On this box
`sysml-lsp` is normally NOT on PATH, so a green run genuinely exercises the workspace `bin/` fallback.
Proof lives in the **"SysML v2" output channel**: `Starting /home/.../bin/sysml-lsp`.
Cross-check the process with `pgrep -a sysml-lsp` — the pid is the cheapest, most reliable signal for
"did the server restart/crash?" (the fixed double-read-loop bug manifested as repeated restarts and
`missing Content-Length header` lines in that channel).

Setting `systemica.server.path` to a bogus path is expected to show a warning
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

## Recording tips

Record the VS Code window maximized (wmctrl above). Verify visual claims by `zoom`ing the status bar
(language indicator "SysML v2"/"KerML", problem counts) and the completion popup — the popup's detail
column is too small to read in a 1024x768 full screenshot.

## Devin Secrets Needed

None.
