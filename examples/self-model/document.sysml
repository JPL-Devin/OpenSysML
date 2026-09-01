// The architecture document itself, written in the notation and rendered by
// OpenSysML's own document generator (see the README for the command).
package OpenSysMLDocument {
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    private import ScalarValues::*;

    // The document's query roots: a table's query is bound to a usage, so the
    // containers it tabulates are named here as usages.
    part analysisPipeline : OpenSysMLPipeline::AnalysisPipeline;
    part passRegistry : OpenSysMLPipeline::PassRegistry;

    // --- Queries the document runs over this model -----------------------

    // A usage's definition, so a query can reach the parts declared in it.
    calc def DefinitionOf :> Query {
        in root : Element;
        RelatedElements(
            source = root,
            relationshipKind = "typing",
            direction = "outgoing",
            maxDepth = 1
        )
    }

    // Every unit a container declares, with the directory implementing it;
    // the container's channels carry no package and drop out here.
    calc def UnitsWithPackages :> Query {
        in root : Element;
        Project(
            source = WhereFeature(
                source = WhereType(
                    source = OwnedElements(source = DefinitionOf(root = root)),
                    type = "PartUsage"
                ),
                'feature' = "goPackage",
                operator = "!=",
                value = ""
            ),
            properties = ("name", "goPackage")
        )
    }

    // The validation passes, with the tier each one runs at.
    calc def PassesWithTiers :> Query {
        in root : Element;
        Project(
            source = WhereType(
                source = OwnedElements(source = DefinitionOf(root = root)),
                type = "PartUsage"
            ),
            properties = ("name", "level", "elementScoped")
        )
    }

    // The conformance oracles, with the baseline each one ratchets.
    calc def OraclesWithBaselines :> Query {
        in root : Element;
        Project(
            source = WhereFeature(
                source = WhereType(
                    source = OwnedElements(source = DefinitionOf(root = root)),
                    type = "PartUsage"
                ),
                'feature' = "baseline",
                operator = "!=",
                value = ""
            ),
            properties = ("name", "reference", "baseline")
        )
    }

    // --- The document ----------------------------------------------------

    part def ArchitectureDocument :> Document {
        attribute redefines title = "OpenSysML Architecture";

        part summary : Paragraph {
            attribute redefines text = "OpenSysML analyses SysML v2 and KerML in one pass from text to a runnable graph. This document is generated from a model of that architecture rather than written alongside it: every diagram and table below is rendered from the same model the toolchain analyses, so a stage that moves in the code moves here too.";
        }

        part reading : Paragraph {
            part lead : Span {
                attribute redefines text = "The model lives in";
            }
            part source : Link {
                attribute redefines text = "examples/self-model";
                attribute redefines target = "https://github.com/Open-MBEE/OpenSysML/tree/main/examples/self-model";
            }
            part rest : Span {
                attribute redefines text = "and the prose reference it illustrates is docs/internals/architecture.md.";
            }
        }

        part pipeline : Section {
            attribute redefines title = "The analysis pipeline";

            part intro : Paragraph {
                attribute redefines text = "Text becomes tokens, a tree, a symbol index, resolved names, types, diagnostics and finally an execution graph. Each stage owns one artifact and hands it on; nothing downstream reaches back.";
            }

            part structure : Diagram {
                attribute redefines caption = "The stages and the artifacts that travel between them";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::pipelineStructure;
            }

            part stages : Table {
                attribute redefines caption = "Each stage and the package that implements it";
                calc rows : UnitsWithPackages {
                    in redefines root = analysisPipeline;
                }
            }

            part immutable : Paragraph {
                attribute redefines text = "The tree is immutable once parsed and carries syntax only. Everything derived from it lives in a side table keyed by node or symbol, which is why resolution can be lazy and memoized without a stale tree to invalidate.";
            }
        }

        part checking : Section {
            attribute redefines title = "Validation and execution";

            part tiers : Paragraph {
                attribute redefines text = "Validation is a ladder: syntax, name resolution, type, then constraint. A tier runs only where the tier below it is clean, per element rather than per file, so one broken declaration does not silence the rest of the document.";
            }

            part tierStates : Diagram {
                attribute redefines caption = "The tier ladder, and where it halts";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::tierStates;
            }

            part passes : Table {
                attribute redefines caption = "The validation passes, their tiers, and which one gates itself per element";
                calc rows : PassesWithTiers {
                    in redefines root = passRegistry;
                }
            }

            part executing : Paragraph {
                attribute redefines text = "What runs is the lowered graph, never the syntax tree: actions execute as Petri-net token flow and states as run-to-completion event dispatch, both under a step budget, so a model that cannot finish reports budget exhaustion rather than hanging the tool.";
            }

            part analysed : Diagram {
                attribute redefines caption = "One document analysed end to end";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::analysisFlow;
            }
        }

        part surfaces : Section {
            attribute redefines title = "Surfaces over one pipeline";

            part intro : Paragraph {
                attribute redefines text = "The REPL, the language server, the gRPC service and the command line are four surfaces over one analysis pipeline, and the generated Python, Node, Rust and Java clients speak to the service rather than to the engine, so a rule fixed in the engine is fixed everywhere at once.";
            }

            part toolchain : Diagram {
                attribute redefines caption = "The whole toolchain";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::toolchainStructure;
            }
        }

        part documents : Section {
            attribute redefines title = "Documents and views";

            part intro : Paragraph {
                attribute redefines text = "A document is written in the notation: its queries compile to an immutable plan, the plan evaluates to a backend-agnostic tree carrying provenance on every node, and the output form is chosen last, so Markdown and PDF cannot disagree about the content. This document is one of them.";
            }

            part path : Diagram {
                attribute redefines caption = "From a query in the model to a rendered artifact";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::documentPipeline;
            }

            part branches : Diagram {
                attribute redefines caption = "The two branches in rendering a document";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::documentFlow;
            }
        }

        part identity : Section {
            attribute redefines title = "Element identity";

            part intro : Paragraph {
                attribute redefines text = "An element's repository identity is an annotation, not a field on the tree: an explicit id when one is declared, otherwise one derived from the qualified name. The constraint tier checks the whole generated id space for collisions, and the RDF writer and reader carry the distinction between a declared and a derived id across a round trip.";
            }

            part path : Diagram {
                attribute redefines caption = "The identity path, from the library to RDF and back";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::identityPath;
            }

            part roundTrip : Diagram {
                attribute redefines caption = "Carrying an identity through a graph and back";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::identityRoundTrip;
            }

            part unbuilt : Paragraph {
                attribute redefines text = "Repository synchronisation is modelled and not implemented; the model says so, so this document cannot claim otherwise.";
            }
        }

        part quality : Section {
            attribute redefines title = "Invariants and oracles";

            part intro : Paragraph {
                attribute redefines text = "The architecture rules are requirements over the modelled parts, and the test suites that verify them are named in the model, so each rule points at the run that would catch its violation.";
            }

            part invariants : Diagram {
                attribute redefines caption = "The architecture invariants and the parts they constrain";
                ref redefines source = OpenSysMLViews::invariantTable;
            }

            part oracles : Table {
                attribute redefines caption = "The conformance oracles and the baselines they ratchet";
                calc rows : OraclesWithBaselines {
                    in redefines root = OpenSysMLSurfaces::opensysml;
                }
            }
        }
    }
}
