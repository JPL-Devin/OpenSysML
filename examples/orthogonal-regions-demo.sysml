package OrthogonalRegionsDemo {
    private import ScalarValues::*;
    
    // Demonstrates concurrent orthogonal regions in state machines
    // Each region executes independently and processes events in parallel
    
    // Example: Traffic Light with Pedestrian Crossing
    state def TrafficLight parallel {
        // Vehicle signal region
        state vehicleSignal {
            entry; then start;
            state start;
            state red;
            state yellow;
            state green;
            
            succession first start then red;
            transition first red then yellow;
            transition first yellow then green;
            transition first green then red;
        }
        
        // Pedestrian signal region (runs concurrently)
        state pedestrianSignal {
            entry; then start;
            state start;
            state dontWalk;
            state walk;
            
            succession first start then dontWalk;
            transition first dontWalk then walk;
            transition first walk then dontWalk;
        }
        
        // Key concepts:
        // - Both regions start simultaneously (v_start→red, p_start→dontWalk)
        // - Active configuration: (red, dontWalk) or (yellow, walk) etc
        // - Events broadcast to both regions independently
        // - Run-to-completion: one transition per region per event
    }
}
