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
npm run package                               # -> opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Open any `.sysml` file: it is highlighted immediately, and the extension starts
the server it finds, in order:

1. `opensysml.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If no server is found, highlighting still works and a warning explains how to
build one. Point the extension at a specific build with `.vscode/settings.json`:

```json
{
  "opensysml.server.path": "/absolute/path/to/bin/sysml-lsp",
  "opensysml.trace.server": "messages"
}
```

### Asking the strict question in the editor

The server reports OpenSysML's own notation as warnings, like the CLI does. An
editor that can send settings asks the strict question with the boolean
`sysml.strictConformance` ([LSP extensions](../reference/lsp.md#strict-conformance-setting)),
and the diagnostics of every open document are republished as errors at once. In
this extension, start the server strictly instead:

```json
{
  "opensysml.server.args": ["-strict"]
}
```

### The diagram panel

`SysML: Open Diagram`, from the command palette with a `.sysml` or `.kerml` file
open, puts a diagram of the model beside the editor. It draws the same
renderings the REPL's `%view` prints, as Mermaid, and it redraws as the model is
typed.

- **What it draws.** The view the document declares, picked from the dropdown
  when it declares several, or — the usual case for a model being written — the
  document itself, as a tree, an interconnection diagram, a state diagram, an
  action flow or a table. A view whose rendering is not supported (`sequence`,
  `geometry`, `textual`) stays in the picker, saying why it cannot be drawn.
- **Click a node** to jump to the declaration it was built from; move the cursor
  in the editor and the node containing it is highlighted. A node with no
  locatable declaration — a standard library symbol — is not clickable.
- **While typing.** A keystroke that leaves the model unparseable dims the last
  good diagram and puts the error in the status line under it; the panel never
  blanks. What the rendering could not represent is listed under the diagram.
- **Cost.** The panel asks for a diagram only while it is visible, and only once
  an editing burst settles. Mermaid is bundled into the extension, so nothing is
  fetched from the network.

It is read-only: it renders the model, and editing the diagram does not edit the
model. The panel appears only when the server it is talking to serves the render
methods ([LSP extensions](../reference/lsp.md)), so an older `sysml-lsp` simply
does not offer the command.

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
- ✅ Renderings, as the custom `opensysml/render` and `opensysml/views` requests
  and the `opensysml/renderChanged` notification, announced as
  `experimental: { openSysmlRender: true }` — what the diagram panel is built on,
  documented in [LSP extensions](../reference/lsp.md)
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
