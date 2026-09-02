# Interfaces

The same pipeline is reachable from the command line, the REPL, the gRPC
service with its Python client, and the LSP server behind the VS Code
extension. Whichever door you use, a document renders identically.

## CLI

```console
$ sysml model.sysml -render-document Observatory::MassReport            # Markdown to stdout
$ sysml model.sysml -render-document Observatory::MassReport -o report.md
$ sysml model.sysml -render-document Observatory::MassReport \
    -doc-form html -html-css theme.css -o report.html
$ sysml model.sysml -render-documents site -doc-form html
$ sysml model.sysml -render-document Observatory::MassReport \
    -doc-form pdf -pdf-engine weasyprint -doc-title-page -doc-toc \
    -doc-number-sections -o report.pdf
$ sysml model.sysml -run-query "Observatory::SubsystemTable root=Observatory::telescope"
```

| Flag | Meaning |
|---|---|
| `-render-document <name>` | Render the named document definition |
| `-doc-form <form>` | `markdown` (default), `html` or `pdf` |
| `-pdf-engine <engine>` | `weasyprint` (default), `pandoc` or `prince` |
| `-doc-title-page` | Separate title page (HTML or PDF) |
| `-doc-toc` | Table of contents (HTML or PDF) |
| `-doc-number-sections` | Hierarchical section numbers (HTML or PDF) |
| `-html-css <file\|url>` | Style the HTML with this sheet, after the default one (repeatable) |
| `-html-no-default-css` | Leave the default stylesheet out |
| `-html-default-css` | Write the default stylesheet and exit |
| `-html-fragment` | The document element alone, to embed in your own page |
| `-run-query "<name> [<p>=<expr> ...]"` | Run one document query directly |
| `-o <file>` | Output file; required for PDF |

`-run-query` bindings are space-separated `parameter=expression` pairs after
the query's qualified name. A name expression binds the element it refers to;
quoted strings and numeric literals bind values. The exit code is non-zero
on any planning, binding or execution error. Full details are in the
[CLI reference](../reference/cli.md).

## REPL

Inside `sysml`'s interactive session, the same two operations are commands:

```
%run-query Observatory::SubsystemTable root=Observatory::telescope
%render-document Observatory::MassReport
```

`%render-document` prints Markdown; PDF output is CLI-only. See the
[REPL command reference](../reference/repl-commands.md).

## gRPC and Python

The `sysml-lsp -grpc` service exposes two document RPCs, advertised as the
`document_query` and `render_document` capabilities:

- **`RunDocumentQuery`** — run a named document query with typed bindings;
  the reply carries the projected columns and typed cell values in the
  engine's deterministic order.
- **`RenderDocument`** — render a named document definition; the reply
  carries the Markdown.

The Python client wraps both on its model handle:

```python
import opensysml
from opensysml.document import ElementRef

model = opensysml.load("observatory.sysml")

result = model.run_document_query(
    "Observatory::SubsystemTable",
    bindings={"root": ElementRef("Observatory::telescope")},
)
print(result.columns)          # ('name', 'mass')
for row in result:             # DocumentRow: row.element, cells by column index
    print(row[0], row[1])

markdown = model.render_document("Observatory::MassReport")
```

Bindings accept an `ElementRef` (a model element by qualified name), `str`,
`int`, `float`, `bool`, or a sequence of those. Query results decode the
service's typed values back into Python values, with unbounded multiplicity
as `opensysml.document.INFINITY`. Errors are typed exceptions
(`InvalidRequestError`, `SymbolNotFoundError`, `MissingCapabilityError`,
`ModelNotFoundError`). See the [API reference](../reference/api.md).

## VS Code and LSP

The LSP server gives editors document-generation support on top of the usual
diagnostics:

- **`opensysml/documents`** lists the document definitions the workspace
  declares (qualified name plus declaring file), which fills the extension's
  Render Document picker.
- **`opensysml/renderDocument`** renders one of them to Markdown through the
  same pipeline the CLI uses, against the same workspace model the
  diagnostics come from.
- **`opensysml/renderChanged`** notifies the client, debounced, when edits
  invalidate a rendering, so an open preview can refresh itself. Rendering
  is on demand — the server does not re-render while you type.

Authoring a document also benefits from the general language support:
planning mistakes (a missing title, an unknown query, a bad binding) surface
as diagnostics in the editor with the same messages the CLI prints. See the
[LSP reference](../reference/lsp.md).
