---
name: testing-vscode-extension
description: How to build, install, drive and record end-to-end GUI tests of the Systemica VS Code extension (editors/vscode) and its sysml-lsp language client — server discovery, completion, diagnostics, and the xdotool pitfalls that waste time.
---

# Testing the Systemica VS Code extension

## Build & install (always rebuild; a stale vsix looks identical)

Run everything from the repo root (`AGENTS.md` §2: never `cd`):

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

## GUI driving pitfalls (cost real time here)

- `ctrl+shift+u` via xdotool types a literal `u` into the editor instead of opening Output. Use the
  **View → Output** menu, or the Output tab in the bottom panel.
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
