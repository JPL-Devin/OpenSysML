// How the pipeline behaves: one analysis run, the tier gating that decides how
// far it gets, and the two execution engines the runtime drives.
package OpenSysMLBehavior {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;

    attribute def EditEvent;
    attribute def TokenArrival;
    attribute def SignalEvent;

    // One document analysed end to end. The tiers decide whether lowering and
    // execution are reached at all.
    action def AnalyzeDocument {
        attribute syntaxErrors : Integer = 0;
        attribute semanticErrors : Integer = 0;
        attribute executable : Boolean = true;

        in item text : SourceText;
        out item findings : Diagnostic;
        out item trace : ExecutionTrace;

        first start;

        action scan;
        action parse;
        action buildScopes;
        action resolveNames;
        action buildSemanticModel;

        action runSyntaxTier;
        action runNameTier;
        action runTypeTier;
        action runConstraintTier;

        action lower;
        action execute;
        action publish;

        succession first start then scan;
        succession first scan then parse;
        succession first parse then buildScopes;
        succession first buildScopes then resolveNames;
        succession first resolveNames then buildSemanticModel;
        succession first buildSemanticModel then runSyntaxTier;
        succession first runSyntaxTier then syntaxGate;

        // A failed tier stops the tiers above it; diagnostics are still published.
        decide syntaxGate;
        if syntaxErrors == 0 then runNameTier;
        else publish;

        succession first runNameTier then nameGate;

        decide nameGate;
        if semanticErrors == 0 then runTypeTier;
        else publish;

        succession first runTypeTier then typeGate;

        decide typeGate;
        if semanticErrors == 0 then runConstraintTier;
        else publish;

        succession first runConstraintTier then executionGate;

        decide executionGate;
        if executable then lower;
        else publish;

        succession first lower then execute;
        succession first execute then publish;
        succession first publish then done;
    }

    // The editor path: an edit reanalyses its own document immediately and
    // sweeps the other open ones once the burst settles.
    action def ServeEdit {
        in item edit : SourceText;
        out item findings : Diagnostic;

        first start;

        action applyEdit;
        action reanalyzeEdited;
        action publishEdited;
        action debounce;
        action sweepOpenDocuments;

        succession first start then applyEdit;
        succession first applyEdit then reanalyzeEdited;
        succession first reanalyzeEdited then split;

        fork split;
        succession first split then publishEdited;
        succession first split then debounce;
        succession first debounce then sweepOpenDocuments;
        succession first publishEdited then sync;
        succession first sweepOpenDocuments then sync;

        join sync;
        succession first sync then done;
    }

    // The library loaded once per process: the snapshot when it was written from
    // these files and decodes intact, the files otherwise.
    action def LoadLibrary {
        attribute snapshotCurrent : Boolean = true;
        attribute snapshotIntact : Boolean = true;

        in item snapshotBlob : LibrarySnapshot;
        out item index : SymbolIndex;

        first start;

        action digestFiles;
        action readHeader;
        action verifyChecksum;
        action decodeSnapshot;
        action parseFiles;
        action indexFiles;
        action expandImports;
        action freeze;
        action share;

        succession first start then digestFiles;
        succession first digestFiles then readHeader;
        succession first readHeader then digestGate;

        // A snapshot of other files, another format or an override parses.
        decide digestGate;
        if snapshotCurrent then verifyChecksum;
        else parseFiles;

        succession first verifyChecksum then checksumGate;

        decide checksumGate;
        if snapshotIntact then decodeSnapshot;
        else parseFiles;

        succession first parseFiles then indexFiles;
        succession first indexFiles then expandImports;
        succession first expandImports then freeze;
        succession first decodeSnapshot then share;
        succession first freeze then share;
        succession first share then done;
    }

    // The tier ladder as states: each tier is entered only from a clean tier
    // below it, and any failure halts short of execution.
    state def TierProgression {
        attribute failures : Integer = 0;
        attribute elementScopedPending : Boolean = false;

        entry; then syntaxTier;

        state syntaxTier;
        state nameTier;
        state typeTier;
        state constraintTier {
            entry; then documentScoped;

            state documentScoped;
            state elementScoped;

            // An element-scoped pass still runs for subjects that are not
            // downstream of the failure.
            transition documentScoped_to_elementScoped
                first documentScoped
                if elementScopedPending
                then elementScoped;
        }
        state executing;
        state halted;

        transition first syntaxTier if failures == 0 then nameTier;
        transition first syntaxTier if failures > 0 then halted;
        transition first nameTier if failures == 0 then typeTier;
        transition first nameTier if failures > 0 then halted;
        transition first typeTier if failures == 0 then constraintTier;
        transition first typeTier if failures > 0 then halted;
        transition first constraintTier if failures == 0 then executing;
        transition first constraintTier if failures > 0 then halted;
    }

    // One calc invocation: a pure body runs compiled unless the run is traced or
    // an argument is bound by name or is not a scalar; the answer is the same.
    action def InvokeCalc {
        attribute tierEnabled : Boolean = true;
        attribute untraced : Boolean = true;
        attribute bodyIsPure : Boolean = true;
        attribute positionalScalars : Boolean = true;

        first start;

        action bindArguments;
        action compileBody;
        action runCompiled;
        action boxResult;
        action evaluateBody;

        succession first start then bindArguments;
        succession first bindArguments then traceGate;

        // A traced run records every sub-expression, which only the evaluator does.
        decide traceGate;
        if tierEnabled and untraced then bodyGate;
        else evaluateBody;

        decide bodyGate;
        if bodyIsPure then compileBody;
        else evaluateBody;

        succession first compileBody then argumentGate;

        decide argumentGate;
        if positionalScalars then runCompiled;
        else evaluateBody;

        succession first runCompiled then boxResult;
        succession first boxResult then done;
        succession first evaluateBody then done;
    }

    // The runtime tiers, from an effective feature list to a simulated model.
    state def RuntimeTiers {
        entry; then featureFlattening;

        state featureFlattening;
        state instanceModel;
        state expressionEvaluation;
        state behavioralModel;
        state behavioralExecution;

        transition first featureFlattening then instanceModel;
        transition first instanceModel then expressionEvaluation;
        transition first expressionEvaluation then behavioralModel;
        transition first behavioralModel then behavioralExecution;
    }

    // Token flow: a step moves every enabled token, and a round with no
    // movement is a deadlock rather than a hang. The budget is the action-step one.
    state def TokenFlow {
        attribute progressed : Boolean = true;
        attribute budgetLeft : Integer = 1000000;

        entry; then idle;

        state idle;
        state stepping;
        state blocked;
        state completed;

        transition first idle accept TokenArrival then stepping;
        transition first stepping if progressed and budgetLeft > 0 then idle;
        transition first stepping if not progressed then blocked;
        transition first stepping if budgetLeft <= 0 then blocked;
        transition first idle if not progressed then completed;
    }

    // Event dispatch: run-to-completion, with unhandled events deferred while a
    // deferring state is active.
    state def EventDispatch {
        attribute handled : Boolean = true;
        attribute deferring : Boolean = false;

        entry; then waiting;

        state waiting;
        state dispatching;
        state deferred;
        state settled;

        transition first waiting accept SignalEvent then dispatching;
        transition first dispatching if handled then settled;
        transition first dispatching if not handled and deferring then deferred;
        transition first deferred then waiting;
        transition first settled then waiting;
    }

    // Six budgets, each exhausted on its own: the first to run out ends the run
    // with the error that names it, and the others are not consulted.
    state def BudgetExhaustion {
        attribute stepsLeft : Integer = 10000000;
        attribute actionStepsLeft : Integer = 1000000;
        attribute stateEventsLeft : Integer = 1000000;
        attribute doStepsLeft : Integer = 5000000;
        attribute elementsLeft : Integer = 1000000;
        attribute calcDepthLeft : Integer = 10000;

        entry; then running;

        state running;
        state stepLimitExceeded;
        state actionStepLimitExceeded;
        state stateEventLimitExceeded;
        state doStepLimitExceeded;
        state elementLimitExceeded;
        state calcRecursionLimitExceeded;

        transition first running if stepsLeft <= 0 then stepLimitExceeded;
        transition first running if actionStepsLeft <= 0 then actionStepLimitExceeded;
        transition first running if stateEventsLeft <= 0 then stateEventLimitExceeded;
        transition first running if doStepsLeft <= 0 then doStepLimitExceeded;
        transition first running if elementsLeft <= 0 then elementLimitExceeded;
        transition first running if calcDepthLeft <= 0 then calcRecursionLimitExceeded;
    }

    // What a step costs, for the action-step budget the runtime enforces.
    calc def StepBudget {
        in attribute rounds : Integer;
        in attribute tokensPerRound : Integer;
        return attribute steps : Integer = rounds * tokensPerRound;
    }
}
