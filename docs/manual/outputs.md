# Outputs

## Markdown

Markdown is the primary output form and the default:

```console
$ sysml report.sysml -render-document Observatory::MassReport            # stdout
$ sysml report.sysml -render-document Observatory::MassReport -o report.md
```

What the renderer emits:

- The document title as a level-1 ATX heading; each section one level deeper,
  saturating at level 6.
- Paragraphs as prose, with inline runs styled (`*emphasis*`, `**strong**`,
  `` `code` ``, and links as `[text]` followed by the angle-bracketed URL in
  parentheses).
- A stable `<a id="..."></a>` anchor before every block that a `Ref` targets;
  the reference renders as a link to it.
- Tables as GitHub-flavored pipe tables; grouped tables as one subtable per
  group key.
- Lists as `-` bullets or `1.` numbered items.
- Diagrams as fenced ` ```mermaid ` blocks (or a pipe table for the `table`
  kind), with captions in emphasis.
- All model-derived text escaped so it cannot break document structure.
- A single trailing newline, no trailing whitespace.

The output is deterministic: the same model produces byte-identical Markdown
on every run, which is why the repository can keep rendered documents as
golden files (this manual does exactly that — see
[the worked example](worked-example.md)).

### Multi-document sets

`-render-documents <dir>` renders every document definition the model
declares into the directory, one Markdown file per document:

```console
$ sysml reports.sysml -render-documents rendered
```

File names are deterministic — the document's fully qualified name with `::`
replaced by `-` and any byte outside ASCII letters, digits and `_` escaped as
`.XX` (uppercase hex), plus `.md` — so cross-document references (see
[the authoring chapter](authoring.md)) resolve as relative links between the
written files, and repeated runs write identical bytes. A single-document
render of a cross-referencing document still succeeds; its external links
point at the targets' expected file names and dangle until those documents
are rendered into the same directory.

## PDF

`-doc-form pdf` renders the same document tree to PDF. It requires `-o`
(PDFs are not written to stdout):

```console
$ sysml report.sysml -render-document Observatory::MassReport \
    -doc-form pdf -o report.pdf
```

Internally the engine renders Markdown, converts it to styled HTML, renders
any Mermaid diagrams to SVG with Mermaid CLI (`mmdc`), and hands the result
to an external HTML-to-PDF converter.

### Engines

`-pdf-engine` selects the converter (default `weasyprint`):

| Engine | Tool invoked | Notes |
|---|---|---|
| `weasyprint` | `weasyprint`, an HTML-to-PDF paged-media engine | Default; open source |
| `pandoc` | `pandoc` reading the Markdown itself, with WeasyPrint as its PDF engine | |
| `prince` | `prince`, a commercial HTML-to-PDF engine | License required |

The converters are external tools, not bundled with the binary. If the
selected tool is not on `PATH`, the render fails with a typed `tool-missing`
error naming it. Environment variables override discovery:
`OPENSYSML_WEASYPRINT`, `OPENSYSML_PANDOC`, `OPENSYSML_PRINCE`,
`OPENSYSML_MMDC`, and `OPENSYSML_MMDC_PUPPETEER` (extra Puppeteer
configuration for Mermaid CLI). The repository's
`scripts/download-doc-pdf-toolchain.sh` fetches a pinned WeasyPrint, pandoc
and Mermaid CLI and prints the exports to use them.

### PDF options

| Flag | Effect |
|---|---|
| `-pdf-title-page` | A separate title page before the content |
| `-pdf-toc` | A table of contents built from the section headings |
| `-pdf-number-sections` | Hierarchical section numbers (1, 1.1, ...) |

All three are off by default and only valid with `-doc-form pdf`.

### PDF rendering of inline runs and anchors

Inline runs and cross-reference anchors render semantically in PDF. A
paragraph built from `Span`/`Link`/`Ref` runs renders with emphasis, strong
and code styling and working links; a `Ref` anchor becomes an invisible
PDF-native anchor, so an in-document `Ref` is a clickable internal link; and
a grouped table's group key renders in strong type above each subtable. All
three engines support internal links: `weasyprint` and `prince` from the
prepared HTML's element ids and fragment hrefs, and `pandoc` from the
Markdown itself, whose CommonMark reader keeps the anchor's raw HTML.

## Determinism

**Markdown** is fully deterministic: byte-identical output for the same
model and binary. Query results preserve declaration order unless ordered
explicitly, ordering policies are explicit parameters, and rendering
introduces no timestamps, random identifiers or map-order dependence.

**PDF** is deterministic *against a pinned toolchain*. The engine does its
part — it invokes converters with `SOURCE_DATE_EPOCH=0` so they embed a fixed
creation date — but the bytes also depend on the converter version and the
fonts installed on the machine. Two runs on the same machine with the same
toolchain produce identical PDFs; two machines with different WeasyPrint
versions or fonts generally do not. For reproducible PDFs, pin the toolchain
(the download script above is how CI does it).
