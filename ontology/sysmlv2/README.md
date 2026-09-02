# SysML v2 ontology modules

The [Open-MBEE SysML v2 RDF ontology](https://github.com/Open-MBEE/sysmlv2-rdf-ontology)
renders the OMG KerML/SysML metamodel as a single OWL file: 172 classes, 411 properties and
7 enumeration datatypes in one 623 KB `SysML.owl`. This directory is that ontology split into
**one Turtle module per package of the normative metamodel**, so a consumer can import
`KerML/Root/Elements` or `SysML/Systems/Requirements` without the rest.

Every file here is generated. Do not edit it: run `make ontology-modules` and commit the result.

## Layout

The module boundaries are the packages of the OMG KerML and SysML XMI (release `20240201`,
the release the ontology was rendered from), not a hand-drawn grouping:

```
KerML.ttl                  imports KerML/Root, KerML/Core, KerML/Kernel
KerML/Root.ttl             imports Elements, Namespaces, Annotations, Dependencies
KerML/Root/Elements.ttl    Element, Relationship and their properties
KerML/Core.ttl             imports Types, Classifiers, Features
KerML/Kernel.ttl           imports the 13 Kernel packages (Classes, Connectors, Expressions, …)
SysML.ttl                  imports SysML/Systems
SysML/Systems.ttl          imports the 21 Systems packages (Parts, Actions, Requirements, …)
SysML/Systems/*.ttl        the SysML metaclasses, one module per package
catalog.tsv                every term → the module that declares it
VERSION                    the pinned sources the modules were generated from
```

A **module** (a leaf package) declares terms. A **layer** (`KerML`, `KerML/Root`, `SysML/Systems`,
…) declares none and imports its children, so importing `KerML/Core` gives the whole Core layer.

Each ontology is named `https://www.omg.org/spec/SysML/modules/<path>`; the terms keep their
upstream IRIs in `https://www.omg.org/spec/SysML#`, so a graph valid against the monolithic
ontology is valid against the union of the modules and vice versa.

## What goes where

- A class or enumeration goes to the package that owns it in the XMI.
- A property goes to the package of its `rdfs:domain` class — `sysml:Element_owner` is in
  `KerML/Root/Elements` with `sysml:Element`. The catalog resolves any term.
- Blank-node content (OWL restrictions, cardinalities, enumeration value lists) stays with the
  named subject it hangs from.
- The upstream `owl:Ontology` header is the only thing not in a module; each module states its
  own header with `owl:versionInfo`, `dc:source` and `owl:imports`.

Every source triple lands in exactly one module. The generator fails, rather than dropping or
guessing, on a term the metamodel does not place or a term a module uses that no module declares.

## Imports

A module imports every module that declares a term it refers to (a superclass, a range, a
property under a restriction), plus Ecore when it uses `ecore:isOrdered`. So a module's import
closure always contains every term it mentions. The metamodel is not a strict hierarchy, so
imports are cyclic in places (`KerML/Root/Elements` and `KerML/Root/Namespaces` refer to each
other); OWL 2 allows this and every reasoner handles it, but a consumer wanting one package's
terms only should read the file directly rather than its closure.

## Regenerating

```bash
make ontology-modules         # fetch pinned sources, regenerate, verify
make ontology-modules-check   # CI: fail if the committed modules are stale
```

`scripts/download-ontology-sources.sh` fetches the ontology at a pinned commit and the two OMG XMI
files by pinned SHA-256 into `build/ontology-sources/` (not committed; third-party content stays
out of the tree). `cmd/ontology-modules` reads them and writes here; `-check` compares instead of
writing. To move to a newer ontology, change the pins in the script, regenerate, and review the
diff — the class set of the OWL and of the XMI release must agree or the generator reports the
terms it cannot place.

The implementation is `internal/core/rdf/ontology/modules`: a strict RDF/XML reader for the
constructs `SysML.owl` uses, the XMI package map, the partitioner, and a deterministic Turtle
writer.
