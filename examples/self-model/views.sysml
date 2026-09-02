// The views that present the architecture, one per rendering kind, plus the
// filtered layer views.
package OpenSysMLViews {
    private import Views::*;
    private import StandardViewDefinitions::*;
    private import OpenSysMLArtifacts::*;

    viewpoint def MaintainerPerspective {
        frame concern latency : OpenSysMLInvariants::KeystrokeLatency;
    }

    viewpoint maintainerPerspective : MaintainerPerspective;

    view pipelineStructure {
        doc /* The analysis pipeline: stages, channels and the artifacts flowing. */
        expose OpenSysMLPipeline::AnalysisPipeline;
        render asInterconnectionDiagram;
    }

    view toolchainStructure {
        doc /* Surfaces, clients and oracles around the pipeline. */
        expose OpenSysMLSurfaces::Toolchain;
        render asInterconnectionDiagram;
    }

    view documentPipeline {
        expose OpenSysMLSurfaces::DocumentPipeline;
        render asInterconnectionDiagram;
    }

    view documentFlow : ActionFlowView {
        doc /* One document rendered, and the two branches in it. */
        expose OpenSysMLSurfaces::RenderDocument;
    }

    view stageTable {
        doc /* Every stage with the Go package that implements it. */
        expose OpenSysMLPipeline::AnalysisPipeline;
        render asElementTable;
    }

    view invariantTable {
        expose OpenSysMLInvariants::**[@OpenSysMLArtifacts::Invariant];
        render asElementTable;
    }

    view analysisFlow : ActionFlowView {
        expose OpenSysMLBehavior::AnalyzeDocument;
    }

    view editFlow : ActionFlowView {
        expose OpenSysMLBehavior::ServeEdit;
    }

    view libraryLoadFlow : ActionFlowView {
        doc /* The bundled library loaded: from its snapshot, or from its files. */
        expose OpenSysMLBehavior::LoadLibrary;
    }

    view tierStates : StateTransitionView {
        expose OpenSysMLBehavior::TierProgression;
    }

    view runtimeTiers : StateTransitionView {
        expose OpenSysMLBehavior::RuntimeTiers;
    }

    view tokenFlow : StateTransitionView {
        expose OpenSysMLBehavior::TokenFlow;
    }

    view eventDispatch : StateTransitionView {
        expose OpenSysMLBehavior::EventDispatch;
    }

    view budgetExhaustion : StateTransitionView {
        doc /* Six budgets, six ways a run can end early, each naming itself. */
        expose OpenSysMLBehavior::BudgetExhaustion;
    }

    view editorPipeline {
        doc /* What an editor reaches: highlighting, fixes, suggestions, edits. */
        expose OpenSysMLSurfaces::EditorPipeline;
        render asInterconnectionDiagram;
    }

    view renderingKinds {
        doc /* Every rendering kind the view engine recognizes, and which it produces. */
        expose OpenSysMLSurfaces::ViewEngine;
        render asElementTable;
    }

    view identityPath {
        doc /* How an element's repository id reaches RDF and comes back. */
        expose OpenSysMLIdentity::IdentityPath;
        render asInterconnectionDiagram;
    }

    view identityRoundTrip : ActionFlowView {
        expose OpenSysMLIdentity::CarryIdentity;
    }

    view syncFlow : ActionFlowView {
        doc /* A local model diffed against its repository, keyed by effective id. */
        expose OpenSysMLIdentity::SyncModel;
    }

    view identityLayer {
        expose OpenSysMLIdentity::**[@OpenSysMLIdentity::Identity];
    }

    // The layer views, each reached by a filtered recursive expose.
    view frontendLayer {
        expose OpenSysMLPipeline::**[@OpenSysMLArtifacts::Frontend];
    }

    view semanticLayer {
        expose OpenSysMLPipeline::**[@OpenSysMLArtifacts::SemanticEngine];
    }

    view backendLayer {
        expose OpenSysMLPipeline::**[@OpenSysMLArtifacts::Backend];
    }

    view oracleLayer {
        expose OpenSysMLSurfaces::**[@OpenSysMLArtifacts::Oracle];
    }

    view overview {
        expose OpenSysMLSurfaces::opensysml;
        expose OpenSysMLSurfaces::Toolchain::lsp;
        view pipelineSubview :> pipelineStructure;

        satisfy maintainerPerspective;
        frame concern latency : OpenSysMLInvariants::KeystrokeLatency;
    }
}
