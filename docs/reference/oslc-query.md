# OSLC Query text

OpenSysML accepts OSLC Query 3.0 text as an element-identification query over
the symbols in a parsed model. It is not an OSLC server: this implementation
does not provide query capabilities, result containers, service providers, or
resource shapes.

## Grammar

The supported filter is a compound term:

```text
term (" and " term)*
term ::= identifier comparison_op value
       | identifier " in " "[" value ("," value)* "]"
comparison_op ::= "=" | "!=" | "<" | ">" | "<=" | ">="
```

Identifiers are prefixed names. Values may be `<uri>`, a prefixed name,
quoted strings (with `\"` and `\\` escapes), `true`, `false`, or a decimal.
The supported typed literal suffixes are `^^xsd:string`, `^^xsd:boolean`,
`^^xsd:integer`, `^^xsd:decimal`, and `^^xsd:double`.

The `oslc.where`, `oslc.select`, `oslc.orderBy`, and `oslc.prefix` parameters
may be supplied as a URL-encoded query-parameter string. Standard decoding
applies: `+` means a space and percent-encoded sequences are decoded. A bare
where-clause remains accepted. `oslc.orderBy` accepts `+` and `-` prefixes.
Selection and ordering are comma-separated.

## Prefixes and properties

`sysml:` and `rdf:` are bound by default to the OMG SysML and RDF namespaces.
`oslc.prefix` adds or rebinds a prefix. An unbound prefix is an error, as is a
bound predicate that is not in this table:

| OSLC predicate | Query property |
| --- | --- |
| `rdf:type` | `@type` |
| `sysml:qualifiedName` | `qualifiedName` and element identity |
| `sysml:name` | `name` |
| `sysml:declaredName` | `declaredName` |
| `sysml:owner` | `owner` |
| `sysml:isAbstract` | `isAbstract` |
| `sysml:type` | `type` |
| `sysml:multiplicityLower` | `multiplicityLower` |
| `sysml:multiplicityUpper` | `multiplicityUpper` |

Unknown properties fail the query instead of silently returning no matches.

## Semantics and choices

Equality, inequality, and `in` compare lexical values. A multi-valued property
matches when any value equals an operand. Ordered comparisons are numeric and
are limited to the two multiplicity properties; `*` is positive infinity for
those existing model values and operands. Ordered comparisons require exactly
one operand.

Results retain declaration order, with parents before their children, unless
`oslc.orderBy` is supplied. `oslc.select` controls the reported property
projection; identity and metamodel type remain present on every result.
`oslc.orderBy` compares multiplicity properties numerically, with `*` as
positive infinity, and compares other properties lexically. Missing values
retain the existing ordering behavior.
For ordered multiplicity comparisons, `*` is positive infinity. For `=`, `!=`,
and `in`, a value `*` is compared lexically.

OSLC compound terms have no `or`, so OSLC text and the structured API Query
surface are deliberately not interchangeable: structured queries retain their
`and`/`or` constraint tree, while OSLC text provides the OSLC grammar and
operators.

## Unsupported constructs

| Construct | Behavior |
| --- | --- |
| `scoped_term` / nested property query | Typed error. The OSLC rationale is to match a resource using a related resource's property; graph-pattern traversal is not implemented by this symbol-index evaluator. |
| Property wildcard (`*` in `oslc.where`, `oslc.select`, or `oslc.orderBy`) | Typed error. Generic property wildcards are not implemented. |
| `oslc.searchTerms` | Typed error. Free-text search is distinct from property identification. |
| Language-tagged literals | Typed error; model properties do not carry language tags. |
| Non-`xsd:` or unsupported `xsd:` datatypes | Typed error rather than a potentially misleading lexical comparison. |

## API and command surfaces

The gRPC `QueryRequest.oslc_query` field carries this text and is mutually
exclusive with the structured `query` field. The service advertises the
`oslc_query` capability in addition to `query`.

The REPL accepts `%query <oslc-query>`. The `sysml` command accepts
`-query <oslc-query>` alongside the other model modes. Both print one matched
element per line as qualified name and metamodel type, followed by selected
properties.
