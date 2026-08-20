/*
 * Parser Features Demo: Edge Cases & Advanced Patterns
 * 
 * Demonstrates edge case handling (Tasks 62, 67, 70, 90).
 * All patterns extracted from actual stdlib files.
 */

package EdgeCasesAndAdvanced {
    
    // The library is not implicitly visible: a bare name needs an import.
    private import ScalarValues::*;
    private import StatePerformances::StatePerformance;
    
    // ===== Keywords as Feature Names in Expressions (Task 70) =====
    // From StatePerformances.kerml line 77-90
    // Keywords like 'exit', 'entry' can appear in feature chain expressions
    
    function testKeywordAccess {
        in oSP: StatePerformance;
        in thatSP: StatePerformance;
        return result: Boolean = 
            oSP.exit == thatSP.exit;
    }
    
    // ===== Identifier-Based Multiplicities (Task 70) =====
    // From TransitionPerformances.kerml
    
    feature ProcessFlow {
        feature itemCount: Natural;
        
        step initialize[1];
        step process[1];
        
        succession items [itemCount] 
            first [1] initialize
            then [1] process;
    }
    
    // ===== Binding Connectors in a Behavior Body (Task 62) =====
    // From TransitionPerformances.kerml
    
    // 'binding' binds a feature to a feature of a nested step
    behavior AcceptingTransition {
        inout payload[0..*];
        feature receiver[0..1];
        
        step accepter {
            inout payload[0..*];
            feature receiver[0..1];
        }
        
        binding payload = accepter.payload;
        binding receiver = accepter.receiver;
    }
    
    // ===== Step Usage with 'do' as Name (Task 67) =====
    // From StatePerformances.kerml
    
    feature StatePerformanceSteps {
        abstract step middle[1..*];
        
        step entry[1];
        step do[1] subsets middle;
        step exit[1];
    }
    
    // ===== Feature with Body (Task 90) =====
    // From Transfers.kerml
    
    feature payload [0..*] {
        doc
        /* Reference with body */
        
        feature dataType: String;
        feature size: Integer;
    }
}
