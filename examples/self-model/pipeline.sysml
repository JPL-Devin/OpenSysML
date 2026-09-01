// The analysis pipeline of OpenSysML itself: what each stage consumes and
// produces, and the channels the artifacts travel over.
package OpenSysMLArtifacts {
    private import ScalarValues::*;

    // What moves between stages.
    item def SourceText;
    item def TokenStream;
    item def SyntaxTree;
    item def SymbolIndex;
    item def SideTable;
    item def ResolutionTable :> SideTable;
    item def TypeTable :> SideTable;
    item def Diagnostic;
    item def QuickFix;
    item def ActionGraph;
    item def StateGraph;
    item def ExecutionTrace;
    item def InstanceGraph;
    item def RdfGraph;
    item def QueryRows;
    item def DocumentPlan;
    item def DocumentTree;

    port def ArtifactPort;
    port def DiagnosticPort;
    port def RequestPort;

    connection def ArtifactChannel {
        end producer : ArtifactPort;
        end consumer : ArtifactPort;
    }

    connection def DiagnosticChannel {
        end reporter : DiagnosticPort;
        end collector : DiagnosticPort;
    }

    // Where a unit sits in the architecture, for filtered views.
    metadata def Frontend;
    metadata def SemanticEngine;
    metadata def Backend;
    metadata def Surface;
    metadata def Oracle;
    metadata def Invariant;

    // Every modelled unit names the Go package that implements it.
    abstract part def CodeUnit {
        attribute goPackage : String;
        attribute lazy : Boolean = false;
        attribute memoized : Boolean = false;
    }

    abstract part def Stage :> CodeUnit {
        port diagnostics : DiagnosticPort;
    }
}

