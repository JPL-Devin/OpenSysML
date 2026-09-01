# SysML CLI Usage Examples

## OSLC element queries

`-query <oslc-query>` loads the positional model and evaluates OSLC Query text,
printing one matched element per line as qualified name and metamodel type,
followed by any selected properties. It is a model-query mode alongside
`-convert`.

```bash
sysml -query 'oslc.where=sysml:name="wheel"' model.sysml
```

## Interactive Mode (Default)

Start the REPL with no arguments:

```bash
sysml
```

Load files and enter interactive mode:

```bash
sysml model.sysml
sysml types.sysml instances.sysml
```

## Non-Interactive Mode

Execute expressions and exit without entering interactive mode.

### Basic Evaluation

Evaluate an expression:

```bash
sysml -e "5 + 3"
# Output: ✓ 5 + 3
#           = 8
```

### Load Files and Evaluate

Load a model first, then evaluate:

```bash
sysml -e "someAttribute" model.sysml
```

**Note:** flags may be written before or after the files — `sysml model.sysml -e "x"`
and `sysml -e "x" model.sysml` do the same thing.

### Multiple Evaluations

Evaluate multiple expressions in sequence:

```bash
sysml -e "x" -e "y" -e "z" model.sysml
```

### Multiple Files

Load multiple files before evaluating:

```bash
sysml -e "result" types.sysml instances.sysml
```

## Real-World Examples

### 1. Quick Calculation

```bash
sysml -e "10 * 2 + 5"
# Output: ✓ 10 * 2 + 5
#           = 25
```

### 2. Validate Model and Check Constraint

```bash
sysml -e "speedLimit < 120" vehicle-model.sysml
```

### 3. Extract Calculated Values

```bash
# model.sysml contains: attribute totalCost = partCost + laborCost;
sysml -e "totalCost" model.sysml
# Output: ✓ totalCost
#           = 1500.0
```

### 4. Batch Processing

```bash
#!/bin/bash
for model in models/*.sysml; do
    result=$(sysml -e "result" "$model" 2>&1 | grep "=" | awk '{print $2}')
    echo "$model: $result"
done
```

### 5. CI/CD Integration

