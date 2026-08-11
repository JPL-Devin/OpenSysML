# Saving Models and Converting Between SysML Notation and RDF

Systemica writes a model in two representations and converts between them:

- **SysML v2 textual notation** — `.sysml`, `.kerml`
- **RDF, Turtle syntax** — `.ttl`

There is no JSON involved anywhere in this path: not as an input, not as an
output, and not as an intermediate form. A conversion goes from notation
straight to triples, or from triples straight to notation.

## Saving from the REPL

```
sysml> package Demo { part def Vehicle { attribute mass : Real = 1200.0; } }
sysml> %save model.sysml      # the notation
sysml> %save model.ttl        # the same model as RDF
```

`%save` writes everything declared in the session, in declaration order. The
format comes from the file extension; an unrecognized extension is an error
rather than a guess, and an empty session writes no file.

## Converting from the command line

```bash
sysml -convert model.sysml -o model.ttl     # notation to RDF
sysml -convert model.ttl   -o model.sysml   # RDF to notation
sysml -convert model.sysml                  # to stdout, in the other format
```

The format of each side is taken from its file extension. When an extension is
missing or unrecognized, name the formats explicitly:

```bash
sysml -convert input.txt -from sysml -to ttl
```

`-from`/`-to` accept `sysml`, `kerml`, `text`, `ttl`, `turtle` and `rdf`.

Converting to the same format rewrites the input: notation is reformatted, and
Turtle is normalized (prefixes sorted, predicates grouped by subject).

### Exit status

The command exits non-zero and writes nothing on any input it cannot convert
faithfully — a syntax error in the notation, malformed Turtle, or an RDF
construct outside the mapping below. It never writes a partial model.

## The RDF mapping

### Namespaces

| Prefix  | IRI                                            | Holds |
|---------|------------------------------------------------|-------|
| `sysml:` | `https://www.omg.org/spec/SysML#`             | Metaclasses and metamodel properties |
| `elmt:`  | `urn:sysmlv2:element:`                        | The elements of the converted model |
| `sysx:`  | `urn:systemica:sysml:`                        | The few properties the metamodel does not define |
| `rdf:`, `xsd:` | the standard RDF and XML Schema namespaces | `rdf:type`, literal datatypes |

