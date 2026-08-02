/*
 * Parser Features Demo: Advanced Bodies
 * 
 * Demonstrates advanced body features:
 * - Return statements in predicate bodies (Task 83)
 * - Anonymous return parameters (Task 85)
 * - Body params with multiplicity after type (Task 75)
 * - Shorthand body param syntax (Task 87)
 * - Bool bodies with return statements (Task 80)
 * - General members in constraint bodies (Task 91)
 * 
 * Based on real stdlib patterns
 */

package AdvancedBodies {
    
    // ===== Predicate with Return Statements (Task 83) =====
    
    abstract predicate TransferCheck {
        in t1: Transfer [1];
        in t2: Transfer [1];
        
        return t1Before: Boolean;
        return t2Before: Boolean;
    }
    
    // ===== Anonymous Return Parameters (Task 85) =====
    
    abstract predicate BooleanEvaluation {
        doc
        /* Predicate that returns a boolean value */
        
        return : Boolean[1];
    }
    
    calc CalculateSum {
        in x: Integer;
        in y: Integer;
        
        return : Integer;
    }
    
    // ===== Body Param with Multiplicity After Type (Task 75) =====
    
    function processTransfers {
        in t1: Transfer [1];
        in t2: Transfer [2..*];
        
        return result: Boolean;
    }
    
    // ===== Shorthand Body Param Syntax (Task 87) =====
    
    feature Point { feature x: Real; feature y: Real; }
    
    feature Shape {
        feature vertices: Point[*];
        
        // Shorthand: {param : Type; expression} 
        feature hasPositive: Boolean = 
            vertices->exists{p : Point; p.x > 0};
        
        feature allPositive: Boolean = 
            vertices->forAll{v : Point; v.x >= 0};
    }
    
    // ===== Bool Bodies with Return (Task 80) =====
    
    bool earlierCheck : Boolean {
        in t1: Transfer [1];
        in t2: Transfer [1];
        
        return t1First = true;
    }
    
    // Bool body with single expression
    bool simpleCheck {
        in value: Integer;
        
        value > 0
    }
    
    // ===== General Members in Constraint Bodies (Task 91) =====
    
    constraint SystemRequirements {
        doc
        /* Constraint can contain definitions and usages */
        
        // Feature declarations inside constraint body
        ref ownedPerformances: Performance[*];
        ref subperformances: Performance[*];
        
        // Constraint expressions
        assert ownedPerformances->size() > 0;
    }
}