A pipeline gates on the exit status: an expression that could not be evaluated
exits `2`, and only a value on stdout is left to compare (see
[Exit status](#exit-status)):

```bash
# Check that a calculated value matches what is expected
expected=42
actual=$(sysml -e "designParameter" design.sysml | awk '/^ *=/ {print $2}') || exit $?
if [ "$actual" = "$expected" ]; then
    echo "✓ Design parameter validated"
else
    echo "✗ Design parameter mismatch: expected $expected, got $actual" >&2
    exit 1
fi
```

Constraint, requirement and satisfy verdicts gate the same way, and report
themselves:

```bash
sysml -satisfy -constraint MassBudget design.sysml   # 0 held, 1 answered false, 2 undecided
```

### 6. Use REPL Meta Commands

Load a file and use meta commands:

```bash
echo "%load model.sysml
%instantiate Vehicle
%features Vehicle
%eval speedLimit" | sysml
```

## Command Reference

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--eval <expr>` | `-e` | Evaluate expression and exit (repeatable) |
| `--debug` | | Report every diagnostic over the whole session buffer, with the pass that produced it |
| `--quiet` | | Report errors only, suppressing warnings |
| `--strict` | | Judge the model as conforming SysML v2: notation no pinned production admits is an error, not a warning (see [Strict conformance](../guide/03-command-line.md#strict-conformance)) |
| `--trace` | | Report each execution step: expression evaluation, calc invocation, action tokens, state transitions |
| `--convert <format>` | | Convert the model instead of running it: `sysml`, `kerml`, `ttl`, `turtle` or `rdf`. RDF is [experimental](rdf-mapping.md#status-experimental) and every run that converts it says so on stderr (see [the RDF mapping](rdf-mapping.md)) |
| `--from <format>` | | Input format for `--convert` (default: from the input's extension) |
| `--render <view>` | | Render this view of the model instead of running it, in the form its `render` member states (see [Rendering a view](#rendering-a-view)) |
| `--render-all <dir>` | | Render every declared view into the directory, one artifact per view |
| `--render-form <form>` | | Form `--render` or `--render-all` writes: `text`, `mermaid` or `markdown` (default: destination-dependent for `--render`, each kind's machine-readable form for `--render-all`) |
| `--render-document <name>` | | Compile a document definition — a `part def` specializing `DocumentQueries::Document` — run its queries against the model, render its diagram blocks through the view engine and write the rendered CommonMark Markdown, as `%render-document` does. Paragraphs compose statically-authored inline runs (`Span` with a `plain`/`emphasis`/`strong`/`code` style, `Link` to a URL, `Ref` linking to another content block's anchor), a query-backed paragraph or list styles its projected values through nested `SpanColumn`/`LinkColumn` column runs, and a table with a `groupBy` column writes one subtable per group value; table columns are the query's projected properties and computed `Column` names. A `Diagram` content block embeds a declared view, or an element with a stated rendering kind, as a fenced ` ```mermaid ` block (a table-kind view as a pipe table), with an optional caption and `TB`/`LR`/`RL`/`BT` flow direction. Markdown is the default document form, `-doc-form pdf` converts it (see [Rendering a document as PDF](#rendering-a-document-as-pdf)), and `-json` does not apply |
| `--doc-form <form>` | | Form `--render-document` writes: `markdown` (default) or `pdf`, which drives an external converter |
| `--render-documents <dir>` | | Render every document definition the model declares as a linked Markdown set into the directory, one file per document, so cross-document references resolve on disk |
| `--pdf-engine <engine>` | | Converter `--doc-form pdf` drives: `weasyprint` (default), `pandoc` or `prince` |
| `--pdf-title-page` | | Put the document title on a page of its own (`--doc-form pdf`) |
| `--pdf-toc` | | Write a table of contents ahead of the content (`--doc-form pdf`) |
| `--pdf-number-sections` | | Number the section headings hierarchically (`--doc-form pdf`) |
| `--output <file>` | `-o` | Write the conversion, the rendering or the rendered document to a file instead of stdout |
| `--version` | `-v` | Show version information |
| `--help` | `-h` | Show usage information |

Check flags, each repeatable. `-instantiate` runs first whatever order they are
written in, so the verdicts after it are about that object:

| Flag | Checks |
|------|--------|
| `-validate` | Nothing about the model's conditions: only that it analyses cleanly, and that the objects `-instantiate` asked for materialized |
| `-constraint <name>` | One constraint, as `%constraint` does |
| `-requirement <name>` | One requirement, as `%requirement` does |
| `-satisfy` | Every satisfaction assertion the model states |
| `-satisfy=<name>` | Only the assertions the named element states (`-satisfy=false` asks for none) |
| `-instantiate <name>` | Creates an object first, so the verdicts are about it |
| `-calc "<name>(<args>)"` | Invokes a calculation and reports what it computed |
| `-run-query "<name> [<p>=<expr>...]"` | Executes a document query and reports its rows, as `%run-query` does — including any computed `Column(name = "<column>", expression = <expr>)` projections evaluated per row. Each binding is written as `<parameter>=<expression>` |
| `-action "<name> [object]"` | Runs an action to completion and reports its outputs |
| `-state "<name> [object]"` | Runs a state machine and reports where it settled |
| `-advance <time>` | Simulated time units each `-state` machine is run for |
| `-json` | Reports the checks as one JSON document rather than as lines |

**Arguments:**
- `[file...]` - SysML files to load (loaded in order)

**Usage pattern:**
```
sysml [options] [file...]
```

Flags may be written before or after the files. A file named like a flag is read
as a file after `--`, which ends the flags: `sysml -trace -- -m.sysml`.

## Examples

```bash
# Interactive REPL
sysml

# Load file and start REPL
sysml model.sysml

# Evaluate and exit
sysml -e "5 + 3"

# Load file, evaluate, and exit
sysml -e "expr" file.sysml

# Multiple evaluations
sysml -e "x" -e "y" file.sysml

# Multiple files
sysml -e "result" file1.sysml file2.sysml
```

## Rendering a view

`-render <view>` renders one view of the model and exits. The rendering is the one the view's
`render` member states, and a containment tree where it states none; the kinds this build produces
are a tree, an interconnection diagram, a state machine, an action flow, a sequence diagram and a
table. A geometry view is recognized but not drawn. A pseudo-view renders without a declaration:
`#tree` renders the one model file accepted by `-render` (or every document loaded in the REPL),
while `#tree:<name>`, `#interconnection:<name>`, `#state:<name>`, `#action:<name>`,
`#sequence:<name>` and `#table:<name>` render the named element directly. Only kinds this build
produces are offered; newly supported kinds become pseudo-views automatically.

```bash
# The ASCII text form a person reads, written to fit the terminal
sysml model.sysml -render Views::vehicleView

# The machine-readable form of the kind: piped, redirected or written to a file
sysml model.sysml -render Views::vehicleView | tee view.mmd
sysml model.sysml -render Views::partsTable > parts.md
sysml model.sysml -render Views::vehicleView -o view.mmd

# Either form, whatever the destination
sysml model.sysml -render Views::partsTable -render-form markdown
sysml model.sysml -render Views::vehicleView -render-form text

# Render a named element, or one model directly, without declaring a view
sysml model.sysml -render '#state:Vehicle::controller'
sysml model.sysml -render '#tree'
```

Where `-render-form` names no form, the form follows the destination: the text form at a terminal,
where a person reads it, and the machine-readable form of the kind into a file or a pipe, where a
tool does. The text form is ASCII and its table is written to fit the terminal, wrapping a cell
wider than its column rather than truncating it; into a file or a pipe every column is as wide as
its widest cell, so a saved artifact does not depend on the window it was written from.

The artifact is the run's result, so it goes on stdout alone — what was loaded, what the model
analysed to, an empty rendering, and any element the rendering cannot represent all go on stderr,
and `-o` writes the artifact only. A view exposing nothing renders an empty artifact and says so; a
name that is no view, a rendering kind this build does not produce, a form the kind is not written
in, and a model that did not analyse cleanly each stop the run with status 2. Rendering decides
nothing about the model, so it is not asked for together with a check flag or with `-convert`.

`-render-all <dir>` writes every declared view of all loaded files, in document and declaration
order. Each qualified view name becomes a file name with `::` replaced by `.`. With no
`-render-form`, graph-shaped kinds use Mermaid (`.mmd`) and tables use Markdown (`.md`); a forced
text form uses `.txt` and unbounded width.

```bash
sysml types.sysml model.sysml -render-all rendered
sysml model.sysml -render-all rendered-text -render-form text
```

The directory is created when necessary. Written paths, load reports, and notices prefixed by
their view go to stderr; stdout stays empty. An unsupported rendering kind, or a forced form the
kind cannot write, is reported and skipped without failing the run. No declared views or an
analysis error stops the run with status 2. `-render-all` cannot be combined with `-render`, `-o`,
`-convert`, or a check flag.

The rendering is **tool-defined output**: SysML v2 §10.2 specifies the notation a view is written
in, not how a tool draws it. Mermaid is the machine-readable form of the graph-shaped kinds because
it renders as-is in Markdown, documentation sites and editors without a separate rendering tool, and
has dedicated state diagram and sequence diagram grammars; a table is written as a Markdown table,
which Mermaid has no grammar for, so `-render-form mermaid` of a table names Markdown rather than
drawing a diagram of rows.

`-render-documents <dir>` renders every document definition the loaded model declares into the
directory, one Markdown file per document, in fully-qualified-name order. Each file name is the
document's fully qualified name with `::` replaced by `-` and any byte outside ASCII letters,
digits and `_` escaped as `.XX` (uppercase hex), plus `.md` — deterministic, so cross-document
references (see [the authoring chapter](../manual/authoring.md)) resolve as relative links between
the written files. Repeated runs write identical bytes.

```bash
sysml model.sysml -render-documents rendered
```

The directory is created when necessary; written paths go to stderr and stdout stays empty. A
model that declares no documents, more than one document with the same name, or does not analyse
cleanly stops the run with status 2. `-render-documents` cannot be combined with
`-render-document`, `-render`, `-render-all`, `-o`, `-convert`, a query flag, or a check flag.
A single `-render-document` of a document with cross-document references still succeeds: the links
point at the targets' expected file names and dangle until those documents are rendered into the
same directory.

`-render-document` takes as many model files as the document needs, loaded as one model, so a
document may query elements its siblings declare:

```bash
sysml model/*.sysml -render-document Reports::MassReport -o report.md
```

## Rendering a document as PDF

`-render-document <name> -doc-form pdf -o report.pdf` converts the rendered Markdown to PDF. The
conversion never runs inside the `sysml` binary: it drives an external converter as a subprocess,
selected with `-pdf-engine`, so the binary links no PDF renderer and Markdown output needs none of
these tools.

```bash
sysml model.sysml -render-document Reports::MassReport -doc-form pdf -o report.pdf
sysml model.sysml -render-document Reports::MassReport -doc-form pdf \
    -pdf-engine pandoc -pdf-title-page -pdf-toc -pdf-number-sections -o report.pdf
```

The engines, each discovered on `PATH` by its default name unless an environment variable points
at a specific executable:

| Engine | Tools it drives | Override |
|--------|-----------------|----------|
| `weasyprint` (default) | `weasyprint`, an HTML-to-PDF paged-media engine | `OPENSYSML_WEASYPRINT` |
| `pandoc` | `pandoc` reading the Markdown itself, with WeasyPrint as its PDF engine | `OPENSYSML_PANDOC` (and `OPENSYSML_WEASYPRINT`) |
| `prince` | `prince`, a commercial HTML-to-PDF engine | `OPENSYSML_PRINCE` |

The title page, table of contents and section numbering are choices of this output step alone —
they are flags of the run, never attributes of the document model, so the same document renders to
Markdown unchanged.

Diagram blocks are pre-rendered to SVG with [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)
(`mmdc`, override `OPENSYSML_MMDC`; `OPENSYSML_MMDC_PUPPETEER` names a puppeteer configuration file
for a browser that needs launch flags, such as `--no-sandbox` in a container). A document without
diagrams needs no diagram tool.

Inline runs render semantically in PDF: emphasis, strong and code styling, links, and `Ref`
cross-references as clickable internal links to their targets' invisible anchors, in every engine —
`weasyprint` and `prince` through the prepared HTML, `pandoc` through the Markdown itself. A grouped
table's group key renders in strong type above each subtable.

A PDF is a binary artifact, so `-doc-form pdf` requires `-o`. A missing tool stops the run with
status 2 and a report naming the tool, its override variable and the other engines; a converter
that fails reports its own words. `scripts/download-doc-pdf-toolchain.sh` provisions pinned copies
of WeasyPrint, pandoc and mermaid-cli under `build/doc-pdf/` and prints the variables to export
(Prince is commercial and installed separately). Every tool is run with `SOURCE_DATE_EPOCH=0`, so
an engine that embeds a creation date embeds the same one every run and the artifact is
reproducible against one toolchain.

## Output Format

All evaluations include checkmark and result:

```bash
$ sysml -e "10 * 2"
✓ 10 * 2
  = 20
```

For file loads, declarations are summarized:

```bash
$ sysml demo.sysml
package Demo
  part def Vehicle {
    attribute speed : Real;
  }
SysML v2 REPL — %help for commands, Ctrl-D to exit
sysml> 
```

## Error Handling

**On every run that is not a prompt, what was asked for goes to stdout and what
went wrong goes to stderr.** Evaluated values, conversion output, verdict lines
and the `✓` echoes of what a load declared are results; model diagnostics —
errors and warnings alike — and anything that stopped the run are findings, and
so is the `wrote <file> …` note a `-convert -o` prints when it succeeds, which is
kept off stdout so a conversion can be piped.

A file that cannot be read ends the run:

```bash
$ sysml missing.sysml
sysml: cannot read missing.sysml: no such file or directory
$ echo $?
2
```

A model that does not analyse cleanly answers nothing, so its diagnostics end the
run rather than an evaluation being reported against a model nobody could read:

```bash
$ sysml -e "1+1" bad.sysml 2>/dev/null
$ echo $?
2
$ sysml -e "1+1" bad.sysml 2>&1 >/dev/null
bad.sysml:2:39: error: expected an expression
  part def Vehicle { attribute mass = ; }
                                      ^
sysml: bad.sysml did not analyse cleanly
```

An expression that cannot be evaluated is reported the same way, leaving what the
load declared on stdout:

```bash
$ sysml -e "Demo::Vehicle::nope" model.sysml
✓ package Demo
sysml: unresolved reference: Demo::Vehicle::nope
$ echo $?
2
```

So `2> errors.log` collects everything a script would otherwise have to read the
results for, plus the `wrote …` note of a successful `-convert -o` and any
warning the model raised — neither of which changes the status, so a non-empty
log is not by itself a failure. The status is.

## Exit status

The whole contract, which is the same whatever the run was asked to do. This is
the one place it is written down; [the guide](../guide/)
links here.

| Status | Means |
|--------|-------|
| `0` | What was asked for was done: every file loaded and analysed cleanly, every `-e` expression produced a value, every check held, a conversion was written. Warnings leave the status `0`. |
| `1` | The model answered false: a constraint, requirement or satisfaction assertion the model decided did not hold. Only a verdict reports this status. |
| `2` | What was asked for could not be done, so the model answered nothing: a file that could not be read, a model that did not analyse cleanly, an object whose feature values did not materialize, an unresolved name, a check that could not be made, a conversion that could not be written because the RDF graph cannot rebuild a source construct, a misused flag or an invalid `OPENSYSML_MAX_*` value. |

```bash
$ printf '%s\n' 'constraint MassBudget { 1 > 2 }' > model.sysml
$ sysml -constraint MassBudget model.sysml; echo $?      # a verdict the model decided
✓ constraint MassBudget
✗ Constraint MassBudget failed
  Assertion evaluated to false: 1 > 2
1

$ sysml -debug -quiet model.sysml; echo $?
sysml: -debug and -quiet are mutually exclusive
2

$ OPENSYSML_MAX_STEPS=abc sysml -e "1+1"; echo $?
sysml: OPENSYSML_MAX_STEPS="abc" is not an integer: set it to a positive number of evaluation steps (default 10000000)
2

$ sysml examples/parser_features_demo_advanced_bodies.kerml -convert ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
sysml: cannot convert the operator expr at examples/parser_features_demo_advanced_bodies.kerml:87:9: save to .sysml or .kerml instead, which writes the source exactly; see docs/reference/rdf-mapping.md § Limitations
2

$ sysml examples/state-machine-demo.sysml -convert ttl -o /tmp/state-machine.ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/state-machine.ttl (ttl, 2078 bytes)
0
```

Materializing an object is part of the run, so what it finds is a diagnostic
about the model: `-instantiate` reports every feature value it could not materialize —
a default whose value count does not conform to the multiplicity governing its
feature, which is the assumed `1..1` for a feature that declares none — and
`-validate` reports `no errors` only for a run that found none. The prompt surface
follows the same rule: a command that rendered a feature value it could not materialize —
a `%features` listing carrying `<error: …>`, or an `%eval` of such a value, pinned to
a context (`%eval in <name> : <expr>`) or not — answered nothing about it, so a
session driven from a pipe exits `2` rather than reporting success, whatever
analysis found. A name that is no feature of the object is a request the command got
wrong, not a feature value that failed to materialize, and does not change the status.

```bash
$ cat > model.sysml <<'EOF'
package test {
  part def Sub;
  part def Craft {
    part left : Sub;
    part right : Sub;
    part volumes : Sub = (left, right);
  }
  part craft : Craft;
}
EOF
$ sysml model.sysml -instantiate test::craft -validate; echo $?
✓ package test
✓ Created instance of test::craft
  ID: 1
  Use %features test::craft to inspect
error: feature value craft.volumes: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1
sysml: model.sysml did not materialize cleanly
2

$ printf '%%instantiate test::craft\n%%features test::craft\n' | sysml model.sysml; echo $?
✓ package test
Instance: test::craft (ID: 1)
Features:
  left = Instance(ID: 2)
    (no features)
  right = Instance(ID: 3)
    (no features)
  volumes: <error: feature value craft.volumes: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1>
2
```

Nesting multiplies, and reading a feature value materializes the objects it holds, so the
check is bounded, as the `%features` listing is: a model wide enough to spend that
budget, deeper than the walk descends, or one whose part holds its own kind is
reported as checked in part (`warning: … materialization is bounded; not every
feature value was checked`, and `no errors in the feature values checked`) rather than read to the
end. Being no model error, that leaves the status `0`.

The prompt is the exception: a line it could not carry out is reported and the
session goes on, and `%quit` or Ctrl-D exits `0`. `sysml model.sysml` at a
terminal loads the model, reports what analysis found, and opens the prompt with
status `0` — the prompt is where the model gets fixed. The same command with its
lines coming from a pipe or a file gates: it exits `2` for a model that did not
analyse cleanly, and for one whose feature values a command could not materialize.

## Use Cases

1. **Quick Calculations**: Use as a calculator with `-e`
2. **Automated Testing**: Run model evaluations in CI/CD pipelines
3. **Scripting**: Extract calculated values for other tools
4. **Validation**: Check model properties without manual interaction
5. **Batch Processing**: Process multiple models programmatically
6. **Interactive Development**: Load files and explore in REPL

## Tips

- Use `%help` in REPL to see all meta commands
- Combine multiple `-e` flags to evaluate related expressions
- Load common definitions before custom models
- Read [Exit status](#exit-status) before gating a pipeline on one: `0` means the
  model answered what was asked, `1` that it answered false, `2` that it answered
  nothing
- Results are on **stdout** and findings — diagnostics, warnings, whatever
  stopped the run — on **stderr**, so `> model.ttl` and `2> errors.log` separate
  them
- Use shell pipes for REPL automation: `echo "%load file.sysml" | sysml`
