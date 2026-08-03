// Action Executor Demo - Showcasing Token Flow Execution
// This file demonstrates the action executor capabilities implemented in Phase 2

package ActionExecutorDemo {
    
    // Demo 1: Sequential execution (Initial → Action → Final)
    action def ComputeAction { in x : Integer; out result : Integer; }
    
    action def SequentialAction {
        first start;
        
        action compute : ComputeAction {
            in item x;
            out item result;
        }
        
        done;
        
        succession start then compute;
        succession compute then end;
    }
    
    // Demo 2: Fork/Join (Parallel execution)
    action def Task { out result : Integer; }
    
    action def ParallelAction {
        first start;
        fork fork;
        
        action task1 : Task {
            out item result;
        }
        action task2 : Task {
            out item result;
        }
        action task3 : Task {
            out item result;
        }
        
        join join;
        done;
        
        succession start then fork;
        succession fork then task1;
        succession fork then task2;
        succession fork then task3;
        succession task1 then join;
        succession task2 then join;
        succession task3 then join;
        succession join then end;
    }
    
    // Demo 3: Decision/Merge (Conditional branching)
    action def ConditionalAction {
        first start;
        decision decision;
        
        action pathA {
            // Path A logic
        }
        action pathB {
            // Path B logic
        }
        
        merge merge;
        done;
        
        succession start then decision;
        succession decision then pathA if (x > 10);
        succession decision then pathB if (x <= 10);
        succession pathA then merge;
        succession pathB then merge;
        succession merge then end;
    }
    
    // Demo 4: Complex workflow (Fork → Decision → Merge → Join)
    action def ComplexWorkflow {
        first start;
        fork fork;
        
        // Branch 1: Decision
        decision decision1;
        action branch1A {
            // Branch 1A logic
        }
        action branch1B {
            // Branch 1B logic
        }
        merge merge1;
        
        // Branch 2: Decision
        decision decision2;
        action branch2A {
            // Branch 2A logic
        }
        action branch2B {
            // Branch 2B logic
        }
        merge merge2;
        
        join join;
        action final {
            // Final action
        }
        done;
        
        succession start then fork;
        
        // Branch 1
        succession fork then decision1;
        succession decision1 then branch1A if (x > 50);
        succession decision1 then branch1B; // else
        succession branch1A then merge1;
        succession branch1B then merge1;
        succession merge1 then join;
        
        // Branch 2
        succession fork then decision2;
        succession decision2 then branch2A if (x > 25);
        succession decision2 then branch2B; // else
        succession branch2A then merge2;
        succession branch2B then merge2;
        succession merge2 then join;
        
        succession join then final;
        succession final then end;
    }
}
