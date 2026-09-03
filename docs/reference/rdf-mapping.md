# The SysML ↔ RDF mapping

This page describes which triples a model becomes, and which constructs the mapping does not
represent. For saving and converting as a task, see [guide chapter 7](../guide/07-saving-and-rdf.md).

## Status: experimental

RDF conversion (`sysml -convert ttl`, `%save model.ttl`, the service's `Convert`
to or from `ttl`, and each in reverse) is **experimental** as of 0.1.0. Saving
and converting notation (`.sysml`, `.kerml`) is stable; this mapping is not. Each
of the following is a deliberate property of the mapping rather than a defect to
report:

- **What is not mapped is refused, not partly converted**, and the refusal names
  the construct. 268 of the 345 models under `examples/` (committed, training and
  pilot corpora) convert to Turtle; the other 77 are refused. Of the 268, a second
  conversion of the written-back notation reproduces the graph for 249 (177
  byte-for-byte, 72 up to the whitespace inside `sysx:sourceText`), differs for
  14, and 5 cannot be written back or re-read at all. These figures are the
  per-file ratchet in `internal/core/export/corpus_roundtrip_test.go`, described
  in [rdf-corpus-roundtrip.md](../project/rdf-corpus-roundtrip.md). See
  [Behavior](#behavior) and [Limitations](#limitations).
- **The vocabulary may change without a compatibility path.** A graph written by
  one release may not read back into the next, and no migration is provided.
  Treat a `.ttl` as an interchange artifact you can regenerate, not as the copy
  of record.
- **Interoperability is not yet demonstrated**, and the gap is measured rather
  than argued. The `sysml:` vocabulary and the `elmt:` element base match Flexo
  MMS's `Namespaces.kt`. OpenSysML's ids (the part of an IRI after the final
  `:`, for elements and expression nodes alike) match that service's
  `requireValidId` (`[a-zA-Z0-9_-]+`). Every element carries the
  `sysml:elementId` that paged listing and query select use, and ownership is
  written as the memberships and owner references the roots endpoint filters on.
  A round trip through a running Flexo MMS stack, measured before that work was
  done, delivered every element of the reference fixture but only 86 of its 142
  properties, while the same model posted through the service's own commit path
  lost nothing. What no test here shows yet is a graph carrying the current output
  loaded into a live service and read back; that is separate work. Known
  mismatches remain: the reader ignores predicates outside `sysml:` and
  `urn:sysmlv2:annotation:json:`, so the 56 `sysx:` properties of that fixture
  do not survive the path, and a standard property carrying more than one value
  is skipped because it has no JSON annotation. The measurement lives in
  `internal/interop/flexo`, an opt-in gate described in
  `.agents/skills/flexo-interop`, and its committed report records what changes
  as the remaining work lands.

Every surface reports this status where it is used: the command line writes a
`note:` to stderr, `%save` prints one, and `ConvertResponse` carries `experimental`
and `experimental_notice`, which the Python client raises as an
`ExperimentalFeatureWarning`. The wording is a single constant,
`export.ExperimentalNotice`.

## The RDF mapping

### Namespaces

| Prefix  | IRI                                            | Holds |
|---------|------------------------------------------------|-------|
| `sysml:` | `https://www.omg.org/spec/SysML#`             | Metaclasses and metamodel properties |
| `elmt:`  | `urn:sysmlv2:element:`                        | The elements of the converted model |
| `sysx:`  | `urn:opensysml:sysml:`                        | The few properties the metamodel does not define |
| `expr:`  | `urn:opensysml:expr:`                         | The expressions an element's positions hold, see [Expressions](#expressions) |
| `rdf:`, `xsd:` | the standard RDF and XML Schema namespaces | `rdf:type`, literal datatypes |

The `sysml:` vocabulary and the `elmt:` element base match the ones the
[Flexo MMS SysML v2 service](https://github.com/Open-MBEE/flexo-mms-sysmlv2)
writes into its triplestore (`Namespaces.kt`). That service derives an
element's `@id` from the substring after the final `:`, and `requireValidId`
permits only `[a-zA-Z0-9_-]+`. OpenSysML's encoded element ids satisfy both, and
so do the `expr:` node ids. (A node's id used to contain a `.`, which that service
refused to read; the position is now joined with `_p` and encoded instead.)
Other mismatches remain: the reader ignores predicates outside `sysml:` and
`urn:sysmlv2:annotation:json:`, so `sysx:` triples do not survive that path, and
collection properties carry no JSON annotation. See
[Status](#status-experimental).

OpenSysML's own additions live in a separate `sysx:` namespace so a consumer can
tell them apart from the standard vocabulary and ignore them if it wants only
standard SysML.

### Element IRIs

An element's IRI is its qualified name, encoded as an id, appended to `elmt:`:

```
package Demo { part def Vehicle; }
```

```turtle
elmt:Demo         a sysml:Package ;
    sysml:qualifiedName "Demo" .
elmt:Demo__Vehicle a sysml:PartDefinition ;
    sysml:qualifiedName "Demo::Vehicle" ;
    sysml:elementId "Demo__Vehicle" ;
    sysml:owner elmt:Demo .
```

The encoding (`rdf.EncodeElementID`) works over the UTF-8 bytes of the
qualified name, with `_` as the escape character:

- the `::` separator becomes `__`
- a byte in `[A-Za-z0-9-]` stands for itself
- every other byte — a literal `_` included — becomes `_` plus two lowercase
  hex digits: `A_B::C` → `A_5fB__C`, `A::B_C` → `A__B_5fC`,
  `Importer::@0` → `Importer___400`, `Vehicle Mass` → `Vehicle_20Mass`

The id therefore always matches `[A-Za-z0-9_-]+`, distinct qualified names
never collide, and `rdf.DecodeElementID` reverses the encoding exactly. The
IRI is **deterministic**: converting the same model twice yields the same
IRIs, and re-converting after an edit leaves the untouched elements at the same
addresses. The id is an address, not the copy of record for the name. The
name is carried by `sysml:qualifiedName`, which is where reading a graph back
takes it from.

### Element identity

The encoded qualified name is only the **derived** id, the one an element gets
when nothing declares one. An `@IdentityMetadata::ElementId` annotation
declares the id explicitly, and then the element's IRI and `sysml:elementId`
carry the declared id instead, so a rename keeps the subject:

```
package Demo {
    @IdentityMetadata::ProjectRef { projectId = "proj-1"; }
    part def Vehicle {
        @IdentityMetadata::ElementId { id = "8f3a41d0"; }
    }
}
```

```turtle
elmt:8f3a41d0 a sysml:PartDefinition ;
    sysml:qualifiedName "Demo::Vehicle" ;
    sysml:elementId "8f3a41d0" ;
    sysx:declaredId "true"^^xsd:boolean .
```

The identity annotations are **consumed into identity** rather than exported as
metadata content, exactly as names are consumed into IRIs. `sysx:declaredId`
records that the id came from an annotation. That fact cannot be recovered from
the value itself, since a declared id may happen to equal the encoding of the
current qualified name, and dropping it would turn the next rename into a delete
plus a create. A membership's id derives from its member's effective id
(`8f3a41d0_om`), and an expression node's from its owner's, so both inherit the
id's stability.

A `@IdentityMetadata::ProjectRef` annotation on a scope root is written as
provenance triples on that root: `sysx:projectId`, `sysx:branch`, `sysx:org`.

A document holding **more than one project scope** qualifies each element's IRI
with its scope's provenance (`elmt:<encoded-org>.<encoded-project>:<id>`), so an
id repeated across scopes stays two subjects; two scopes whose elements would
still land on one IRI are refused rather than silently merged.

Reading a graph back keys the subjects on `sysml:elementId`; the qualified
name is a mutable label. A graph without `sysml:elementId` (from before the
property existed, or from another tool) falls back to the encoding of its
qualified name, which is what its IRIs carry. The notation writer re-materializes an
`@ElementId` annotation wherever the graph marks `sysx:declaredId` true **or**
the id differs from the encoding of the qualified name, and one `@ProjectRef`
per root carrying provenance. Subjects are classified by their `rdf:type`,
never by parsing the id — a declared id may legitimately end in `_om` or embed
`_p` without being a membership or expression node.

### What each element carries

- `rdf:type` — the SysML metaclass (`sysml:PartUsage`, `sysml:ActionDefinition`, …).
  Every definition and usage keyword the parser accepts has a metaclass; the
  tables in `internal/core/export/kinds.go` are the source of truth, and the
  reverse direction is derived from them so the two cannot disagree.
- `sysml:declaredName`, `sysml:declaredShortName`, `sysml:qualifiedName`
- `sysml:elementId` — the id the element's own IRI ends in, which is what the
  SysML v2 API addresses it by. Every element carries one, including the
  memberships below and the expression nodes of
  [Expressions](#expressions)
- Ownership, described under [Ownership](#ownership): `sysml:owner`, plus
  `sysml:owningMembership` and `sysml:owningRelationship`, or
  `sysml:owningRelatedElement` for an element a relationship owns
- `sysml:owningNamespace` — the containing element (absent on a root), kept
  alongside `sysml:owner` as the compact spelling earlier releases wrote
- `sysml:visibility`, `sysml:direction`
- Feature flags, written only when true, so an absent flag reads as false:
  `isAbstract`, `isVariation`, `isVariant`, `isReference`, `isComposite`, `isDerived`,
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
- `sysml:lowerBound`, `sysml:upperBound` — multiplicity, as expression nodes
  ([Expressions](#expressions))
- `sysml:value` — a feature's value, as an expression node
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
| `sysx:endForm` | The notation an end-binding head writes its ends in — `to`, `nary`, `equals`, `firstThen`, `fromTo`, `flowTo`, `satisfy`, `then` — so the head is rebuilt from the graph rather than read back from its text. See [End-binding heads](#end-binding-heads). |
| `sysx:endVerb` | The verb a head writes ahead of its ends when its own keyword is the noun form (`connection c connect a to b`). Without it the verb would be missing or doubled. |
| `sysx:sourceMember`, `sysx:targetMember` | The member a succession sequences from or to where the notation names no end (`then b;`, or a `then` beside an unnamed member). The end is the element itself rather than a name, since there is none to write. |
| `sysx:condition` | The condition a condition member states, as its notation. |
| `sysx:prefixMetadata` | A metadata annotation as written (`#Safety`). It states what the element it prefixes is, and the AST records no span for it, so the notation is read from the source. |
| `sysx:declaredId` | The element's id came from an explicit `@IdentityMetadata::ElementId` annotation, see [Element identity](#element-identity). |
| `sysx:projectId`, `sysx:branch`, `sysx:org` | The `@IdentityMetadata::ProjectRef` provenance of a scope root, see [Element identity](#element-identity). |
| `sysx:isKindImplicit` | The declaration wrote no kind keyword (`in x : Real;`), which takes its kind from its owner. Without it the canonical keyword would come back written out, declaring what the author did not. A kind named in a comment in the head (`in /* attribute */ x : Real;`) is trivia, not a keyword the declaration wrote. |
| the behavioral properties | `sysx:guard`, `sysx:expression`, `sysx:payload`, … — the parts of a behavioral node the vocabulary has no predicate for, listed under [Behavior](#behavior). |

Metaclass names with no counterpart in the OMG vocabulary are typed in the
`sysx:` namespace rather than `sysml:`, so a consumer can tell them from the
standard metaclasses: `sysx:Alias`, `sysx:FilterMember`,
`sysx:MultiplicityDeclaration`, `sysx:ConstraintMember`, `sysx:AssumeMember`,
`sysx:RequireMember`, and the behavioral ones listed under
[Behavior](#behavior).

Comments, documentation and textual representations convert as their own
elements (`sysml:Comment`, `sysml:Documentation`, `sysml:TextualRepresentation`)
carrying `sysml:body`.

### Ownership

The notation states containment by nesting; the abstract syntax states it as a
membership element between the owner and the member, and that is what the SysML
v2 API's payloads carry. The mapping materializes those memberships.

A namespace owning an ordinary member mints one membership element:

```
package Demo { part def Vehicle { attribute mass; } }
```

```turtle
elmt:Demo
    a sysml:Package ;
    sysml:elementId "Demo" ;
    sysml:ownedMember elmt:Demo__Vehicle ;
    sysml:ownedMembership elmt:Demo__Vehicle_om ;
    sysml:ownedRelationship elmt:Demo__Vehicle_om .

elmt:Demo__Vehicle_om
    a sysml:OwningMembership ;
    sysml:elementId "Demo__Vehicle_om" ;
    sysml:owner elmt:Demo ;
    sysml:memberElement elmt:Demo__Vehicle ;
    sysml:ownedMemberElement elmt:Demo__Vehicle ;
    sysml:ownedRelatedElement elmt:Demo__Vehicle ;
    sysml:owningRelatedElement elmt:Demo ;
    sysml:membershipOwningNamespace elmt:Demo .

elmt:Demo__Vehicle
    a sysml:PartDefinition ;
    sysml:elementId "Demo__Vehicle" ;
    sysml:owner elmt:Demo ;
    sysml:owningRelationship elmt:Demo__Vehicle_om ;
    sysml:owningMembership elmt:Demo__Vehicle_om .
```

- A membership's id is the member's id with `_om` appended, which no element id
  can be: an `_` in an element id starts either `__` for `::` or a hex escape.
  It is minted by `rdf.OwningMembershipID`, so it is deterministic and reverses
  to the member's qualified name.
- A **type owning a feature** — a usage or a state inside a definition — mints a
  `sysml:FeatureMembership` instead, and adds `sysml:ownedMemberFeature` and
  `sysml:owningType` on it and `sysml:ownedFeature` and
  `sysml:ownedFeatureMembership` on the owner. `FeatureMembership` specializes
  `OwningMembership`, so the `_om` id and the properties above still apply.
- A **relationship a namespace declares** — an import, a dependency, a state's
  entry membership — is owned directly, with `sysml:owningRelatedElement` on it
  and `sysml:ownedRelationship` on the owner, and no membership between. An
  import also states `sysml:importOwningNamespace` and `sysml:ownedImport`.
- An element a **relationship** owns — the action of an entry membership — states
  the relationship in `sysml:owner` and `sysml:owningMembership`, and the
  relationship states it in `sysml:memberElement`.
- **Visibility belongs to the membership.** `private part wheel;` writes
  `sysml:visibility "private"` on the membership, which is where the metamodel
  declares the property; a relationship that states its own visibility, such as
  an import, keeps it on itself.
- A membership is **not** a declaration of its own: it carries no
  `sysml:qualifiedName`, and reading a graph back traverses it as the ownership
  edge it stands for rather than writing it out. A membership the notation does
  name, such as a `sysml:StateSubactionMembership`, has a qualified name and is
  written back.
- **The compact shape still reads.** `sysml:owningNamespace` is still written,
  and a graph carrying only it — what earlier releases wrote — converts back
  unchanged. A membership that states neither of its ends is reported as
  unsupported naming `sysml:memberElement`, rather than dropping the member.

Tests: `ownership_graph_test.go` (element ids, roots, membership wiring, the
tree coming back from the memberships with `sysx:sourceText` and
`sysml:owningNamespace` stripped, the compact shape, malformed memberships).

## Expressions

An expression-valued position — a feature value, a multiplicity bound, a guard,
a filter, a condition, a send payload, a loop's collection — states the
expression as a **tree of typed nodes** in the `expr:` namespace, so a consumer
can query the model's semantics and not only its structure:

```sysml
package P {
    private import ScalarValues::*;
    attribute a : Integer;
    attribute total : Integer = a * 2;
}
```

```turtle
elmt:P__total
    a sysml:AttributeUsage ;
    sysml:value expr:P__total_pvalue .

expr:P__total_pvalue
    a sysml:OperatorExpression ;
    sysx:sourceText "a * 2" ;
    sysml:elementId "P__total_pvalue" ;
    sysx:operator "*" ;
    sysml:argument expr:P__total_pvalue_pa0, expr:P__total_pvalue_pa1 .

expr:P__total_pvalue_pa0
    a sysml:FeatureReferenceExpression ;
    sysx:sourceText "a" ;
    sysml:referent elmt:P__a ;
    sysx:argumentIndex "0"^^xsd:integer .
```

The rules the tree follows:

- **A node's IRI is built from its owner and its position**: `expr:<owner id>_p<slot>`,
  and a nested operand appends its own index (`_pa0`, `_pa1`). Two expressions of
  one element therefore never collide, and the IRIs are deterministic, like element
  IRIs. The `_p` marker and the encoding of the position keep a node's id inside
  `[A-Za-z0-9_-]+`, the alphabet the SysML v2 API's `requireValidId` accepts, and
  it can never be read as an element id or a membership id, because an element id
  never ends in a lone `_`.
- **Every node carries `sysx:sourceText`**, the notation it was written as. The
  tree is *additive*: the text is what a conversion back to notation is written
  from, so exactness does not depend on the tree being complete, and
  `TestRoundTripIsLossless` covers the same round trip it did before.
- **Operands are ordered** by `sysx:argumentIndex`, because an RDF graph is a set
  and `a - b` is not `b - a`.
- **Metaclasses are the standard ones** where the metamodel names them:
  `LiteralBoolean`, `LiteralInteger`, `LiteralRational`, `LiteralString`,
  `LiteralInfinity`, `NullExpression`, `FeatureReferenceExpression`,
  `FeatureChainExpression`, `OperatorExpression`, `InvocationExpression`,
  `CollectExpression`, `SelectExpression`, `ConstructorExpression`,
  `MetadataAccessExpression`, `Expression` for a body. `sysx:operator`,
  `sysx:argumentIndex` and `sysx:sourceText` are the properties the metamodel
  does not define.
- **A feature reference links to the element** it names (`sysml:referent`) when
  that element is in the graph, and carries its name as a literal when it
  resolves outside it, the same rule the declaration-head relationships follow.
- **A node carries `sysml:elementId`**, the id its own IRI ends in, so it can be
  read and queried by that id like an element. It is still not a model element:
  it has no `sysml:qualifiedName` and no ownership properties, it is reached only
  from the position that holds it, and reading a graph back never turns one into
  a declaration. Writing expressions as elements owned through memberships is
  separate work.
- **A graph from another tool is read from its structure** when it carries no
  `sysx:sourceText`: the supported shapes above are written back as notation, and
  a shape this mapping cannot write (a missing operator, an operand count an
  operator does not take, a literal with no value) is reported as unsupported,
  naming the node, never guessed.
- **Older graphs still read.** A position holding a plain literal
  (`sysml:value "1200.0"`), which is what releases before this wrote, is read as
  that notation.

Tests: `w6g4_rdf_expr_test.go` (structure, ordering, per-position identity,
legacy literals, foreign trees, unsupported shapes, round-trip exactness).

## Behavior

An action or state body converts: each node in it has a metaclass and the
properties its notation is rebuilt from, so `notation → RDF → notation` returns
the body byte for byte (`behavior_test.go`). Where the OMG vocabulary names
the node, that name is used; the rest are `sysx:` terms, marked below.

| written | metaclass | carries |
|---|---|---|
| `first x;`, `first x then y { … }` | `sysx:InitialNode` | `sysml:sourceFeature` (the member the body starts at — a reference, not a name it declares), `sysml:targetFeature`, `sysx:guard`, `sysx:hasBody` and the members of its body |
| `done;` | `sysx:FinalNode` | — |
| `action a;`, `action a { x + 1 }` | `sysx:ActionExecutionNode` | `sysml:references` or `sysx:expression` |
| `perform a;` | `sysml:PerformActionUsage` | `sysx:expression` (the action performed) |
| `assign x := 1;` | `sysml:AssignmentActionUsage` | `sysx:target`, `sysml:value`, `sysx:assignmentOperator` when it is not `:=` |
| `send M(x) to p;`, `… via p;` | `sysml:SendActionUsage` | `sysx:payload`, `sysx:receiver`, `sysx:isVia` |
| `terminate;`, `terminate x;` | `sysml:TerminateActionUsage` | `sysx:expression` |
| `accept sig : Signal;`, `accept when c;` | the usage's own metaclass | `sysml:isAccept`, and `sysx:declaredKeyword "accept"` where the optional `action` was not written |
| `fork`, `join`, `merge`, `decide` | `sysml:ForkNode`, `JoinNode`, `MergeNode`, `DecisionNode` | `sysml:declaredName` |
| `succession first a then b;`, `if g then b;`, `else b;` | `sysml:SuccessionAsUsage` | `sysml:sourceFeature`, `sysml:targetFeature`, `sysx:guard`, `sysx:isElse`, `sysx:declaredKeyword` |
| `while c { … }`, `loop { … } until c;` | `sysml:WhileLoopActionUsage` | `sysx:whileCondition`, `sysx:untilCondition` |
| `for x in c { … }` | `sysml:ForLoopActionUsage` | `sysx:loopVariable`, `sysx:collection` |
| `if c { … } else { … }` | `sysml:IfActionUsage` + `sysx:IfBranch` per branch | `sysx:condition`, `sysx:branchKind` |
| `state s { … }`, `state s parallel { … }`, `entry; then s; state s;` | `sysml:StateUsage` | `sysml:declaredName`, `sysx:declaredKeyword`, `sysml:isParallel`, its members |
| `entry`/`do`/`exit`, `entry do { … }` (whatever separates the `do` from the body) | `sysml:StateSubactionMembership` | `sysx:subactionKind`, `sysx:declaredKeyword`, its actions |
| `defer sig, other;` | `sysx:DeferMember` | `sysx:deferredEvent` per event |
| `choice`, `junction`, `fork`, `join`, `entry point`, `exit point`, `shallow`/`deep history` | `sysx:Pseudostate` | `sysx:pseudostateKind`, `sysx:declaredKeyword` |
| `transition [n] [first] s [accept t] [if g] [do e] then t;` | `sysml:TransitionUsage` | `sysml:sourceFeature`, `sysml:targetFeature`, `sysx:trigger`, `sysx:triggerKeyword`, `sysx:guard`, `sysx:transitionSyntax`, its effect |

A state's members are held in the AST in one bucket per kind (entry, do, exit,
defer, substates); they are written back in the order they were
declared, taken from their source spans, so `do` before `entry` stays that way.

The conditions and expressions these nodes carry are expression trees, like
every other expression-valued position ([Expressions](#expressions)): they
convert back exactly *and* SPARQL can see inside them.

What is still refused, naming the node:

- **A succession that does not name both of its ends.** `then fork;` and
  `then monitorPedal;` written after a preceding member express an order whose
  source end the notation leaves implicit, and the parser records the node the
  statement introduces separately from the edge into it. Reconstructing that
  shape would mean inferring which node an edge belongs to from member position,
  which could silently reattach edges, so it is reported instead. Nine of the
  eighteen remaining refusals under `examples/` are this shape.
- **A succession end whose name is not a basic name.** The two-end form the
  graph is written back as (`succession first a then b;`) is read by the parser
  only when both ends are basic names, so `succession first 'enter vehicle'
  then 'drive vehicle';` would not parse. The edge is reported rather than
  written.
- **Prefix metadata** (`#Safety part p;`, `@M { … }`) and an **operator
  expression member**, both unchanged from before.

## Limitations

These are the constructs the mapping does not fully represent. Each is a
documented limitation, not a silent one: converting an affected element from a
graph that lacks the source text reports an error naming the element rather than
guessing.

**An expression tree is not the metamodel's own expression model.** Feature
values, multiplicity bounds, filter and constraint conditions and guards are
expression trees ([Expressions](#expressions)), which makes them queryable, but
the nodes are not `Feature`s owned through `FeatureMembership`s the way the
abstract syntax models an expression. A conversion back to notation is written
from the text each node was written as. A consumer that wants the metamodel's own
shape does not get it from this mapping.

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
Save straight to `.sysml` when the comments matter; that path writes the source
and keeps everything.

### End-binding heads

**A head that binds ends records the form it writes them in.** A `connect`,
`bind`, `flow`, `succession`, `transition`, `accept` or `satisfy` declaration is
carried as `sysx:sourceText` (the exact text is what a save writes back) and,
beside it, as the structure the head expresses: its ends, and `sysx:endForm`, the
notation those ends are written in. The form is what makes the head
reconstructible without the text, so a graph from another tool converts to
notation as well:

```turtle
elmt:P__Car___402
    a sysml:ConnectionUsage ;
    sysx:sourceText "connect left to right;" ;
    sysx:endForm "to" ;
    sysx:relatedFeature expr:P__Car___402.end0, expr:P__Car___402.end1 .

expr:P__Car___402.end0
    a sysml:FeatureReferenceExpression ;
    sysx:sourceText "left" ;
    sysml:referent elmt:P__Car__left ;
    sysx:endIndex "0"^^xsd:integer .
```

`sysx:relatedFeature` points at one expression node per end, each carrying
`sysx:endIndex` and — for a flow — `sysx:endRole` (`source`, `target`,
`payload`). The forms and what each writes:

| `sysx:endForm` | Notation | Head |
|----------------|----------|------|
| `to` | `<end0> to <end1, …>` | `connect a to b`, `allocate a to b` |
| `nary` | `(<end0>, <end1>, …)` | `connect (a, b, c)` |
| `equals` | `<end0> = <end1>` | `bind a = b` |
| `firstThen` | `<end0> then <end1>` | `succession first a then b` |
| `fromTo` | `[of <payload>] from <end0> to <end1>` | `flow of P from a to b` |
| `flowTo` | `[of <payload>] <end0> to <end1>` | `flow a to b` |
| `satisfy` | `<requirement>` (the `sysml:subsets` end, written bare) | `satisfy R by v`, `verify R` |
| `then` | the source end is the nearest member written before it that is not itself an edge | `then b;`, `then part b;` |

A head whose own keyword is the noun form writes a verb ahead of its ends, and
that verb is `sysx:endVerb` (`connection c connect a to b`). Where the keyword
is a synonym for the kind (`verify` for a satisfy, `allocate` for an
allocation) it is carried as `sysx:declaredKeyword`, as elsewhere.

**The form is only recorded when rebuilding from it reproduces the head as written.**
The encoder writes the ends back from `sysx:endForm` and compares them with the
source before recording it, so a head this mapping cannot rebuild exactly
carries no form and stays readable as text alone. Those are the heads that say
more than their ends: an end with a multiplicity or a `references` clause, an
inline payload declaration (`flow of x : P from a to b`), or a satisfy that
declares a name of its own (`satisfy s : R by v`).
Converting such an element from a graph that carries no `sysx:sourceText` is
reported, not guessed. A graph that relates ends but gives no form at all is
reported the same way (`export_test.go:TestEndsWithoutTheirFormAreReported`).

**The body of such a head is mapped like any other body.** `sysx:sourceText`
carries the head alone — up to its `{` or `;` — and the members written in the
body (`interface seam connect w.outp to r.inp { attribute coupling : C = C::x; }`)
are elements of their own, owned through `sysml:ownedMember`,
`sysml:ownedFeature` and their membership with a `sysx:memberIndex`, with
`sysx:hasBody` stating that a body was written. The same holds for the body an
action's `first a then b { … }` or `then b { … }` carries. The decoder writes
the body from those members whether or not the graph carries the head's text.

Tests: `export_test.go:TestEndBindingHeadsComeBackFromTheGraphAlone`,
`TestEndBindingBodiesComeBackFromTheGraphAlone` and
`TestBehavioralHeadsComeBackFromTheGraphAlone` strip `sysx:sourceText` from the
graph, write the notation back from the mapping alone, and convert it again.
The second graph must equal the first, which is what proves the second hop loses
nothing. `TestBindingEndsAreStatedAsStructure` covers the ends themselves.

**A succession carries its two ends.** Every succession is one node naming
the members it sequences, whether it was written as its own member
(`succession first a then b;`) or attached to one (`then action b : B;`, which
the parser desugars to the same edge), so the order a model declares survives
the hop:

```turtle
elmt:P__Move___402
    a sysml:SuccessionAsUsage ;
    sysx:endForm "then" ;
    sysml:targetFeature elmt:P__Move__c ;
    sysx:sourceMember elmt:P__Move__a .
```

A `then` that names neither end writes `sysx:sourceMember` and
`sysx:targetMember` instead, pointing at the members it sequences, since the
notation gives them no name. That is what carries a `then` beside a member
the notation leaves unnamed (`then send Show(x) to screen;`, a state's
`entry; then s1;`), the shape the parser used to warn about
(`unnamed-succession-end`) and the encoder used to refuse. Both ends are
positions in one body, so writing them back is exact: the source end is the
member before the succession, and a target that *is* that preceding member is
the declaration the `then` was written ahead of.

The member a `then` sequences from is the one the parser gives it: the nearest
member before it that is not itself an edge. A `flow`, `bind`, `connect`,
`succession` or `transition` relates other members rather than declaring one,
so a `then` written after it is read past it, while an `attribute`, a `doc` or
any other declaration is the source. The writer folds a succession back into
`then` by the same rule, shared with the parser as `ast.UsageKind.IsEdge`, so
`action a; flow from a.x to b.x; then action b;` comes back as written. The
source end is compared as the name the member answers to, which is what the
parser records: a `first a then b;` sequences from `a`, and a `perform walk;`
or `action redefines walk;` that declares no name of its own answers to `walk`
(KerML 7.3.4.5). A graph describing a position the notation cannot express —
sequencing from an earlier member, or from the flow the `then` is read past —
is reported rather than written back somewhere else
(`export_test.go:TestUnnamedSuccessionEndComesBackFromTheGraph`,
`TestHalfNamedSuccessionInAGraphIsReported`,
`behavior_test.go:TestThenComesBackPastTheMembersTheParserSkips`,
`TestThenIsRefusedWhenTheGraphSequencesFromAnotherMember`).

Every body that can carry a succession (definition, usage, action, state,
including a parallel state's regions, calculation and requirement) reads these
forms back as the same node, and on the fixtures a second conversion writes the
same Turtle byte for byte (`export_test.go:TestSuccessionRoundTripsInEveryBody`).
That is a statement about the fixtures, not the mapping: over the example corpus
the second hop reproduces the graph exactly for 177 of the 268 files that
convert, up to `sysx:sourceText` whitespace for 72 more, and differs for the
rest ([rdf-corpus-roundtrip.md](../project/rdf-corpus-roundtrip.md)). The
explicit two-ended form reads only basic names, so a succession naming an end
that needs quotes is reported rather than written as notation the parser would
reject.

**A reference end is written back in a spelling the parser reads
differently.** `end [*] ref cause : Situation;` is carried faithfully (the
graph records `sysml:isReference`) and comes back as
`end ref attribute cause : Situation[*];`, but the parser records no reference
flag for a `ref` that follows `end`, so converting *that* notation again drops
the `ref`. The graph is right; a stable second hop needs the parser to record
the flag.

**Conditions convert as their notation.** The members that express
a condition are carried, each as the `sysx:` metaclass named above with its
condition as `sysx:condition`: a constraint body's conditions (`assert`,
`assume`, a bare condition, and the `not` of `assert not …` as
`sysml:isNegated`), a nested `assert constraint [name] { … }`, a requirement's
`assume`/`require` members in all three forms (an expression, the constraint
they name, or a body), `subject s : X;` as the `sysml:SubjectMembership` it
declares. The `assert` prefixing a named usage
(`assert constraint c : C`) is carried as `sysx:declaredPrefix`. The conditions
themselves are notation, with the limits stated above.

The nodes in an action or state body are mapped under
[Behavior](#behavior), together with the shapes still refused there.

**A synonym keyword on a declaration with no name of its own is refused.** `snapshot s;`
shares its AST kind with `occurrence`, and a declaration that carries no name of
its own has nothing for `sysx:declaredKeyword` to attach to, so writing it back as
the canonical `occurrence` would be a different declaration. It is reported
instead. `perform a : A;` does convert: the `perform` is kept as the keyword it
was written with.

**A metadata annotation is carried as the notation it was written as**
(`sysx:prefixMetadata "#Safety"`), read from the source because the AST records
no span for the annotation itself. Two shapes are reported rather than written
back: an annotation carrying a body of its own (`@M { isSet = true; }`), which
the vocabulary has no properties for, and an `@` annotation ahead of a
definition (`@Safety part def Car;`), which the parser records on the
declaration *before* the one it prefixes, so writing it back would annotate a
different element.

**A name declared twice in one namespace is refused.** An element's derived id
is the encoding of its qualified name, so `part def A; part def A;` in one
container would merge into a single subject. The duplicate is reported instead.

A shorthand relationship declares no name of its own: the `result` in `bind result = x;`
and the `x` in `first x;` name the end the statement relates. Those elements are
therefore addressed by position (`sysx:memberIndex`) and the name is carried as a
reference. Without that they would collide with the member they name and the model
would be refused as a duplicate.

**Unsupported on the RDF input side**, each reported as an error naming the line or element:

- blank nodes and `[ ... ]` — every element must have a stable IRI
- RDF collections `( ... )` — order is carried by `sysx:memberIndex`
- an element with no `rdf:type`, or a metaclass outside the mapping
- a reference whose IRI names no subject of the graph and whose id no subject
  carries as `sysml:elementId`; a dangling id is reported as such, never left
  as a silently unresolvable name
- a referenced element with no `sysml:qualifiedName`; the name is read from
  that property, never recovered from the IRI, so a graph with foreign ids
  (UUIDs, say) converts exactly as long as it carries the names
- an element whose `sysml:owningNamespace` is not in the graph
- ownership that forms a cycle, leaving an element no root owns; printing walks
  down from the roots, so this would otherwise write an empty document
- Turtle syntax errors, reported with a line number
- literal shorthands (bare numbers and booleans); literals must be quoted,
  with an `xsd:` datatype where one applies

A graph that uses none of OpenSysML's `sysx:` properties (one produced by
another tool) converts as far as the mapping allows and errors on the first
element it cannot place, rather than emitting a model with elements missing.

## Where the code lives

| Package | Role |
|---------|------|
| `internal/core/rdf` | Triple/graph model, Turtle writer, Turtle parser |
| `internal/core/export` | `ToRDF` (AST → graph), `ToSysML` (graph → notation), and the `Convert` entry point |
| `internal/core/export/corpus_roundtrip_test.go` | The per-file round-trip ratchet over every model under `examples/`, with its baseline in `testdata/corpus_roundtrip_expected.txt` ([rdf-corpus-roundtrip.md](../project/rdf-corpus-roundtrip.md)) |
| `internal/repl` | `%save` |
| `cmd/sysml` | `-convert`, `-from`, `-o` |

The RDF layer is hand-written against the Turtle grammar rather than pulled in
as a dependency: the subset needed here is small, and the parser rejects what it
does not support instead of accepting it and dropping data.
