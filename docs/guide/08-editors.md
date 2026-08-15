# 8. Editors

`sysml-lsp` speaks LSP over stdio, so any editor with a generic LSP client can drive it. The
VS Code extension in [editors/vscode](../../editors/vscode) adds `.sysml`/`.kerml` highlighting
on top.

## VS Code

This repository ships its own VS Code extension in
[editors/vscode](../../editors/vscode): syntax highlighting for `.sysml` and
`.kerml` plus an LSP client that launches `sysml-lsp`. It is not published to any
marketplace, so build and side-load it:

```bash
make build                                    # builds bin/sysml-lsp
cd editors/vscode
npm install
npm run package                               # -> systemica-sysml.vsix
code --install-extension systemica-sysml.vsix
```

Open any `.sysml` file: it is highlighted immediately, and the extension starts
the server it finds, in order:

1. `systemica.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If no server is found, highlighting still works and a warning explains how to
build one. Point the extension at a specific build with `.vscode/settings.json`:

```json
{
  "systemica.server.path": "/absolute/path/to/bin/sysml-lsp",
  "systemica.trace.server": "messages"
}
```

Run `SysML: Restart Language Server` from the command palette after rebuilding
the binary. `editors/vscode/README.md` documents every setting, the grammar
generator (keywords come from `internal/core/lexer.Keywords()`, so they cannot
drift) and the F5 development loop.

Other editors can still launch `bin/sysml-lsp` over stdio through their own
generic LSP client; only the highlighting is VS Code-specific.

**What the server advertises at `initialize`** — the capabilities it answers, taken
from a live session with `bin/sysml-lsp`:

- ✅ Document synchronization, incremental (`textDocumentSync.change: 2`)
- ✅ Diagnostics (syntax + semantic errors, published on open and on change)
- ✅ Hover (symbol info, type, multiplicity)
- ✅ Go-to-definition (cross-document navigation)
- ✅ Find references (workspace-wide search)
- ✅ Completion (trigger characters `:` and `.`; typed kinds and details, `v.` offers members of `v`'s type, `Pkg::` offers that namespace's members, library names included)
- ✅ Document symbols (outline view)
- ✅ Workspace symbols (global search)
- ✅ Document formatting (`textDocument/formatting`, whole-file edit)
- ✅ Rename, with prepare (`textDocument/prepareRename`, `textDocument/rename`)
- ✅ Semantic tokens, full document and range (`textDocument/semanticTokens/full`,
  `textDocument/semanticTokens/range`; legend advertised at `initialize`,
  keywords/comments/literals from the token stream, names classified from the
  symbol table and the resolver, with the `declaration`, `definition`, `readonly`
  and `abstract` modifiers)
- ✅ Code actions, quick-fix kind only (`textDocument/codeAction`: spelling of an
  unresolved name, importing the namespace that declares it, inserting a missing
  semicolon the parser located exactly)

**Not implemented:** semantic token deltas (`semanticTokens/full/delta` — the
server holds no previous result to diff against, so a client re-requests the
full set), signature help, range formatting, code lens, inlay hints. A client
asking for one of those gets the method-not-found answer rather than a partial
result. Quick fixes are offered only where the repair is unambiguous: a syntax
error that may want either a body or a semicolon carries none.

**Test the server:** the protocol is JSON-RPC over stdio, so a request can be sent
by hand. Formatting a badly indented file and renaming a definition, run against
`bin/sysml-lsp`:

```
→ textDocument/formatting  (file: "package P {\npart def Wheel {\nattribute diameter = 16.0;\n}\npart w : Wheel;\n}\n")
← [{"range": {"start": {"line": 0, "character": 0}, "end": {"line": 6, "character": 0}},
    "newText": "package P {\n    part def Wheel {\n        attribute diameter = 16.0;\n    }\n    part w : Wheel;\n}\n"}]

→ textDocument/rename      (position: line 1, character 10; newName: "Tyre")
← {"changes": {"file:///tmp/lsp-demo.sysml": [
      {"range": {"start": {"line": 1, "character": 9}, "end": {"line": 1, "character": 14}}, "newText": "Tyre"},
      {"range": {"start": {"line": 4, "character": 9}, "end": {"line": 4, "character": 14}}, "newText": "Tyre"}]}}
```

The rename edits the declaration and the `part w : Wheel` reference together —
it is resolution-driven, not a textual replace.

In an editor, check the install by hovering: open a file containing
`part Wheel { attribute diameter = 16.0; }` and hover over `Wheel`.

---

Next: [9. Python](09-python.md).
