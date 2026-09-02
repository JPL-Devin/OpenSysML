# 8. Editors

`sysml-lsp` implements the Language Server Protocol over standard input and output, so any editor
with a generic LSP client can use it. The VS Code extension in
[editors/vscode](../../editors/vscode) also adds `.sysml` and `.kerml` syntax
highlighting.

## VS Code

The VS Code extension in [editors/vscode](../../editors/vscode) provides
syntax highlighting for `.sysml` and `.kerml` and an LSP client that launches
`sysml-lsp`. It is not published to any marketplace, so you build it and
side-load it:

```bash
make build                                    # builds bin/sysml-lsp
cd editors/vscode
npm install
npm run package                               # -> opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Opening any `.sysml` file highlights it immediately, and the extension starts the first server it
finds, looking in this order:

1. `opensysml.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If no server is found, highlighting still works and a warning explains how to build one. To
point the extension at a specific build, use `.vscode/settings.json`:

```json
{
  "opensysml.server.path": "/absolute/path/to/bin/sysml-lsp",
  "opensysml.trace.server": "messages"
}
```

### Strict conformance in the editor

As on the command line, the server reports OpenSysML's own notation extensions as warnings. An
editor that can send settings turns on strict conformance with the boolean
`sysml.strictConformance` ([LSP extensions](../reference/lsp.md#strict-conformance-setting)),
after which the diagnostics for every open document are republished as errors. With this
extension, start the server in strict mode instead:

```json
{
  "opensysml.server.args": ["-strict"]
}
```

### The diagram panel

Running `SysML: Open Diagram` from the command palette with a `.sysml` or `.kerml` file open
shows a diagram of the model beside the editor. The panel draws the same renderings the
REPL's `%view` command prints, using Mermaid, and redraws as you edit the model.

- **Content.** The panel draws a view the document declares (chosen from a dropdown when there
  are several) or, as is usual for a model under development, the document itself, rendered as a
  tree, an interconnection diagram, a state diagram, an action flow, a sequence diagram or a
  table. A view whose rendering is unsupported (`geometry`, `textual`) stays in the picker and
  explains why it cannot be drawn.
- **Navigation.** Clicking a node jumps to the declaration it was built from, and moving the
  cursor in the editor highlights the node that contains it. A node with no locatable declaration,
  such as a standard library symbol, is not clickable.
- **Behavior while editing.** A keystroke that leaves the model unparseable dims the last valid
  diagram and reports the error in the status line beneath it; the panel is never blanked.
  Anything the rendering could not represent is listed below the diagram.
- **Resource use.** The panel requests a diagram only while it is visible, and only after a
  burst of editing has settled. Mermaid is bundled with the extension, so nothing is fetched from
  the network.

The panel is read-only: it renders the model, and editing the diagram does not change the model.
It is only available when the connected server provides the render methods
([LSP extensions](../reference/lsp.md)), so an older `sysml-lsp` does not offer the command.

After rebuilding the binary, run `SysML: Restart Language Server` from the command palette.
`editors/vscode/README.md` documents every setting, the grammar generator (keywords are taken
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
- ✅ Code actions (`textDocument/codeAction`): quick fixes for the spelling of an
  unresolved name, importing the namespace that declares it, inserting a missing
  semicolon the parser located exactly; and the `refactor.rewrite` actions on a
  declaration's header that annotate it with a minted element id (an
  `IdentityMetadata::ElementId`, UUID v4, inline in its body or standalone at
  the end of the file) and bind an unbound root namespace to a project
  (`IdentityMetadata::ProjectRef` with a placeholder `projectId` to fill in), see
  [element identity](../project/element-identity-annotations.md)

**Not implemented:** semantic token deltas (`semanticTokens/full/delta`; the server keeps no
previous result to diff against, so clients re-request the full set), signature help, range
formatting, code lens and inlay hints. A client that requests one of these gets a
method-not-found response rather than a partial result. Quick fixes are offered only where the
repair is unambiguous: a syntax error that could be fixed with either a body or a semicolon gets
none.

**Testing the server:** the protocol is JSON-RPC over standard input and output, so you can send
requests by hand. The following exchange, run against `bin/sysml-lsp`, formats a badly indented
file and renames a definition:

```
→ textDocument/formatting  (file: "package P {\npart def Wheel {\nattribute diameter = 16.0;\n}\npart w : Wheel;\n}\n")
← [{"range": {"start": {"line": 0, "character": 0}, "end": {"line": 6, "character": 0}},
    "newText": "package P {\n    part def Wheel {\n        attribute diameter = 16.0;\n    }\n    part w : Wheel;\n}\n"}]

→ textDocument/rename      (position: line 1, character 10; newName: "Tyre")
← {"changes": {"file:///tmp/lsp-demo.sysml": [
      {"range": {"start": {"line": 1, "character": 9}, "end": {"line": 1, "character": 14}}, "newText": "Tyre"},
      {"range": {"start": {"line": 4, "character": 9}, "end": {"line": 4, "character": 14}}, "newText": "Tyre"}]}}
```

The rename edits the declaration and the `part w : Wheel` reference together, because it works
from name resolution rather than textual replacement.

To check the installation in an editor, open a file containing
`part Wheel { attribute diameter = 16.0; }` and hover over `Wheel`.

---

Next: [9. From your own program](09-python.md).
