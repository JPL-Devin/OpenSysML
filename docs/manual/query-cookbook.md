# Query Cookbook

Every recipe in this chapter runs against one model,
[`examples/cookbook.sysml`](examples/cookbook.sysml), and every output shown
is what the `sysml` binary printed. The model is a small observatory:

```sysml
package Cookbook {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}
	part def OpticalSubsystem :> Subsystem;
	part def MirrorAssembly :> OpticalSubsystem {
		attribute :>> mass = 10.0;
	}

	metadata def Critical;

	port def OpticalPort;
	port def DataPort;

	part telescope {
		part primaryMirror : MirrorAssembly {
			@Critical;
			port opticalOut : OpticalPort;
		}
		part instrumentCluster : Subsystem {
			attribute redefines mass = 4.5;
			port opticalIn : OpticalPort;
			port dataOut : DataPort;
		}
		part mountControl : Subsystem {
			attribute redefines mass = 15.0;
			port dataIn : DataPort;
		}
		connection opticalPath connect primaryMirror.opticalOut to instrumentCluster.opticalIn;
		connection dataPath connect instrumentCluster.dataOut to mountControl.dataIn;
	}

	part def Computer;
	part scienceComputer : Computer;
	allocation processing allocate telescope.instrumentCluster to scienceComputer;

	requirement massRequirement;
	part observatory {
		satisfy massRequirement by telescope;
	}

	verification def MassTest;
	verification massVerification : MassTest {
		objective {
			verify massRequirement;
		}
	}

	// ... the recipe queries below ...
}
```

Each recipe is a `calc def` specializing `DocumentQueries::Query` declared in
the same package. Run one with:

```console
$ sysml docs/manual/examples/cookbook.sysml -run-query "<name> [<parameter>=<expression> ...]"
```

A binding whose expression is a name binds the element it denotes; anything
else is evaluated as an expression (strings in quotes, numbers as literals).

## Anatomy of a query

```sysml
calc def MassTable :> Query {
	in root : Element;                 // entry parameters, bound by the caller
	Project(                            // operations compose inside-out
		source = PartsByMass(root = root),  // ... and queries invoke queries
		properties = ("name", "mass", "qualifiedName")
	)
}
```

- A query is a `calc def` specializing `DocumentQueries::Query`.
- Its `in` parameters are the entry bindings a caller supplies — an element,
  a string, a number, a boolean, or a sequence of them.
- Its body is one expression composing the library operations; `source`
  arguments chain them, innermost first. A name in an argument reads the
  query's parameter of that name, or binds the model element it refers to
  (`OwnedElements(source = telescope)` starts from that part), just as a
  `%run-query` binding does. The element is checked against the parameter's
  type when the query is planned.
- A query can invoke another query by name, with its own bindings. Invocation
  is dependency-ordered and cycle-checked, with depth and count budgets.

Results are ordered element sequences. Order is the model's declaration order
until an `OrderBy` says otherwise, and elements are deduplicated by identity,
so a query is deterministic by construction.

## Parameter defaults

An `in` parameter may declare a default, and a caller that leaves it unbound
gets that default — from `%run-query`, `-run-query`, `RunDocumentQuery`, a
document's content block, or another query's invocation alike:

```sysml
calc def HeavySubsystems :> Query {
	in root : Element = telescope;         // a name binds the element it refers to
	in threshold : String default "10";    // anything else is evaluated
	WhereFeature(
		source = Descendants(source = root, maxDepth = 3),
		'feature' = "mass", operator = ">=", value = threshold
	)
}

calc def LightSubsystems :> HeavySubsystems {
	in redefines threshold default "5";    // a redefining default wins
}
```

- A default follows the binding rule of `%run-query <p>=<expr>`: a default that
  names a model element binds that element; any other default is an expression.
  The rule applies wherever a value is expected, so `in roots : Element[0..*] =
  (telescope, groundStation);` binds both elements, a list may mix element names
  with parameters and query invocations, and `in candidates : Element[0..*] =
  OwnedElements(source = telescope);` starts the traversal from that part.
- An expression default is evaluated once per query execution, before any row is
  produced, in the scope of the query that declared it — it may name that
  query's other parameters (`in candidates : Element[0..*] = OwnedElements(source = root);`)
  or invoke another query, within the usual visit and invocation budgets.
  Defaults are filled in parameter order after the explicit bindings, so a
  default may read a parameter bound explicitly or defaulted before it; one that
  reads a later, still unbound parameter fails as a missing binding.
