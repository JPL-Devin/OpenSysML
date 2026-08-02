/*
 * Parser Features Demo: Edge Cases & Advanced Patterns
 * 
 * Demonstrates edge case handling (Tasks 62, 67, 70, 90).
 * All patterns extracted from actual stdlib files.
 */

package EdgeCasesAndAdvanced {
    
    // ===== Keywords as Feature Names in Expressions (Task 70) =====
    // From StatePerformances.kerml
    
    feature StateWithKeywordNames {
        feature entry: Action;
        feature exit: Action;
        feature do: Action;
        
        feature entryHandler = state.entry;
        feature exitHandler = state.exit;
    }
    
    // ===== Identifier-Based Multiplicities (Task 70) =====
    // From TransitionPerformances.kerml
    
    feature ProcessFlow {
        feature itemCount: Integer;
        
        succession [itemCount] items 
            first [1] initialize
            then [1] process;
    }
    
    // ===== Bind Keyword Shorthand (Task 62) =====
    // From Actions.sysml and States.sysml
    
    state AcceptingState {
        bind payload = accepter.payload;
        bind receiver = accepter.receiver;
    }
    
    // ===== Step Usage with 'do' as Name (Task 67) =====
    // From StatePerformances.kerml
    
    feature StatePerformance {
        step entry[1];
        step do[1] subsets middle;
        step exit[1];
    }
    
    // ===== Ref with Body (Task 90) =====
    // From Flows.sysml
    
    ref payload [0..*] {
        doc
        /* Reference with body */
        
        feature dataType: String;
        feature size: Integer;
    }
}
