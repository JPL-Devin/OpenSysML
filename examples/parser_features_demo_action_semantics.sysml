/*
 * Parser Features Demo: Action Semantics
 * 
 * Demonstrates action body statements (Tasks 72, 93).
 * All patterns extracted from Actions.sysml stdlib file.
 */

package ActionSemantics {
    
    // The library is not implicitly visible: a bare name needs an import.
    private import ScalarValues::*;
    
    // The actions the performs below refer to.
    action ValidateInput;
    action Transform;
    action SaveResults;
    
    // ===== Basic Action with Assign (Task 72) =====
    
    action InitializeCounter {
        attribute counter : Integer;
        
        assign counter := 0;
    }
    
    // ===== Action with Perform and Then =====
    
    action ProcessWorkflow {
        perform ValidateInput;
        then perform Transform;
        then perform SaveResults;
    }
    
    // ===== While Loop (Task 72) =====
    
    action IterateSequence {
        attribute index : Integer;
        attribute var : Integer;
        
        assign index := 1;
        then while index <= 10 {
            assign var := index;
            then assign index := index + 1;
        }
    }
    
    // ===== If/Else Statement (Task 72) =====
    
    action ConditionalProcess {
        attribute value : Integer;
        attribute result : Integer;
        
        if value > 0 {
            assign result := 1;
        } else {
            assign result := 0;
        }
    }
    
    // ===== Namespace-Level Succession within Action (Task 93) =====
    // Exact pattern from Actions.sysml forEachLoop
    
    action forEachLoop {
        attribute index : Integer;
        attribute var : Integer;
        
        private action initialization
            assign index := 1;
        then private action whileLoop
            while index <= 10 {
                assign var := index;
                then assign index := index + 1;
            }
    }
}