The `sysml:` vocabulary and the `elmt:` element base match the ones the
[Flexo MMS SysML v2 service](https://github.com/Open-MBEE/flexo-mms-sysmlv2)
writes into its triplestore (`Namespaces.kt`), so a graph produced here loads
into that service.

Systemica's own additions are namespaced separately as `sysx:` so a consumer can
tell them from the standard vocabulary and ignore them if it wants only standard
SysML.

### Element IRIs

An element's IRI is its qualified name appended to `elmt:`:

```
package Demo { part def Vehicle; }
```

```turtle
elmt:Demo         a sysml:Package .
elmt:Demo::Vehicle a sysml:PartDefinition ;
    sysml:owningNamespace elmt:Demo .
```

The IRI is therefore **deterministic**: converting the same model twice yields
the same IRIs, and re-converting after an edit leaves the untouched elements
addressed as before. Characters that an IRI cannot carry are percent-escaped and
decode back exactly.

### What each element carries

- `rdf:type` — the SysML metaclass (`sysml:PartUsage`, `sysml:ActionDefinition`, …).
  Every definition and usage keyword the parser accepts has a metaclass; the
  tables in `internal/core/export/kinds.go` are the source of truth, and the
  reverse direction is derived from them so the two cannot disagree.
- `sysml:declaredName`, `sysml:declaredShortName`, `sysml:qualifiedName`
- `sysml:owningNamespace` — the containing element (absent on a root)
- `sysml:visibility`, `sysml:direction`
- Feature flags, written only when true, so an absent flag reads as false:
  `isAbstract`, `isVariation`, `isReference`, `isComposite`, `isDerived`,
  `isOrdered`, `isNonunique`, `isEnd`, `isConstant`, `isEvent`, `isIndividual`,
  `isSnapshot`, `isConjugated`, `isAll`, `isAccept`, `isResult`
- Declaration-head relationships, as element IRIs where the target resolves
  inside the model and as plain literals where it does not: `sysml:type`
  (the `:` clause), `specializes`, `subsets`, `redefines`, `references`,
  `crosses`, `disjointFrom`, `intersects`, `inverseOf`, `unions`, `chains`,
  `includes`, `via`, `annotates`, `subject`
- `sysml:lowerBound`, `sysml:upperBound` — multiplicity
- `sysml:value` — a feature's value
- `sysml:importedNamespace`, `sysml:aliasedElement`, `sysml:client`,
  `sysml:supplier`, `sysml:body`, `sysml:language`, `sysml:locale`,
  `sysml:annotatedElement`

The three `sysx:` properties:

| Property | Why it exists |
|----------|---------------|
| `sysx:memberIndex` | Declaration order. The notation is sensitive to the order of members; an RDF graph is an unordered set, so the index is what lets a conversion back to notation reproduce the original sequence. |
| `sysx:hasBody` | Distinguishes `part def A;` from `part def A { }`, which are different source and would otherwise convert back identically. |
| `sysx:sourceText` | The verbatim source of the constructs described under *Limitations*. |

Comments, documentation and textual representations convert as their own
elements (`sysml:Comment`, `sysml:Documentation`, `sysml:TextualRepresentation`)
carrying `sysml:body`.

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

## Limitations

These are the constructs the mapping does not fully represent. Each is a
documented limitation, not a silent one: converting an affected element from a
graph that lacks the text reports an error naming the element rather than
guessing.

**Expressions are carried as source text.** Feature values, multiplicity
bounds, filter conditions and succession guards are stored as their notation
(`sysml:value "1200.0"`) rather than decomposed into expression trees. They
convert back exactly and a consumer reading model structure is unaffected, but
SPARQL cannot see inside them.

**Lexical comments do not survive the RDF hop.** `//` and `/* */` trivia is
attached to no element, so a `notation → RDF → notation` round trip drops it:

```sysml
// this line is lost through .ttl
package Demo {
    doc /* this is kept: doc is a declaration, not trivia */
    comment about Wheel /* kept for the same reason */
    part def Wheel;
}
```

The `comment` and `doc` keywords declare elements, so they convert both ways.
Save straight to `.sysml` when the trivia matters — that path writes the source
and keeps everything.

**Declaration heads that bind ends are carried as `sysx:sourceText`.** A
`connect`, `bind`, `flow`, `succession`, `transition`, `accept` or `satisfy`
declaration has a head whose participants are not reconstructible from the
properties above, so the head is kept verbatim alongside its structural
properties. These round-trip exactly through Systemica. A graph produced by
*another* tool will not carry the text, and converting such an element to
notation then reports it as unsupported.

**A `then` succession is refused.** `ast.Membership.HasSuccession` records only
that a `then` was written, not which two members it sequences — and the parser
sets it on the member *before* the keyword in some positions and the member
*after* it in others. Writing the keyword back from that flag would be a guess,
and a wrong guess reorders execution, so a model containing `then` is reported
rather than converted:

```
cannot convert the `then` succession at model.sysml:5:3: this mapping cannot
tell which members a succession sequences, and will not guess at execution order
```

Making the parser record successions unambiguously is roadmap item D4; until
then, behavioral models that sequence steps with `then` convert only to notation.

**A name declared twice in one namespace is refused.** An element's identity in
the graph is its qualified name, so `part def A; part def A;` in one container
would merge into a single subject. The duplicate is reported instead.

**Unsupported on the RDF input side**, each an error naming the line or element:

- blank nodes and `[ ... ]` — every element must have a stable IRI
- RDF collections `( ... )` — order is carried by `sysx:memberIndex`
- an element with no `rdf:type`, or a metaclass outside the mapping
- an element whose `sysml:owningNamespace` is not in the graph
- ownership that forms a cycle, leaving an element no root owns — printing walks
  down from the roots, so this would otherwise write an empty document
- Turtle syntax errors, reported with a line number
- literal shorthands (bare numbers and booleans); literals must be quoted,
  with an `xsd:` datatype where one applies

A graph that uses none of Systemica's `sysx:` properties — one produced by
another tool — converts as far as the mapping allows and errors on the first
element it cannot place, rather than emitting a model with elements missing.

## Where the code lives

| Package | Role |
|---------|------|
| `internal/core/rdf` | Triple/graph model, Turtle writer, Turtle parser |
| `internal/core/export` | `ToRDF` (AST → graph), `ToSysML` (graph → notation), and the `Convert` entry point |
| `internal/repl` | `%save` |
| `cmd/sysml` | `-convert`, `-from`, `-to`, `-o` |

The RDF layer is hand-written against the Turtle grammar rather than pulled in
as a dependency: the subset needed here is small, and the parser rejects what it
does not support instead of accepting it and dropping data.
