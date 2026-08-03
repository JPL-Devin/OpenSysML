package ActionExecutorDemo {
    import ScalarValues::*;
    
    // Simplified action demo - basic sequence
    action def SimpleAction {
        in x : Real;
        out result : Real;
        
        // Simple action sequence
        bind result = x * 2.0;
    }
    
    // Demo removed - control flow nodes (fork/join/merge/decision) 
    // require more complex syntax not yet fully supported
}
