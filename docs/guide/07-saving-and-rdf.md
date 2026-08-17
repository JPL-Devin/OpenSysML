# 7. Saving, and converting to RDF

A model is written in two representations — SysML v2 notation (`.sysml`, `.kerml`) and RDF in
Turtle (`.ttl`) — and converted between them, from the prompt, the command line, or over gRPC.
No JSON is involved anywhere in this path, not even as an intermediate form.

The vocabulary each triple uses, and what the mapping does not cover, is
[reference/rdf-mapping.md](../reference/rdf-mapping.md).

> **RDF conversion is experimental.** It covers model structure and the behavior
> its bodies state, refuses what it cannot write back, and its vocabulary may
> change without a compatibility path, so every run that converts RDF says so.
> Saving to `.sysml` or `.kerml` is stable and exact. See
> [reference/rdf-mapping.md § Status](../reference/rdf-mapping.md#status-experimental).

## Saving a session

`%save` writes the session out. The format follows the extension — `.sysml` for
notation, `.ttl` for RDF Turtle:

```bash
sysml> %save my_model.sysml
saved 181 bytes of sysml to my_model.sysml (replaced the existing file)

sysml> %save my_model.ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
saved 1872 bytes of ttl to my_model.ttl
```

A leading `~` is expanded, an existing file is replaced and the replacement is
stated, and the write is atomic — an interrupted save leaves the previous file
intact. A file that already exists keeps its permissions, and a symlink is
written through rather than replaced.

A session that does not fully parse is still saved as notation: that file is
your own text re-indented, so the syntax errors are reported as warnings and the
work is never trapped in the REPL.

```bash
sysml> %save my_model.sysml
warning: <session>: 1 syntax error(s):
  4:6: expected a namespace member
warning: the file is saved as typed; fix these and save again
saved 181 bytes of sysml to my_model.sysml (replaced the existing file)
```

`.ttl` keeps the refusal, because a graph built from a tree the parser only
partly recovered would be quietly missing declarations. So does
`sysml -convert`, where the source already exists on disk.

The same conversion is available without starting the REPL:

```bash
$ sysml my_model.sysml -convert ttl -o my_model.ttl   # notation to RDF
$ sysml my_model.ttl -convert sysml -o back.sysml      # RDF to notation
$ sysml my_model.sysml -convert ttl                    # to stdout
```

The model is named the way it is everywhere else on this command line, and
`-convert` names the format to convert it to.

## Converting from the command line

```bash
sysml model.sysml -convert ttl -o model.ttl     # notation to RDF
sysml model.ttl -convert sysml -o model.sysml   # RDF to notation
sysml model.sysml -convert ttl                  # to stdout
```

The model is a positional argument, as it is for every other mode of the
command, and `-convert` names the format to convert it to. Flags may be written
before or after the model.

The input format is taken from the file extension. When that extension is
missing or unrecognized, `-from` names it:

```bash
sysml input.txt -convert ttl -from sysml
```

`-convert`/`-from` accept `sysml`, `kerml`, `text`, `ttl`, `turtle` and `rdf`.
The output path plays no part in choosing the format, so a destination with no
extension — `-o /dev/null`, a FIFO — needs nothing extra.

Converting to the same format rewrites the input: notation is reformatted, and
Turtle is normalized (prefixes sorted, predicates grouped by subject).

### Exit status

The command exits non-zero and writes nothing on any input it cannot convert
faithfully — a syntax error in the notation, malformed Turtle, or an RDF
construct outside the mapping below. It never writes a partial model.

## Converting over gRPC and from Python

The same conversion is a service method, `Convert`, reported as the `convert`
capability by `GetServerInfo`. It reads a `file_path` the service opens or
`content` carried inline, takes the format names `-from`/`-to` take, and returns
the written text with its formats, or an `error` plus the diagnostics explaining
it. `tolerate_syntax_errors` writes notation despite syntax errors, and is
rejected for any direction that builds a graph, where an unparsed declaration
would go missing without saying so.

A response whose conversion went through the RDF mapping sets `experimental` and
`experimental_notice`, on a refusal as well as on a success, so a client learns
the status from the response rather than from this page. pysysml raises it as an
`ExperimentalFeatureWarning`, which `warnings.simplefilter` can silence:

```python
import warnings
from pysysml import ExperimentalFeatureWarning

warnings.simplefilter("ignore", ExperimentalFeatureWarning)
```

From Python:

```python
model = pysysml.load("model.sysml")
model.save("model.ttl")                          # SysML notation to RDF
pysysml.convert("sysml", file_path="model.ttl")  # and back
```

The client API is [reference/python-api.md](../reference/python-api.md), and using it as a task is
[chapter 9](09-python.md).

## Round-tripping

`notation → RDF → notation` returns an equivalent model, and
`notation → RDF → notation → RDF` returns the *same graph* — which is the
property the test suite asserts, over the fixtures in
`internal/core/export/testdata/convert/`.

The notation on the far side of a round trip is equivalent but not always
character-identical: a reference may come back written relative to a different
scope, and a clause written `:>` comes back as `specializes`. Both parse to the
same model, and the second conversion to RDF proves it.

**A save to `.sysml` is different, and is exact.** It writes the session's own
source through the formatter rather than re-printing the graph, so comments,
notes and spacing survive. Only the `.ttl` direction goes through the mapping.
The syntax is still checked: every direction rejects notation the parser cannot
read, so a save never quietly reformats a model that will not parse.

## A worked example

[`examples/rdf-interop-demo.sysml`](../../examples/rdf-interop-demo.sysml) is the
reference model for this document: a rover and its ground link, declared with
packages, definitions, usages, ports, a connection, multiplicity, values and
documentation — all inside the mapping — so it converts and comes back:

```bash
$ sysml examples/rdf-interop-demo.sysml -convert ttl -o /tmp/rover.ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/rover.ttl (ttl, 7937 bytes)
$ sysml /tmp/rover.ttl -convert sysml -o /tmp/rover-back.sysml
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/rover-back.sysml (sysml, 877 bytes)
```

Converting the returned notation again yields a byte-identical graph — the
round-trip property described above. The `//` header comment is the one thing
lost, as [reference/rdf-mapping.md](../reference/rdf-mapping.md) describes; the package's `doc` and `comment` are
declarations and survive.

[`examples/semantic-layer/demo.sysml`](../../examples/semantic-layer/demo.sysml)
and [`examples/repl-behavioral-demo.sysml`](../../examples/repl-behavioral-demo.sysml)
convert too, as do the structure-only `parser_features_demo_*.kerml` files
(except `..._advanced_bodies.kerml`, which computes a value, and three that each
declare one name twice). What converts is structure; a model whose point is a
behavior does not, the conversion stopping at a state, a region, an assignment,
an action node or a name two members of one body share (how much of `examples/`
that leaves converting is measured in
[project/roadmap.md](../project/roadmap.md#d6--a-behavioral-node-has-no-metaclass-so-a-model-stating-steps-cannot-convert)):

```bash
$ sysml examples/state-machine-demo.sysml -convert ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
sysml: cannot convert the substate member at examples/state-machine-demo.sysml:7:13: save to .sysml or .kerml instead, which writes the source exactly; see docs/reference/rdf-mapping.md § Limitations
```

---

Next: [8. Editors](08-editors.md).
