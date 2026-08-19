# SysML v2 for VS Code (OpenSysML)

Syntax highlighting and language support for `.sysml` and `.kerml` files, backed by
OpenSysML's `sysml-lsp` server: diagnostics, hover, go-to-definition, document
symbols, typed completion, and a live diagram panel.

This extension is built and side-loaded from this repository. It is deliberately
**not published** to the Visual Studio Marketplace or Open VSX.

## Build and side-load

```bash
make build          # from the repo root: builds bin/sysml-lsp
cd editors/vscode
npm install
npm run package     # typecheck + bundle + opensysml-sysml.vsix
code --install-extension opensysml-sysml.vsix
```

Then open any `.sysml` file. The extension finds the server in this order:

1. `opensysml.server.path`, if set;
2. `bin/sysml-lsp` inside an open workspace folder (a repo checkout that ran `make build`);
3. `sysml-lsp` on `PATH`.

If none exist, highlighting still works and a warning explains how to build the
server. `SysML: Restart Language Server` restarts it after a rebuild.

## The diagram panel

`SysML: Open Diagram` opens a diagram of the active model beside it, drawn as
Mermaid from the server's rendering and redrawn as the model is typed.

| | |
| --- | --- |
| **What it draws** | The view the document declares, chosen in the picker when it declares several. A document declaring none is drawn directly, as a model tree, interconnection diagram, state diagram, action flow or element table — a table is written as Markdown rather than drawn, and is shown as that. A view whose rendering is not supported (`sequence`, `geometry`, `textual`) is listed but not drawable, with the reason as its tooltip. |
| **Navigation** | Click a node to open the declaration it was built from; moving the cursor in the editor highlights the node whose declaration contains it. A node with no locatable declaration, such as a standard library symbol, is inert. |
| **While typing** | A rendering that fails mid-keystroke leaves the last good diagram on screen, dimmed, with the error in the status line: the panel never blanks. What a rendering could not represent is listed under it. |
| **Cost** | The panel asks for a diagram only while visible, and only once an editing burst settles. Mermaid is bundled into the extension, and the panel's CSP allows the bundled script alone — nothing is fetched from the network. |

It is read-only — Tier 1 of the visual-modeling design: the diagram renders the
model, and cannot edit it, has no persisted layout, and offers no authoring.

The command exists only when the server advertises
`experimental: { openSysmlRender: true }`, so an older `sysml-lsp` keeps working
without it. The requests behind the panel — `opensysml/render`,
`opensysml/views` and the `opensysml/renderChanged` notification — are documented
in [docs/reference/lsp.md](../../docs/reference/lsp.md).

## Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `opensysml.server.path` | `""` | Absolute path to `sysml-lsp`; empty falls back to the workspace build, then `PATH`. |
| `opensysml.server.args` | `[]` | Extra server arguments. |
| `opensysml.server.enabled` | `true` | Set to `false` for highlighting without a server. |
| `opensysml.trace.server` | `"off"` | Trace LSP traffic in the "SysML v2" output channel. |

## Grammar generation

`syntaxes/*.tmLanguage.json` are generated — do not edit them by hand. The
keyword list comes from `internal/core/lexer.Keywords()`, so highlighting cannot
drift from the lexer:

```bash
make vscode-grammar    # regenerate
go test ./editors/...  # fails if the committed grammars are stale
```

## Development

```bash
npm run watch       # rebuild dist/extension.js and dist/webview.js on change
npm run typecheck   # tsc --noEmit, extension and webview
```

`esbuild.mjs` builds two bundles: the extension for Node, and the diagram
webview for the browser with Mermaid bundled in. They typecheck against
different libraries — the webview needs the DOM, the extension must not see it —
so `src/webview` has its own `tsconfig.json`.

Press <kbd>F5</kbd> in VS Code with `editors/vscode` open to launch an Extension
Development Host. `examples/demo.sysml` is a highlighting smoke-test file.