- Defaults are inherited: `LightSubsystems` keeps `root = telescope` from
  `HeavySubsystems`, and its own `threshold` default replaces the inherited one.
  The nearest default along the redefinition chain wins.
- The value a default produces is checked against the parameter's type and
  multiplicity exactly like an explicit binding, and an explicit binding always
  overrides the default.
- A default the plan cannot represent (a form the query expression language has
  no operation for) is a planning error naming the parameter, reported with the
  other `document-query-*` diagnostics rather than at execution time.

## Collection

### Direct children: `OwnedElements`

```sysml
calc def Children :> Query {
	in root : Element;
	OwnedElements(source = root)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Children root=Cookbook::telescope"
✓ Query Cookbook::Children returned 5 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::instrumentCluster
  Row 3: Cookbook::telescope::mountControl
  Row 4: Cookbook::telescope::opticalPath
  Row 5: Cookbook::telescope::dataPath
```

Everything the element owns is returned — here the three parts *and* the two
connections, in declaration order. Filter afterwards to narrow.

### Descendants to a depth: `Descendants`

```sysml
calc def AllParts :> Query {
	in root : Element;
	WhereType(
		source = Descendants(source = root, maxDepth = 10),
		type = "PartUsage"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::AllParts root=Cookbook::telescope"
✓ Query Cookbook::AllParts returned 5 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::instrumentCluster
  Row 3: Cookbook::telescope::mountControl
  Row 4: Cookbook::telescope::opticalPath
  Row 5: Cookbook::telescope::dataPath
```

