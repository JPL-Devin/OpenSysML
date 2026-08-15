/*
 * Parser Features Demo: Advanced Connectors
 * 
 * Demonstrates advanced connector features (Tasks 58-86).
 * All patterns extracted from actual stdlib files.
 */

package AdvancedConnectors {
    
    // The library is not implicitly visible: a bare name needs an import.
    private import ScalarValues::*;
    
    // The features the connectors below relate.
    feature transitionLink[0..1];
    feature transitionLinkSource[0..1];
    feature trigger[1..*];
    
    // ===== Single-End Connector Typing with 'to' (Task 86) =====
    // From TransitionPerformances.kerml line 46
    
    private connector [0..1] transitionLink to [1..*] trigger;
    
    // ===== Connector with 'references' Keyword (Task 59) =====
    // From Transfers.kerml lines 16-17
    
    feature TransferPattern {
        feature source[1];
        feature target[1];
        
        end [1] feature transferSource references source;
        end [1] feature transferTarget references target;
    }
    
    // ===== Named Succession with Identifier Multiplicity (Task 81) =====
    // From TransitionPerformances.kerml line 35
    
    feature triggerNum: Integer;
    succession triggerAfter [triggerNum] 
        first [0..1] transitionLinkSource 
        then [*] trigger;
}
