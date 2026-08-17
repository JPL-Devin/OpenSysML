# The SysML ↔ RDF mapping

Which triples a model becomes, and which constructs the mapping does not represent. Saving and
converting as a task is [guide chapter 7](../guide/07-saving-and-rdf.md).

## Status: experimental

RDF conversion — `sysml -convert ttl`, `%save model.ttl`, the service's `Convert`
to or from `ttl`, and each in reverse — is **experimental** as of 0.1.0. Saving
and converting notation (`.sysml`, `.kerml`) is stable; this mapping is not, and
each of these is a deliberate property of it rather than a defect to report:

- **What is not mapped is refused, not partly converted**, with the construct
  named: 102 of the 120 models under `examples/` convert to Turtle and the other
  18 are refused. See [Behavior](#behavior) and [Limitations](#limitations).
- **The vocabulary may change without a compatibility path.** A graph written by
  one release may not read back into the next, and no migration is provided.
  Treat a `.ttl` as an interchange artifact you can regenerate, not as the copy
  of record.
- **Interoperability is unverified.** The `sysml:` vocabulary and the `elmt:`
  element base are read from Flexo MMS's `Namespaces.kt`, but no round trip
  through a running Flexo service or triplestore has been demonstrated, so
  nothing here claims one (roadmap item D3).

Every surface reports this where it is used: the command line writes a `note:` to
stderr, `%save` prints one, and `ConvertResponse` carries `experimental` and
`experimental_notice`, which pysysml raises as an `ExperimentalFeatureWarning`.
The wording is one constant, `export.ExperimentalNotice`.

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
writes into its triplestore (`Namespaces.kt`), so a graph produced here is
addressed the way that service addresses elements. Whether such a graph loads
into a running Flexo triplestore has not been demonstrated — see
[Status](#status-experimental).

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
  `includes`, `via`, `annotates`, `subject`. A literal carries the name itself,
  without the quotes an unrestricted name is written with; a target that is an
  expression rather than a name (a feature chain, say) is carried as the text it
  was written as, typed `sysx:Expression` to tell the two apart.
- `sysml:lowerBound`, `sysml:upperBound` — multiplicity
- `sysml:value` — a feature's value
- `sysml:importedNamespace`, `sysml:aliasedElement`, `sysml:client`,
  `sysml:supplier`, `sysml:body`, `sysml:language`, `sysml:locale`,
  `sysml:annotatedElement`

The `sysx:` properties:

| Property | Why it exists |
|----------|---------------|
| `sysx:memberIndex` | Declaration order. The notation is sensitive to the order of members; an RDF graph is an unordered set, so the index is what lets a conversion back to notation reproduce the original sequence. |
| `sysx:hasBody` | Distinguishes `part def A;` from `part def A { }`, which are different source and would otherwise convert back identically. |
| `sysx:sourceText` | The verbatim source of the constructs described under *Limitations*. |
| `sysx:declaredKeyword` | The kind keyword as written, when it is one of the synonyms several keywords share (`datatype` and `attribute`, `function` and `calc`, `snapshot` and `occurrence`). The AST records one kind for all of them, so without this the notation would come back rewritten. Also the keyword a constraint body's condition is stated with (`assert`, `assume`, or absent for a bare condition, which asserts implicitly). |
| `sysx:declaredPrefix` | The keyword qualifying the kind keyword after it — the `assert` of `assert constraint c : C`. It says what the declaration is for, and the AST kind alone does not carry it. |
| `sysx:condition` | The condition a condition member states, as its notation. |
| `sysx:prefixMetadata` | A metadata annotation as written (`#Safety`). It states what the element it prefixes is, and the AST records no span for it, so the notation is read from the source. |
| `sysx:isKindImplicit` | The declaration wrote no kind keyword (`in x : Real;`), which takes its kind from its owner. Without it the canonical keyword would come back written out, declaring what the author did not. A kind named in a comment in the head (`in /* attribute */ x : Real;`) is trivia, not a keyword the declaration wrote. |
| the behavioral properties | `sysx:guard`, `sysx:expression`, `sysx:payload`, … — the parts of a behavioral node the vocabulary has no predicate for, listed under [Behavior](#behavior). |

Metaclass names with no counterpart in the OMG vocabulary are typed in the
`sysx:` namespace rather than `sysml:`, so a consumer can tell them from the
standard metaclasses: `sysx:Alias`, `sysx:FilterMember`,
`sysx:MultiplicityDeclaration`, `sysx:ConstraintMember`, `sysx:AssumeMember`,
`sysx:RequireMember`, `sysx:ResultMember`, and the behavioral ones listed under
[Behavior](#behavior).

Comments, documentation and textual representations convert as their own
elements (`sysml:Comment`, `sysml:Documentation`, `sysml:TextualRepresentation`)
carrying `sysml:body`.

## Behavior

An action or state body converts: each node it states has a metaclass and the
properties its notation is rebuilt from, so `notation → RDF → notation` returns
the body byte-identically (`behavior_test.go`). Where the OMG vocabulary names
the node, that name is used; the rest are `sysx:` terms, marked below.

| written | metaclass | carries |
|---|---|---|
| `first x;`, `initial x;` | `sysx:InitialNode` | `sysml:sourceFeature` (the member the body starts at — a reference, not a name it declares), `sysml:targetFeature`, `sysx:guard`, `sysx:declaredKeyword` |
| `done;`, `final;` | `sysx:FinalNode` | `sysml:declaredName`, `sysx:declaredKeyword` |
| `action a;`, `action a { x + 1 }` | `sysx:ActionExecutionNode` | `sysml:references` or `sysx:expression` |
| `perform a;` | `sysml:PerformActionUsage` | `sysx:expression` (the action performed) |
| `assign x := 1;` | `sysml:AssignmentActionUsage` | `sysx:target`, `sysml:value`, `sysx:assignmentOperator` when it is not `:=` |
| `send M(x) to p;`, `… via p;` | `sysml:SendActionUsage` | `sysx:payload`, `sysx:receiver`, `sysx:isVia` |
| `terminate;`, `terminate x;` | `sysml:TerminateActionUsage` | `sysx:expression` |
| `accept sig : Signal;`, `accept when c;` | the usage's own metaclass | `sysml:isAccept`, and `sysx:declaredKeyword "accept"` where the optional `action` was not written |
| `fork`, `join`, `merge`, `decision` | `sysml:ForkNode`, `JoinNode`, `MergeNode`, `DecisionNode` | `sysml:declaredName` |
| `then a b;`, `if g then b;`, `else b;` | `sysml:SuccessionAsUsage` | `sysml:sourceFeature`, `sysml:targetFeature`, `sysx:guard`, `sysx:isElse`, `sysx:declaredKeyword` |
| `while c { … }`, `loop { … } until c;` | `sysml:WhileLoopActionUsage` | `sysx:whileCondition`, `sysx:untilCondition` |
| `for x in c { … }` | `sysml:ForLoopActionUsage` | `sysx:loopVariable`, `sysx:collection` |
| `if c { … } else { … }` | `sysml:IfActionUsage` + `sysx:IfBranch` per branch | `sysx:condition`, `sysx:branchKind` |
| `state s { … }`, `initial s;`, `final s;` | `sysml:StateUsage` | `sysml:declaredName`, `sysx:declaredKeyword`, its members |
| `entry`/`do`/`exit`, `entry do { … }` (with or without a space before the body) | `sysml:StateSubactionMembership` | `sysx:subactionKind`, `sysx:declaredKeyword`, its actions |
| `defer sig, other;` | `sysx:DeferMember` | `sysx:deferredEvent` per event |
| `region r { … }` | `sysx:StateRegion` | `sysml:declaredName`, its states |
| `choice`, `junction`, `fork`, `join`, `entry point`, `exit point`, `shallow`/`deep history` | `sysx:Pseudostate` | `sysx:pseudostateKind`, `sysx:declaredKeyword` |
| `transition [n] first s [accept t] [if g] [do e] then t;`, `transition s to t;` | `sysml:TransitionUsage` | `sysml:sourceFeature`, `sysml:targetFeature`, `sysx:trigger`, `sysx:triggerKeyword`, `sysx:guard`, `sysx:transitionSyntax`, its effect |

A state's members are held in the AST in one bucket per kind (entry, do, exit,
defer, substates, regions); they are written back in the order they were
declared, taken from their source spans, so `do` before `entry` stays that way.

The conditions and expressions these nodes carry are notation, as everywhere
else in this mapping, so they convert back exactly but SPARQL cannot see inside
them.

What is still refused, with the node named:

- **A succession that does not name both of its ends.** `then fork;` and
  `then monitorPedal;` written under a preceding member state an order whose
  source end the notation leaves implicit, and the parser records the node the
  statement introduces separately from the edge into it. Reconstructing that
  shape means inferring which node an edge belongs to from member position,
  which would silently reattach edges, so it is reported instead. Nine of the
  eighteen remaining refusals under `examples/` are this shape.
- **A succession end whose name is not a basic name.** The two-end form the
  graph is written back as (`then a b;`) is read by the parser only when both
  ends are basic names, so `then 'enter vehicle' 'drive vehicle';` would not
  parse; the edge is reported rather than written.
- **Prefix metadata** (`#Safety part p;`, `@M { … }`) and an **operator
  expression member**, both unchanged from before.

## Limitations

These are the constructs the mapping does not fully represent. Each is a
documented limitation, not a silent one: converting an affected element from a
graph that lacks the text reports an error naming the element rather than
guessing.

**Expressions are carried as source text.** Feature values, multiplicity
bounds, filter conditions, constraint conditions and succession guards are
stored as their notation (`sysml:value "1200.0"`) rather than decomposed into
expression trees. They convert back exactly and a consumer reading model
structure is unaffected, but SPARQL cannot see inside them.

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

**A `then` succession carries its two ends.** Every `then` is one node naming
the members it sequences, whether it was written as its own member (`then a b;`)
or attached to one (`then action b : B;`, which the parser desugars to the same
edge), so the order a model declares survives the hop:

```turtle
<urn:sysmlv2:element:P::Move::@2>
    a sysml:SuccessionAsUsage ;
    sysml:sourceFeature elmt:P::Move::a ;
    sysml:targetFeature elmt:P::Move::c .
```

Converting that back writes `then a c;`, which sequences the same pair wherever
the two members are declared — member order alone would not have carried it
(`export_test.go:TestSuccessionRoundTrips`). Every body that can carry a
succession — definition, usage, action, state (a region included), calculation
and requirement — reads that form back as the same node, so a second conversion
yields the same graph (`export_test.go:TestSuccessionRoundTripsInEveryBody`).

A `then` beside a member with no name (`then send Show(x) to screen;`) declares
an order these ends cannot name. The parser warns (`unnamed-succession-end`) and
records no edge, so the conversion carries the members without it; an edge that
reaches the encoder naming only one end is reported rather than written back —
see [Behavior](#behavior) for that boundary.

**Two keyword prefixes are normalized away.** `variant` and `include` prefix a
kind keyword the AST records on its own, and the prefix is not recorded, so
`variant part a : A;` comes back as `part a : A;` and `include U;` as a plain
use-case reference. Unlike the synonyms above these cannot be detected at
conversion time, so they are normalized rather than reported — the one place
this mapping changes a model without saying so. Recording them in the parser is
roadmap item D5. Save straight to `.sysml` when they matter.

**A reference end is written back in a spelling the parser reads back
differently.** `end [*] ref cause : Situation;` is carried faithfully — the
graph states `sysml:isReference` — and comes back as
`end ref attribute cause : Situation[*];`, but the parser records no reference
flag for a `ref` that follows `end`, so converting *that* notation again drops
the `ref`. The graph is right; recording the flag in the parser is what a stable
second hop needs.

**Conditions convert as their notation.** The members that state
a condition are carried, each as the `sysx:` metaclass named above with its
condition as `sysx:condition`: a constraint body's conditions (`assert`,
`assume`, a bare condition, and the `not` of `assert not …` as
`sysml:isNegated`), a nested `assert constraint [name] { … }`, a requirement's
`assume`/`require` members in all three forms (an expression, the constraint
they name, or a body), `subject s : X;` as the `sysml:SubjectMembership` it
declares, and `return <expr>;`. The `assert` prefixing a named usage
(`assert constraint c : C`) is carried as `sysx:declaredPrefix`. The conditions
themselves are notation, with the limits stated above.

The nodes an action or state body states are mapped under
[Behavior](#behavior), together with the shapes still refused there.

**A synonym keyword that names no element of its own is refused.** `snapshot s;`
shares its AST kind with `occurrence`, and a declaration that carries no name of
its own has nothing for `sysx:declaredKeyword` to hang off, so writing it back as
the canonical `occurrence` would be a different declaration. It is reported
instead. `perform a : A;` does convert: the `perform` is kept as the keyword it
was written with.

**A metadata annotation is carried as the notation it was written as**
(`sysx:prefixMetadata "#Safety"`), read from the source because the AST records
no span for the annotation itself. Two shapes are reported rather than written
back: an annotation carrying a body of its own (`@M { isSet = true; }`), which
the vocabulary has no properties for, and an `@` annotation ahead of a
definition (`@Safety part def Car;`), which the parser records on the
declaration *before* the one it prefixes — writing that back would annotate a
different element.

**A name declared twice in one namespace is refused.** An element's identity in
the graph is its qualified name, so `part def A; part def A;` in one container
would merge into a single subject. The duplicate is reported instead.

A shorthand relationship declares no name: the `result` of `bind result = x;` and
the `x` of `first x;` name the end the statement relates, so those elements are
addressed by position (`sysx:memberIndex`) and the name is carried as a
reference. Without that they collided with the member they name and the model was
refused as a duplicate.

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
| `cmd/sysml` | `-convert`, `-from`, `-o` |

The RDF layer is hand-written against the Turtle grammar rather than pulled in
as a dependency: the subset needed here is small, and the parser rejects what it
does not support instead of accepting it and dropping data.
