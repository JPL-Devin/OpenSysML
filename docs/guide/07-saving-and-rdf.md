# 7. Saving, and converting to RDF

A model can be written in two representations, SysML v2 notation (`.sysml`, `.kerml`) and RDF in
Turtle (`.ttl`), and converted between them from the prompt, the command line or over gRPC. JSON
is not used anywhere in this path, including as an intermediate form.

The vocabulary used by each triple, and the constructs the mapping does not cover, are documented
in [reference/rdf-mapping.md](../reference/rdf-mapping.md).

> **RDF conversion is experimental.** It covers model structure and the behavior stated by model
> bodies, refuses any construct it cannot write back, and its vocabulary may change without a
> compatibility path, which every run that converts RDF reports. Saving to `.sysml` or `.kerml`
> is stable and exact. See
> [reference/rdf-mapping.md § Status](../reference/rdf-mapping.md#status-experimental).

## Saving a session

`%save` writes the session out, selecting the format from the file extension: `.sysml` for
notation and `.ttl` for RDF Turtle.

```bash
sysml> package Demo { private import ScalarValues::*; part def Vehicle { attribute mass : Real = 1500.0; } }
✓ package Demo

sysml> %save my_model.sysml
saved 102 bytes of sysml to my_model.sysml

sysml> %save my_model.ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
saved 1487 bytes of ttl to my_model.ttl
```

A leading `~` is expanded, an existing file is replaced and the replacement is reported, and the
write is atomic, so an interrupted save leaves the previous file intact. An existing file retains
its permissions, and a symlink is written through rather than replaced.

A session that does not fully parse is still saved as notation. The resulting file contains the
authored text, re-indented, so the syntax errors are reported as warnings and no work is left
inaccessible in the REPL.

```bash
sysml> package Demo {
  ...>   part def Vehicle;
  ...>   ???
  ...> }
3:3: error: expected a namespace member
  ???
  ^~
warning: <session>: 1 syntax error(s):
  3:3: expected a namespace member
warning: the file is saved as typed; fix these and save again
sysml> %save my_model.sysml
warning: <session>: 1 syntax error(s):
  3:3: expected a namespace member
warning: the file is saved as typed; fix these and save again
saved 47 bytes of sysml to my_model.sysml
```

The `.ttl` direction still refuses such a session, because a graph built from a tree the parser
only partly recovered would silently omit declarations. `sysml -convert` behaves the same way,
since the source already exists on disk.

The same conversion is available without starting the REPL:

```bash
$ sysml my_model.sysml -convert ttl -o my_model.ttl   # notation to RDF
$ sysml my_model.ttl -convert sysml -o back.sysml      # RDF to notation
$ sysml my_model.sysml -convert ttl                    # to stdout
```

The model is named as it is in every other invocation of the command, and `-convert` names the
target format.

## Converting from the command line

```bash
sysml model.sysml -convert ttl -o model.ttl     # notation to RDF
sysml model.ttl -convert sysml -o model.sysml   # RDF to notation
sysml model.sysml -convert ttl                  # to stdout
```

The model is a positional argument, as in every other mode of the command, and `-convert` names
the target format. Flags may be written before or after the model.

The input format is inferred from the file extension. When the extension is missing or
unrecognized, specify it with `-from`:

```bash
sysml input.txt -convert ttl -from sysml
```

`-convert` and `-from` accept `sysml`, `kerml`, `text`, `ttl`, `turtle` and `rdf`. The output path
plays no part in format selection, so a destination without an extension, such as `-o /dev/null`
or a FIFO, requires no additional flags.

Converting to the same format rewrites the input: notation is reformatted, and
Turtle is normalized (prefixes sorted, predicates grouped by subject).

### Exit status

The command exits with a non-zero status and writes nothing for any input it cannot convert
faithfully, whether a syntax error in the notation, malformed Turtle, or an RDF construct outside
the mapping. It never writes a partial model.

## Converting over gRPC and from Python

The same conversion is available as the service method `Convert`, reported as the `convert`
capability by `GetServerInfo`. It accepts either a `file_path` that the service opens or `content`
carried inline, takes the same format names as `-from` and `-to`, and returns the written text
with its formats, or an `error` together with the diagnostics that explain it.
`tolerate_syntax_errors` writes notation despite syntax errors and is rejected for any direction
that builds a graph, where an unparsed declaration would be omitted without notice.

A response whose conversion used the RDF mapping sets `experimental` and `experimental_notice` on
both refusals and successes, so a client obtains the status from the response rather than from
this page. The `opensysml` client raises this as an `ExperimentalFeatureWarning`, which
`warnings.simplefilter` can suppress:

```python
import warnings
from opensysml import ExperimentalFeatureWarning

warnings.simplefilter("ignore", ExperimentalFeatureWarning)
```

From Python:

```python
model = opensysml.load("model.sysml")
model.save("model.ttl")                          # SysML notation to RDF
opensysml.convert("sysml", file_path="model.ttl")  # and back
```

The client API is documented in [reference/python-api.md](../reference/python-api.md), and
[chapter 9](09-python.md) describes its use.

## Round-tripping

`notation → RDF → notation` returns an equivalent model, and
`notation → RDF → notation → RDF` returns the *same graph*. This is the property asserted by the
test suite over the fixtures in `internal/core/export/testdata/convert/`.

The notation produced by a round trip is equivalent but not always character-identical: a
reference may be written relative to a different scope, and a clause written `:>` is returned as
`specializes`. Both forms parse to the same model, which the second conversion to RDF confirms.

**A save to `.sysml` behaves differently and is exact.** It writes the session's own source
through the formatter rather than re-printing the graph, so comments, notes and spacing are
preserved. Only the `.ttl` direction uses the mapping. Syntax is checked in every direction, and
notation the parser cannot read is rejected, so a save never silently reformats a model that does
not parse.

## A worked example

[`examples/rdf-interop-demo.sysml`](../../examples/rdf-interop-demo.sysml) is the reference model
for this chapter: a rover and its ground link, declared with packages, definitions, usages, ports,
a connection, multiplicity, values and documentation, all of which the mapping covers, so the
model converts in both directions:

```bash
$ sysml examples/rdf-interop-demo.sysml -convert ttl -o /tmp/rover.ttl
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/rover.ttl (ttl, 10296 bytes)
$ sysml /tmp/rover.ttl -convert sysml -o /tmp/rover-back.sysml
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/rover-back.sysml (sysml, 877 bytes)
```

Converting the returned notation again yields a byte-identical graph, which is the round-trip
property described above. The `//` header comment is the only content lost, as described in
[reference/rdf-mapping.md](../reference/rdf-mapping.md); the package's `doc` and `comment` are
declarations and are preserved.

[`examples/semantic-layer/demo.sysml`](../../examples/semantic-layer/demo.sysml)
and [`examples/repl-behavioral-demo.sysml`](../../examples/repl-behavioral-demo.sysml)
also convert, as do most `parser_features_demo_*.kerml` files, with the exception of
`..._advanced_bodies.kerml`, which computes a value, and two files that each declare one name
twice. The behavior stated by a body converts as well: states, regions, substates, action nodes,
assignments and transitions all have a mapping. Refusals apply to constructs from which the
notation could not be rebuilt, such as an expression the graph would have to compute, a name
shared by two members of one body, or a declaration without a name. A refusal names the construct
at which conversion stopped and recommends saving the source instead:

```bash
$ sysml examples/parser_features_demo_action_semantics.sysml -convert ttl -o /tmp/action-semantics.ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
wrote /tmp/action-semantics.ttl (ttl, 21671 bytes)
0
```

For example, an advanced body containing an operator expression is refused:

```bash
$ sysml examples/parser_features_demo_advanced_bodies.kerml -convert ttl; echo $?
note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change without a compatibility path; see docs/reference/rdf-mapping.md § Status
sysml: cannot convert the operator expr at examples/parser_features_demo_advanced_bodies.kerml:87:9: save to .sysml or .kerml instead, which writes the source exactly; see docs/reference/rdf-mapping.md § Limitations
2
```

---

Next: [8. Editors](08-editors.md).
