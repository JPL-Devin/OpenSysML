# 8. Editors

`sysml-lsp` implements the Language Server Protocol over standard input and output, so any editor
with a generic LSP client can use it. The VS Code extension in
[editors/vscode](../../editors/vscode) additionally provides `.sysml` and `.kerml` syntax
highlighting.

## VS Code

This repository provides a VS Code extension in [editors/vscode](../../editors/vscode), offering
syntax highlighting for `.sysml` and `.kerml` together with an LSP client that launches
`sysml-lsp`. The extension is not published to any marketplace, so it must be built and
side-loaded:

```bash
make build                                    # builds bin/sysml-lsp
cd editors/vscode
npm install
npm run package                               # -> opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Opening any `.sysml` file highlights it immediately, and the extension starts the first server it
finds, searching in the following order:

1. `opensysml.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If no server is found, highlighting continues to work and a warning explains how to build one. To
direct the extension at a specific build, use `.vscode/settings.json`:

```json
{
  "opensysml.server.path": "/absolute/path/to/bin/sysml-lsp",
  "opensysml.trace.server": "messages"
}
```

### Strict conformance in the editor

As on the command line, the server reports OpenSysML's own notation extensions as warnings. An
editor that can send settings enables strict conformance with the boolean
`sysml.strictConformance` ([LSP extensions](../reference/lsp.md#strict-conformance-setting)),
after which the diagnostics of every open document are republished as errors. In this extension,
start the server in strict mode instead:

```json
{
  "opensysml.server.args": ["-strict"]
}
```

### The diagram panel

Running `SysML: Open Diagram` from the command palette with a `.sysml` or `.kerml` file open
displays a diagram of the model beside the editor. The panel draws the same renderings that the
REPL's `%view` command prints, using Mermaid, and redraws as the model is edited.

- **Content.** The panel draws the view the document declares, selected from a dropdown when the
  document declares several, or, as is usual for a model under development, the document itself,
  rendered as a tree, an interconnection diagram, a state diagram, an action flow, a sequence
  diagram or a table. A view whose rendering is unsupported (`geometry`, `textual`) remains in the
  picker and reports why it cannot be drawn.
- **Navigation.** Clicking a node jumps to the declaration it was built from, and moving the
  cursor in the editor highlights the node containing it. A node with no locatable declaration,
  such as a standard library symbol, is not clickable.
- **Behavior while editing.** A keystroke that leaves the model unparseable dims the last valid
  diagram and reports the error in the status line beneath it; the panel is never blanked.
  Anything the rendering could not represent is listed below the diagram.
- **Resource use.** The panel requests a diagram only while it is visible, and only after an
  editing burst has settled. Mermaid is bundled with the extension, so nothing is fetched from the
  network.

The panel is read-only: it renders the model, and editing the diagram does not modify the model.
It is available only when the server it is connected to provides the render methods
([LSP extensions](../reference/lsp.md)), so an older `sysml-lsp` does not offer the command.

After rebuilding the binary, run `SysML: Restart Language Server` from the command palette. The
file `editors/vscode/README.md` documents every setting, the grammar generator (keywords are taken
from `internal/core/lexer.Keywords()`, so they cannot drift) and the <kbd>F5</kbd>
extension-debugging loop.

Other editors can launch `bin/sysml-lsp` over standard input and output through their own generic
LSP client; only the syntax highlighting is specific to VS Code.

**Capabilities advertised at `initialize`**, recorded from a live session with `bin/sysml-lsp`:

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

**Not implemented:** semantic token deltas (`semanticTokens/full/delta`; the server retains no
previous result to diff against, so clients re-request the full set), signature help, range
formatting, code lens and inlay hints. A client that requests one of these receives a
method-not-found response rather than a partial result. Quick fixes are offered only where the
repair is unambiguous: a syntax error that could require either a body or a semicolon carries
none.

**Testing the server:** the protocol is JSON-RPC over standard input and output, so requests can
be sent manually. The following exchange formats a badly indented file and renames a definition,
run against `bin/sysml-lsp`:

```
→ textDocument/formatting  (file: "package P {\npart def Wheel {\nattribute diameter = 16.0;\n}\npart w : Wheel;\n}\n")
← [{"range": {"start": {"line": 0, "character": 0}, "end": {"line": 6, "character": 0}},
    "newText": "package P {\n    part def Wheel {\n        attribute diameter = 16.0;\n    }\n    part w : Wheel;\n}\n"}]

→ textDocument/rename      (position: line 1, character 10; newName: "Tyre")
← {"changes": {"file:///tmp/lsp-demo.sysml": [
      {"range": {"start": {"line": 1, "character": 9}, "end": {"line": 1, "character": 14}}, "newText": "Tyre"},
      {"range": {"start": {"line": 4, "character": 9}, "end": {"line": 4, "character": 14}}, "newText": "Tyre"}]}}
```

The rename edits the declaration and the `part w : Wheel` reference together, because it is
resolution-driven rather than a textual replacement.

To verify the installation in an editor, open a file containing
`part Wheel { attribute diameter = 16.0; }` and hover over `Wheel`.

---

Next: [9. From your own program](09-python.md).