package OpenSysMLPipeline {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;

    // --- Front end -------------------------------------------------------

    #Frontend part def SourceStore :> Stage {
        attribute :>> goPackage = "internal/core/source";
        attribute tracksLineIndex : Boolean = true;
        out item text : SourceText;
        port textOut : ArtifactPort;
    }

    #Frontend part def Lexer :> Stage {
        attribute :>> goPackage = "internal/core/lexer";
        attribute keywordCount : Integer = 172;
        attribute tracksTrivia : Boolean = true;
        in item text : SourceText;
        out item tokens : TokenStream;
        port textIn : ArtifactPort;
        port tokensOut : ArtifactPort;
    }

    // Recursive descent with error recovery: a tree always comes back.
    #Frontend part def Parser :> Stage {
        attribute :>> goPackage = "internal/core/parser";
        attribute neverPanics : Boolean = true;
        attribute emitsErrorNodes : Boolean = true;
        in item tokens : TokenStream;
        out item tree : SyntaxTree;
        out item fixes : QuickFix;
        port tokensIn : ArtifactPort;
        port treeOut : ArtifactPort;
    }

    // Syntax only, immutable after parse.
    #Frontend part def SyntaxTreeStore :> Stage {
        attribute :>> goPackage = "internal/core/ast";
        attribute immutableAfterParse : Boolean = true;
        attribute carriesSemantics : Boolean = false;
        in item tree : SyntaxTree;
        port treeIn : ArtifactPort;
        port treeOut : ArtifactPort;
    }

    #Frontend part def SymbolBuilder :> Stage {
        attribute :>> goPackage = "internal/core/symbols";
        attribute buildsScopeTree : Boolean = true;
        in item tree : SyntaxTree;
        out item index : SymbolIndex;
        port treeIn : ArtifactPort;
        port indexOut : ArtifactPort;
    }

    // --- Semantic engine -------------------------------------------------

    #SemanticEngine part def Resolver :> Stage {
        attribute :>> goPackage = "internal/core/resolve";
        attribute :>> lazy = true;
        attribute :>> memoized = true;
        attribute expandsWildcardImports : Boolean = true;
        in item index : SymbolIndex;
        out item bindings : ResolutionTable;
        port indexIn : ArtifactPort;
        port bindingsOut : ArtifactPort;
    }

    // Supertypes, conformance, members, multiplicity, constant folding.
    #SemanticEngine part def SemanticModel :> Stage {
        attribute :>> goPackage = "internal/core/semantics";
        attribute :>> memoized = true;
        in item bindings : ResolutionTable;
        out item types : TypeTable;
        port bindingsIn : ArtifactPort;
        port typesOut : ArtifactPort;
    }

    // A validation pass, one per tier level.
    abstract part def ValidationPass :> CodeUnit {
        attribute :>> goPackage = "internal/core/passes";
        attribute level : String;
        attribute elementScoped : Boolean = false;
        out item findings : Diagnostic;
    }

    part def SyntaxCheck :> ValidationPass {
        attribute :>> level = "syntax";
    }

    part def NameResolutionCheck :> ValidationPass {
        attribute :>> level = "nameResolution";
    }

    part def TypeCheck :> ValidationPass {
        attribute :>> level = "type";
    }

    part def ConstraintCheck :> ValidationPass {
        attribute :>> level = "constraint";
    }

    // Gates itself per subject through DownstreamOfFailure rather than being
    // skipped wholesale when a lower tier errors.
    part def NotationCheck :> ValidationPass {
        attribute :>> level = "constraint";
        attribute :>> elementScoped = true;
    }

    #SemanticEngine part def PassRegistry :> Stage {
        attribute :>> goPackage = "internal/core/passes";
        attribute tierCount : Integer = 4;

        part syntax : SyntaxCheck;
        part nameres : NameResolutionCheck;
        part types : TypeCheck;
        part constraints : ConstraintCheck;
        part notation : NotationCheck;

        in item typed : TypeTable;
        out item findings : Diagnostic;
        port typesIn : ArtifactPort;
        port findingsOut : ArtifactPort;
    }

    // Owns the document set, the global index and the diagnostic cache.
    #SemanticEngine part def Workspace :> Stage {
        attribute :>> goPackage = "internal/core/model";
        attribute oneSessionPerWorkspace : Boolean = true;
        port documents : ArtifactPort;
        port requests : RequestPort;
    }

    // Bundled standard library with an on-disk index cache.
    #SemanticEngine part def StandardLibrary :> Stage {
        attribute :>> goPackage = "internal/core/libs";
        attribute bundledFileCount : Integer = 97;
        attribute cached : Boolean = true;
        out item index : SymbolIndex;
        port indexOut : ArtifactPort;
    }

    // --- Back end --------------------------------------------------------

    // Lossless: guards, triggers, effects and pseudostate edges all survive.
    #Backend part def Lowering :> Stage {
        attribute :>> goPackage = "internal/core/lower";
        attribute lossless : Boolean = true;
        in item types : TypeTable;
        out item actions : ActionGraph;
        out item states : StateGraph;
        port typesIn : ArtifactPort;
        port irOut : ArtifactPort;
    }

    // Petri-net token flow over the lowered action graph.
    part def ActionExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute detectsDeadlock : Boolean = true;
        attribute recordsTrace : Boolean = true;
        in item graph : ActionGraph;
        out item trace : ExecutionTrace;
    }

    // Event-driven, hierarchical, with orthogonal regions.
    part def StateExecutor :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute runToCompletion : Boolean = true;
        attribute defersUnhandledEvents : Boolean = true;
        in item graph : StateGraph;
        out item trace : ExecutionTrace;
    }

    part def Evaluator :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute supportsCalcInvocation : Boolean = true;
    }

    part def InstanceModel :> CodeUnit {
        attribute :>> goPackage = "internal/core/runtime";
        attribute lazyFeatureValues : Boolean = true;
        out item instances : InstanceGraph;
    }

    #Backend part def Runtime :> Stage {
        attribute :>> goPackage = "internal/core/runtime";
        attribute stepBudgeted : Boolean = true;

        part instances : InstanceModel;
        part evaluator : Evaluator;
        part actions : ActionExecutor;
        part states : StateExecutor;

        in item actionGraph : ActionGraph;
        in item stateGraph : StateGraph;
        out item trace : ExecutionTrace;
        port irIn : ArtifactPort;
        port traceOut : ArtifactPort;
    }

    // Discharges constraint sets through an external SMT solver.
    #Backend part def Solver :> Stage {
        attribute :>> goPackage = "internal/core/solve";
        attribute engines : String = "z3, cvc5";
        port requests : RequestPort;
    }

    // --- The assembled pipeline ------------------------------------------

    part def AnalysisPipeline {
        part sources : SourceStore;
        part lexer : Lexer;
        part parser : Parser;
        part tree : SyntaxTreeStore;
        part symbols : SymbolBuilder;
        part resolver : Resolver;
        part semantics : SemanticModel;
        part passes : PassRegistry;
        part lowering : Lowering;
        part runtime : Runtime;
        part workspace : Workspace;
        part stdlib : StandardLibrary;
        part solver : Solver;

        port textIn : ArtifactPort;
        port diagnosticsOut : DiagnosticPort;

        connection textChannel : ArtifactChannel connect sources.textOut to lexer.textIn;
        connection tokenChannel : ArtifactChannel connect lexer.tokensOut to parser.tokensIn;
        connection treeChannel : ArtifactChannel connect parser.treeOut to tree.treeIn;
        connection symbolChannel : ArtifactChannel connect tree.treeOut to symbols.treeIn;
        connection indexChannel : ArtifactChannel connect symbols.indexOut to resolver.indexIn;
        connection stdlibChannel : ArtifactChannel connect stdlib.indexOut to resolver.indexIn;
        connection bindingChannel : ArtifactChannel connect resolver.bindingsOut to semantics.bindingsIn;
        connection typeChannel : ArtifactChannel connect semantics.typesOut to passes.typesIn;
        connection loweringChannel : ArtifactChannel connect semantics.typesOut to lowering.typesIn;
        connection irChannel : ArtifactChannel connect lowering.irOut to runtime.irIn;
        connection diagnosticChannel : DiagnosticChannel connect passes.diagnostics to workspace.diagnostics;

        flow of SourceText from sources.text to lexer.text;
        flow of TokenStream from lexer.tokens to parser.tokens;
        flow of SyntaxTree from parser.tree to tree.tree;
        flow of SyntaxTree from tree.tree to symbols.tree;
        flow of SymbolIndex from symbols.index to resolver.index;
        flow of SymbolIndex from stdlib.index to resolver.index;
        flow of ResolutionTable from resolver.bindings to semantics.bindings;
        flow of TypeTable from semantics.types to passes.typed;
        flow of TypeTable from semantics.types to lowering.types;
        flow of ActionGraph from lowering.actions to runtime.actionGraph;
        flow of StateGraph from lowering.states to runtime.stateGraph;
    }
}
