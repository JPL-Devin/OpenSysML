# SysML v2 for VS Code (OpenSysML)

Syntax highlighting and language support for `.sysml` and `.kerml` files, backed by
OpenSysML's `sysml-lsp` server: diagnostics, hover, go-to-definition, document
symbols and typed completion.

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
npm run watch       # rebuild dist/extension.js on change
npm run typecheck   # tsc --noEmit
```

Press <kbd>F5</kbd> in VS Code with `editors/vscode` open to launch an Extension
Development Host. `examples/demo.sysml` is a highlighting smoke-test file.
