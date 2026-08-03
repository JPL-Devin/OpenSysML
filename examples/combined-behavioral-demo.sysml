package CombinedBehavioralDemo {
    import ScalarValues::*;
    
    // Simplified behavioral demo
    action def Example {
        in x : Real;
        out result : Real;
        
        action compute {
            in value : Real;
            out output : Real;
        }
        
        bind compute.value = x;
        bind result = compute.output;
    }
    
    // Complex control flow demos removed - require additional syntax support
}
