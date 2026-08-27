# LSP extensions

`sysml-lsp` speaks the Language Server Protocol, plus the three methods on this
page. They are not in the protocol, so a client must ask for them by name; the
server says it serves them by advertising, in the `initialize` result:

```json
{ "capabilities": { "experimental": { "openSysmlRender": true } } }
```

A client that does not see that capability must not send these methods — that is
how a new client and an older server stay compatible.

Everything here is read-only: it renders what a document says, and never writes
it. The renderings are the ones [`%view`](repl-commands.md) and `sysml -view`
produce, from the same renderer.

## Strict conformance (setting)

Whether the server judges a document as conforming SysML v2 — reporting notation
only OpenSysML accepts as an error instead of a warning — is a boolean setting,
`strictConformance`. It is read from `initialize`'s `initializationOptions` and
from `workspace/didChangeConfiguration`, in any of the three shapes clients nest
settings in:

```json
{ "strictConformance": true }
{ "sysml": { "strictConformance": true } }
{ "sysml.strictConformance": true }
```

A payload that does not mention it leaves the mode alone, and a value that is not
a boolean is ignored rather than read as either answer. Changing it republishes
the diagnostics of every open document, so the editor never keeps the other
mode's verdict. A client that cannot send settings can start the server with
`-strict` instead; both are the CLI's `-strict` and the REPL's `%strict`, and
[the guide](../guide/03-command-line.md#strict-conformance) says what the mode
changes.

## `opensysml/render` (request)

Renders one view of a document.

```json
{
  "textDocument": { "uri": "file:///tmp/kit.sysml" },
  "view": "KitViews::widgetTree",
  "form": "mermaid"
}
```

| Field | Meaning |
| --- | --- |
| `textDocument.uri` | The document to render. It must be one the session holds — an open document, or a workspace file the server read. |
| `view` | The qualified name of a view the document declares, a pseudo-view (below), or omitted. |
| `form` | `mermaid`, `text` or `markdown`. Omitted writes the machine form of the rendering's kind: `markdown` for a table, `mermaid` for every other kind. |

Omitting `view` renders the view the document declares. A document declaring
several is ambiguous, and the request fails naming them
(`declares 6 views (KitViews::widgetActions, …); name the one to render`) rather
than picking one; a document declaring none fails pointing at the pseudo-views.

A `form` a rendering is not written in — Mermaid for a table, Markdown for a
diagram — is refused with the form the kind is written in instead, and a form
that is no form at all is refused naming the three.

**Pseudo-views.** A document being written usually declares no `view`, so a
rendering can be asked for as if one had been declared:

| `view` | Renders |
| --- | --- |
| `#tree` | Everything the document declares, as a tree |
| `#interconnection` | …as an interconnection diagram |
| `#state` | …as a state diagram |
| `#action` | …as an action flow |
| `#table` | …as an element table |
| `#state:Kit::WidgetStates` | One element the document declares, here as a state diagram |

A pseudo-view adds nothing to the model and nothing to the symbol index: the
exposed set is passed to the renderer directly, and the result reports it in
`stated`, as `no view declared; rendering Kit::WidgetStates directly`.

The result, for `{"view": "KitViews::widgetTree"}` over a document declaring
`part def Widget { part cog : Cog; part gear : Cog; connect cog to gear; }`:

```json
{
  "view": "KitViews::widgetTree",
  "kind": "tree",
  "stated": "",
  "form": "mermaid",
  "artifact": "%% KitViews::widgetTree — tree rendering\nflowchart TD\n  n0[\"part def Kit::Widget\"]\n  n1[\"part cog (Cog)\"]\n  n0 --- n1\n  …",
  "nodes": [
    {
      "id": "n0",
      "kind": "part def",
      "name": "Kit::Widget",
      "detail": "",
      "origin": {
        "uri": "file:///tmp/kit.sysml",
        "range": { "start": { "line": 1, "character": 1 }, "end": { "line": 6, "character": 1 } },
        "selectionRange": { "start": { "line": 1, "character": 10 }, "end": { "line": 1, "character": 16 } }
      }
    },
    {
      "id": "n1",
      "kind": "part",
      "name": "cog",
      "detail": "Cog",
      "parent": "n0",
      "origin": { "uri": "file:///tmp/kit.sysml", "range": { "…": "…" } }
    }
  ],
  "edges": [{ "from": "n0", "to": "n1", "label": "", "kind": "connection" }],
  "notices": [],
  "version": 7
}
```

| Field | Meaning |
| --- | --- |
| `view` | The view rendered, by qualified name; empty for a pseudo-view. |
| `kind` | `tree`, `interconnection`, `state`, `action`, `sequence` or `table`. |
| `stated` | How the kind was decided — the rendering the view names, the standard view definition it specializes, or that no view was declared. Empty when the view took the default. |
| `artifact` | What to draw or show: a Mermaid diagram, the text form, or a Markdown table. |
| `nodes`, `edges` | What the artifact is made of, so a client can map a click on it back to the source. A node's `parent` is the node containing it, when one does. An edge's `kind` is `connection`, `transition`, `succession` or `flow`. |
| `rows`, `columns` | A table rendering's cells, in place of nodes and edges. |
| `origin` | Where the element was declared, as a document URI, the `range` of the whole declaration and, when the declaration names one, the `selectionRange` of the identifier alone. A client highlights the element whose `range` holds the cursor and navigates to its `selectionRange`, as `textDocument/definition` does. Absent for an element with no locatable declaration: a standard library symbol the index served from its cache, or a step a lowering sequenced without a declaration of its own, carries none rather than a bogus range. |
| `notices` | What the rendering could not represent, as the text form reports it. |
| `version` | The version of the document the rendering was made from, so a client can tell a rendering of the text it is showing from a stale one. |

A view stating a rendering this implementation does not produce — `geometry`,
`textual` — fails with the reason, e.g.
`KitViews::widgetGeometry: geometry rendering (view def GeometryView) is not supported`.

## `opensysml/views` (request)

Lists the views a document declares, which is what fills a diagram panel's view
picker.

```json
{ "textDocument": { "uri": "file:///tmp/kit.sysml" } }
```

```json
{
  "views": [
    { "name": "KitViews::widgetParts", "kind": "interconnection", "supported": true },
    { "name": "KitViews::widgetSequence", "kind": "sequence", "supported": true },
    {
      "name": "KitViews::widgetGeometry",
      "kind": "geometry",
      "supported": false,
      "reason": "KitViews::widgetGeometry: geometry rendering (view def GeometryView) is not supported"
    }
  ]
}
```

Views come in qualified-name order. An unsupported one stays in the listing,
with `supported: false` and the reason, so a client can say why it cannot be
drawn instead of hiding it.

## `opensysml/renderChanged` (notification, server → client)

```json
{ "textDocument": { "uri": "file:///tmp/kit.sysml" }, "version": 8 }
```

Sent after the analysis that publishes the document's diagnostics, so a client
sees the diagnostics of a version before the notification for it. It is
debounced on the window the cross-document diagnostics sweep uses, so a burst of
keystrokes costs one notification rather than one per keystroke.

It carries no artifact: the client answers with a fresh `opensysml/render` if it
is showing the document, and does nothing if it is not — which keeps a large
diagram off the wire for a panel nobody is looking at.

## Trying it by hand

The protocol is JSON-RPC over stdio, so the methods can be driven without an
editor — `initialize`, `textDocument/didOpen`, then:

```
→ opensysml/render  { "textDocument": { "uri": "file:///tmp/kit.sysml" }, "view": "#tree" }
← { "view": "", "kind": "tree",
    "stated": "no view declared; rendering /tmp/kit.sysml directly",
    "form": "mermaid",
    "artifact": "%%  — tree rendering (no view declared; …)\nflowchart TD\n  n0[\"part def Kit::Widget\"]\n…",
    "nodes": [ … ], "edges": [ … ], "notices": [], "version": 1 }
```

The VS Code extension's diagram panel is the reference client — see
[the editors guide](../guide/08-editors.md#the-diagram-panel).
