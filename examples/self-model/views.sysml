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
