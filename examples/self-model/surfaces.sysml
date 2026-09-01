// What sits on top of the pipeline: the interactive surfaces, the client
// libraries that reach the service, and the artifacts the tool writes out.
package OpenSysMLSurfaces {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLPipeline::*;

    abstract part def Interface :> CodeUnit {
        attribute entrypoint : String;
        attribute protocol : String;
        port pipeline : RequestPort;
    }

    #Surface part def Repl :> Interface {
        attribute :>> goPackage = "internal/repl";
        attribute :>> entrypoint = "cmd/sysml";
        attribute :>> protocol = "terminal";
        attribute stateful : Boolean = true;
        attribute adoptsInstancesOnReanalysis : Boolean = true;
    }

    #Surface part def LanguageServer :> Interface {
        attribute :>> goPackage = "internal/lsp";
        attribute :>> entrypoint = "cmd/sysml-lsp";
        attribute :>> protocol = "lsp-stdio";
        attribute incrementalSync : Boolean = true;
        attribute publishesQuickFixes : Boolean = true;
    }

    #Surface part def ModelService :> Interface {
        attribute :>> goPackage = "internal/grpc";
        attribute :>> entrypoint = "cmd/sysml-grpc";
        attribute :>> protocol = "grpc, connect";
        attribute reflectionEnabled : Boolean = true;
    }

    // The non-interactive side of the REPL binary: convert, query, render.
    #Surface part def CommandLine :> Interface {
        attribute :>> goPackage = "internal/repl";
        attribute :>> entrypoint = "cmd/sysml";
        attribute :>> protocol = "flags";
        attribute exitStatusIsContract : Boolean = true;
    }

    // Client libraries generated from the same protobuf schema.
    abstract part def Client :> CodeUnit {
        attribute implementationLanguage : String;
        attribute generatedFrom : String = "api/proto";
        port service : RequestPort;
    }

    part def PythonClient :> Client {
        attribute :>> goPackage = "clients/python";
        attribute :>> implementationLanguage = "python";
    }

    part def NodeClient :> Client {
        attribute :>> goPackage = "clients/node";
        attribute :>> implementationLanguage = "typescript";
    }

    part def RustClient :> Client {
        attribute :>> goPackage = "clients/rust";
        attribute :>> implementationLanguage = "rust";
    }

    part def JavaClient :> Client {
        attribute :>> goPackage = "clients/java";
        attribute :>> implementationLanguage = "java";
    }

    part def VsCodeExtension :> CodeUnit {
        attribute :>> goPackage = "editors/vscode";
        attribute speaksTo : String = "sysml-lsp";
        port languageClient : RequestPort;
    }

    // --- What the tool writes out ----------------------------------------

    // Views render as trees, interconnection, state, action and table forms.
    part def ViewEngine :> CodeUnit {
        attribute :>> goPackage = "internal/core/view";
        attribute renderingKinds : Integer = 5;
        in item types : TypeTable;
        out item diagram : DocumentTree;
    }

    part def QueryCompiler :> CodeUnit {
        attribute :>> goPackage = "internal/core/queryplan";
        attribute immutablePlans : Boolean = true;
    }

    part def QueryExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/core/queryexec";
        attribute orderedRows : Boolean = true;
        out item rows : QueryRows;
    }

    // A document is written in the notation, not in a template language: the
    // planner compiles a document definition into an immutable plan holding
    // compiled query programs.
    part def DocumentPlanner :> CodeUnit {
        attribute :>> goPackage = "internal/core/docplan";
        attribute libraryFile : String =
            "internal/core/libs/stdlib/OpenSysML Libraries/DocumentQueries.sysml";
        attribute immutablePlans : Boolean = true;
        attribute contentKinds : String = "section, paragraph, table, list, diagram";
        in item rows : QueryRows;
        out item plan : DocumentPlan;
    }

    // Evaluating a plan gives one tree, carrying provenance on every node and
    // knowing nothing about the backend that will write it out.
    part def DocumentIR :> CodeUnit {
        attribute :>> goPackage = "internal/core/docir";
        attribute backendAgnostic : Boolean = true;
        attribute provenancePerNode : Boolean = true;
        attribute resolvesCrossDocumentRefs : Boolean = true;
        in item plan : DocumentPlan;
        out item tree : DocumentTree;
    }

    part def MarkdownBackend :> CodeUnit {
        attribute :>> goPackage = "internal/core/docrender";
        attribute form : String = "markdown";
        attribute needsExternalTool : Boolean = false;
        in item tree : DocumentTree;
    }

    // The only stage that leaves the process: the tree becomes prepared
    // Markdown, its diagrams become images, and a converter makes the PDF.
    part def PdfBackend :> CodeUnit {
        attribute :>> goPackage = "internal/docpdf";
        attribute form : String = "pdf";
        attribute engines : String = "weasyprint, pandoc, prince";
        attribute diagramTool : String = "mmdc";
        attribute needsExternalTool : Boolean = true;
        attribute reportsMissingTool : Boolean = true;
        in item tree : DocumentTree;
    }

    // Textual notation both ways, plus experimental RDF.
    part def Exporter :> CodeUnit {
        attribute :>> goPackage = "internal/core/export";
        attribute formats : String = "sysml, kerml, ttl";
        attribute roundTrips : Boolean = true;
        out item graph : RdfGraph;
    }

    part def OslcQuery :> CodeUnit {
        attribute :>> goPackage = "internal/core/query";
        attribute dialect : String = "oslc";
    }

    // The document path, from a query in the model to a rendered artifact.
    // One plan, one tree, two backends: the form is chosen at the end, so
    // Markdown and PDF cannot disagree about the document's content.
    part def DocumentPipeline {
        part views : ViewEngine;
        part queries : QueryCompiler;
        part rows : QueryExecutor;
        part plans : DocumentPlanner;
        part ir : DocumentIR;
        part markdown : MarkdownBackend;
        part pdf : PdfBackend;

        port modelIn : ArtifactPort;
        port artifactOut : ArtifactPort;

        flow of QueryRows from rows.rows to plans.rows;
        flow of DocumentPlan from plans.plan to ir.plan;
        flow of DocumentTree from ir.tree to markdown.tree;
        flow of DocumentTree from ir.tree to pdf.tree;
        flow of DocumentTree from views.diagram to pdf.tree;
    }

    // One document rendered, and the one branch in it: the form.
    action def RenderDocument {
        attribute form : String = "markdown";
        attribute toolsPresent : Boolean = true;

        in item model : SourceText;
        out item artifact : DocumentTree;

        first start;

        action runQueries;
        action compilePlan;
        action evaluatePlan;
        action writeMarkdown;
        action renderDiagrams;
        action convertPdf;
        action reportMissingTool;

        succession first start then runQueries;
        succession first runQueries then compilePlan;
        succession first compilePlan then evaluatePlan;
        succession first evaluatePlan then formGate;

        decide formGate;
        if form == "markdown" then writeMarkdown;
        else renderDiagrams;

        // A missing converter is reported, never a half-written document.
        succession first renderDiagrams then toolGate;

        decide toolGate;
        if toolsPresent then convertPdf;
        else reportMissingTool;
    }

    requirement def DocumentsCarryTheirProvenance {
        doc /* Every rendered node can be traced back to the element it came from,
             * and both backends read the same tree. */
        subject documents : DocumentPipeline;
        require constraint {
            documents.ir.provenancePerNode and documents.ir.backendAgnostic
                and documents.plans.immutablePlans
        }
    }

    // --- Conformance oracles ---------------------------------------------

    // Advisory harnesses that referee this implementation against the OMG
    // pilot; each holds a committed baseline that must move deliberately.
    abstract part def ConformanceOracle :> CodeUnit {
        attribute reference : String;
        attribute baseline : String;
    }

    #Oracle part def RejectionOracle :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-reject";
        attribute :>> reference = "pilot validator";
        attribute :>> baseline = "docs/project/pilot-rejection-baseline.json";
    }

    #Oracle part def DifferentialOracle :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-diff";
        attribute :>> reference = "pilot validator";
        attribute :>> baseline = "docs/project/pilot-differential-baseline.json";
    }

    #Oracle part def XpectOracle :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-xpect";
        attribute :>> reference = "pilot Xpect suites";
        attribute :>> baseline = "docs/project/pilot-xpect-baseline.json";
    }

    #Oracle part def ExecutionReferee :> ConformanceOracle {
        attribute :>> goPackage = "cmd/pilot-exec-diff";
        attribute :>> reference = "pilot expression evaluator";
        attribute :>> baseline = "docs/project/pilot-execution-referee.md";
    }

    #Oracle part def GrammarCoverage :> ConformanceOracle {
        attribute :>> goPackage = "cmd/grammar-coverage";
        attribute :>> reference = "OMG Xtext grammars";
        attribute :>> baseline = "docs/project/grammar-coverage-baseline.json";
    }

    #Oracle part def DocCounts :> ConformanceOracle {
        attribute :>> goPackage = "cmd/doc-counts";
        attribute :>> reference = "the measured repository";
        attribute :>> baseline = "docs/project/spec-compliance.md";
    }

    // --- The whole tool ---------------------------------------------------

    part def Toolchain {
        part pipeline : AnalysisPipeline;
        part documents : DocumentPipeline;

        part repl : Repl;
        part lsp : LanguageServer;
        part service : ModelService;
        part cli : CommandLine;

        part python : PythonClient;
        part node : NodeClient;
        part rust : RustClient;
        part java : JavaClient;
        part vscode : VsCodeExtension;

        part exporter : Exporter;
        part oslc : OslcQuery;

        part rejection : RejectionOracle;
        part differential : DifferentialOracle;
        part xpect : XpectOracle;
        part execution : ExecutionReferee;
        part grammar : GrammarCoverage;
        part counts : DocCounts;

        connection replDrives : ArtifactChannel connect repl.pipeline to pipeline.textIn;
        connection lspDrives : ArtifactChannel connect lsp.pipeline to pipeline.textIn;
        connection serviceDrives : ArtifactChannel connect service.pipeline to pipeline.textIn;
        connection cliDrives : ArtifactChannel connect cli.pipeline to pipeline.textIn;

        connection pythonCalls : ArtifactChannel connect python.service to service.pipeline;
        connection nodeCalls : ArtifactChannel connect node.service to service.pipeline;
        connection rustCalls : ArtifactChannel connect rust.service to service.pipeline;
        connection javaCalls : ArtifactChannel connect java.service to service.pipeline;
        connection editorSpeaksLsp : ArtifactChannel connect vscode.languageClient to lsp.pipeline;

        connection documentsRead : ArtifactChannel connect pipeline.textIn to documents.modelIn;
    }

    part opensysml : Toolchain;

    requirement documentsAreTraceable : DocumentsCarryTheirProvenance {
        subject documents = opensysml.documents;
    }
}