`maxDepth` bounds the walk; each level is visited in declaration order.
Note that the connections are still here: a `connection` usage *is* a
`PartUsage` in the SysML metamodel (its metaclass conforms to it). Use
a feature or name filter, or `type = "ConnectionUsage"`, to separate them —
see [Type filters](#type-filters).

### Ancestors: `Ancestors`

```sysml
calc def Enclosing :> Query {
	in leaf : Element;
	Ancestors(source = leaf, maxDepth = 2)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Enclosing leaf=Cookbook::telescope::primaryMirror::opticalOut"
✓ Query Cookbook::Enclosing returned 2 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope
```

Owners are returned nearest-first, up to `maxDepth` levels.

## Type filters

`WhereType` keeps elements whose *metamodel* type matches — `"PartUsage"`,
`"ConnectionUsage"`, `"RequirementUsage"`, `"AttributeUsage"`, `"PortUsage"`,
`"PartDefinition"` and so on — including metaclass conformance, so
`type = "Usage"` keeps every kind of usage. A name that is neither a known
metamodel type nor resolvable in the model is a typed
`unknown-classification` error rather than a silently-empty result.

```sysml
calc def Connections :> Query {
	in root : Element;
	WhereType(
		source = Descendants(source = root, maxDepth = 10),
		type = "ConnectionUsage"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Connections root=Cookbook::telescope"
✓ Query Cookbook::Connections returned 2 rows
  Row 1: Cookbook::telescope::opticalPath
  Row 2: Cookbook::telescope::dataPath
```

To select by a *model-defined* classification — "every part typed by
`Subsystem`" — filter on what distinguishes those elements instead: a
metadata annotation ([below](#metadata-filters)) or a characteristic
attribute ([property filters](#property-filters)).

## Metadata filters

`WhereMetadata` keeps elements annotated with a metadata definition, matching
specializations of it too. The model marks `primaryMirror` with
`@Critical`:

```sysml
calc def CriticalParts :> Query {
	in root : Element;
	WhereMetadata(
		source = AllParts(root = root),
		'metadata' = "Cookbook::Critical"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::CriticalParts root=Cookbook::telescope"
✓ Query Cookbook::CriticalParts returned 1 row
  Row 1: Cookbook::telescope::primaryMirror
```

(`'metadata'` is quoted because `metadata` is a SysML keyword.)

## Name filters

`WhereName` compares each element's effective name against a value:

```sysml
calc def MirrorParts :> Query {
	in root : Element;
	WhereName(
		source = AllParts(root = root),
		operator = "contains",
		value = "Mirror"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::MirrorParts root=Cookbook::telescope"
✓ Query Cookbook::MirrorParts returned 1 row
  Row 1: Cookbook::telescope::primaryMirror
```

Text operators: `=`/`==`, `!=`/`<>`, `contains`, `startsWith`, `endsWith`
(also spelled `starts-with`/`ends-with`), and `matches` with a regular
expression.

## Property filters

`WhereFeature` compares an attribute's constant value. The comparison is
typed: numbers compare numerically (`<`, `<=`, `>`, `>=` and equality, with
`*` accepted as infinity), booleans by equality, strings with the text
operators above. An element without the attribute simply does not match; a
property no element in the source has is a typed `unknown-property` error.

```sysml
calc def HeavyParts :> Query {
	in root : Element;
	in threshold : String;
	WhereFeature(
		source = AllParts(root = root),
		'feature' = "mass",
		operator = ">=",
		value = threshold
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::HeavyParts root=Cookbook::telescope threshold=\"10\""
✓ Query Cookbook::HeavyParts returned 2 rows
  Row 1: Cookbook::telescope::primaryMirror
  Row 2: Cookbook::telescope::mountControl
```

Two details worth noting: `value` is always written as a string and parsed by
the operator's type, and `primaryMirror` matches through its *definition* —
`MirrorAssembly` fixes `mass = 10.0`, and the usage inherits it.

## Sorting

`OrderBy` sorts by a property with every policy explicit — there are no
defaults to guess:

- `direction`: `"ascending"` or `"descending"`.
- `missing`: where elements without the property go — `"first"`, `"last"`,
  or `"error"` to refuse them.
- `multiple`: which value to sort by when the property has several —
  `"first"`, `"last"`, or `"error"`.

The sort is stable, so equal keys keep their declaration order. Mixing
incomparable value types across elements is a typed `invalid-order` error.

```sysml
calc def PartsByMass :> Query {
	in root : Element;
	OrderBy(
		source = AllParts(root = root),
		property = "mass",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::PartsByMass root=Cookbook::telescope"
✓ Query Cookbook::PartsByMass returned 5 rows
  Row 1: Cookbook::telescope::mountControl
  Row 2: Cookbook::telescope::primaryMirror
  Row 3: Cookbook::telescope::instrumentCluster
  Row 4: Cookbook::telescope::opticalPath
  Row 5: Cookbook::telescope::dataPath
```

The connections have no `mass`, so `missing = "last"` places them after the
sorted parts.

## Projection

`Project` turns elements into rows of named, typed cells — what a document
table renders. Beyond the model's own attributes, these built-in properties
are always projectable:

| Property | Value |
|---|---|
| `name` | The effective name |
| `declaredName` | The declared name, absent when the name is derived |
| `qualifiedName`, `@id` | The fully-qualified name |
| `owner` | The owner's qualified name |
| `@type` | The metamodel type (`PartUsage`, ...) |
| `type` | The declared type's qualified name |
| `isAbstract` | Boolean |
| `multiplicityLower`, `multiplicityUpper` | Integers, `*` as unbounded |

```sysml
calc def MassTable :> Query {
	in root : Element;
	Project(
		source = PartsByMass(root = root),
		properties = ("name", "mass", "qualifiedName")
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::MassTable root=Cookbook::telescope"
✓ Query Cookbook::MassTable returned 5 rows
  Columns: name, mass, qualifiedName
  Row 1: Cookbook::telescope::mountControl
    name = "mountControl"
    mass = 15.0
    qualifiedName = "Cookbook::telescope::mountControl"
  Row 2: Cookbook::telescope::primaryMirror
    name = "primaryMirror"
    mass = 10.0
    qualifiedName = "Cookbook::telescope::primaryMirror"
  Row 3: Cookbook::telescope::instrumentCluster
    name = "instrumentCluster"
    mass = 4.5
    qualifiedName = "Cookbook::telescope::instrumentCluster"
  Row 4: Cookbook::telescope::opticalPath
    name = "opticalPath"
    mass = (none)
    qualifiedName = "Cookbook::telescope::opticalPath"
  Row 5: Cookbook::telescope::dataPath
    name = "dataPath"
    mass = (none)
    qualifiedName = "Cookbook::telescope::dataPath"
```

A cell for a property the element lacks is empty (`(none)` in the CLI's row
listing, an empty table cell in a document).

## Computed columns

A projection may also derive columns: each `Column(name, expression)` entry
appends a named column whose expression is evaluated once per row over the
row element's declared features. Arithmetic (`+`, `-`, `*`, `/`), string
concatenation with `+` and `??` defaults for absent values are supported:

```sysml
calc def MassBudget :> Query {
	in root : Element;
	Project(
		source = PartsByMass(root = root),
		properties = ("name", "mass"),
		columns = (
			Column(name = "massLbs", expression = (Subsystem::mass ?? 0.0) * 2.2),
			Column(name = "label", expression = "part: " + Element::name)
		)
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::MassBudget root=Cookbook::telescope"
✓ Query Cookbook::MassBudget returned 5 rows
  Columns: name, mass, massLbs, label
  Row 1: Cookbook::telescope::mountControl
    name = "mountControl"
    mass = 15.0
    massLbs = 33.0
    label = "part: mountControl"
  ...
```

Feature references name the declaring definition (`Subsystem::mass`,
`Element::name`); a row element that lacks the feature makes the expression
fail with a typed error naming the query, column and row — unless a `??`
default covers it, which is why `MassBudget`'s `massLbs` defaults to `0.0`
for the two connections in its results. Computed names join the projection:
`OrderBy` can sort by them and a table's `groupBy` can group by them.

## Query invokes query

`MassTable` above already shows it: `PartsByMass(root = root)` invokes the
other query with its own bindings, and `AllParts` invokes `Children`'s
sibling the same way. Factoring collection into one base query and deriving
filtered/sorted/projected variants from it is the intended style. The engine
compiles the invocation graph up front: an unknown name, a cycle, or blowing
the depth/count budget is a typed error at that point.

## Relationship traversal

`RelatedElements` walks one named relationship kind from each source element:

```sysml
RelatedElements(
	source = <elements>,
	relationshipKind = "<kind>",   // specialization, subsetting, redefinition,
	                                // typing, connection, allocation,
	                                // satisfaction or verification
	direction = "<direction>",     // outgoing or incoming
	maxDepth = <n>
)
```

Direction is from the relationship's own point of view — `outgoing` follows
it as declared, `incoming` follows it backwards. Traversal is breadth-first
to `maxDepth`, deduplicated, in declaration order, and bounded by a visit
budget so a pathological model terminates with a typed error rather than
hanging.

### Connections

Connection edges run **port to port** — traverse from the connector's
endpoint, not from the part that owns it:

```sysml
calc def ConnectedTo :> Query {
	in origin : Element;
	RelatedElements(
		source = origin,
		relationshipKind = "connection",
		direction = "outgoing",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::ConnectedTo origin=Cookbook::telescope::primaryMirror::opticalOut"
✓ Query Cookbook::ConnectedTo returned 1 row
  Row 1: Cookbook::telescope::instrumentCluster::opticalIn
```

`outgoing` follows `connect A to B` from A's endpoint to B's; `incoming`
follows it the other way. Untyped `connect` clauses carry connection edges
too.

### Allocations

`allocate X to Y` is outgoing from X, incoming to Y:

```sysml
calc def AllocatedTargets :> Query {
	in origin : Element;
	RelatedElements(
		source = origin,
		relationshipKind = "allocation",
		direction = "outgoing",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::AllocatedTargets origin=Cookbook::telescope::instrumentCluster"
✓ Query Cookbook::AllocatedTargets returned 1 row
  Row 1: Cookbook::scienceComputer
```

### Satisfy relationships

`satisfy R by P` points from the satisfying element to the requirement, so
"who satisfies this requirement" is an **incoming** traversal from the
requirement:

```sysml
calc def SatisfiedBy :> Query {
	in req : Element;
	RelatedElements(
		source = req,
		relationshipKind = "satisfaction",
		direction = "incoming",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::SatisfiedBy req=Cookbook::massRequirement"
✓ Query Cookbook::SatisfiedBy returned 1 row
  Row 1: Cookbook::telescope
```

### Verify relationships

Likewise, "which verifications cover this requirement" is incoming from the
requirement; the result is the verification usage whose objective `verify`s
it:

```sysml
calc def VerifiedBy :> Query {
	in req : Element;
	RelatedElements(
		source = req,
		relationshipKind = "verification",
		direction = "incoming",
		maxDepth = 1
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::VerifiedBy req=Cookbook::massRequirement"
✓ Query Cookbook::VerifiedBy returned 1 row
  Row 1: Cookbook::massVerification
```

### Specialization (and the other structural kinds)

`specialization`, `subsetting`, `redefinition` and `typing` traverse the
declaration hierarchy. Incoming specialization from a general type finds what
specializes it, transitively to `maxDepth`:

```sysml
calc def Specializers :> Query {
	in general : Element;
	RelatedElements(
		source = general,
		relationshipKind = "specialization",
		direction = "incoming",
		maxDepth = 2
	)
}
```

```console
$ sysml cookbook.sysml -run-query "Cookbook::Specializers general=Cookbook::Subsystem"
✓ Query Cookbook::Specializers returned 2 rows
  Row 1: Cookbook::OpticalSubsystem
  Row 2: Cookbook::MirrorAssembly
```

Traversal results are elements like any others — feed them into `Project` for
a traceability table, as the [worked example](worked-example.md) does for its
requirement section.
